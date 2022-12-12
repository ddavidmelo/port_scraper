package scan

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/csv"
	"io"
	"net"
	"net/http"
	"os"
	"port_scraper/internal/config"
	"port_scraper/internal/sqldb/storage"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/datadog/mmh3"
	"github.com/remeh/sizedwaitgroup"
	log "github.com/sirupsen/logrus"
)

var (
	nRoutines = config.GetGeneralConfig().N_routines
	userAgent = config.GetUserAgent()
)

const (
	tcpTimeout  = 500 * time.Millisecond
	httpTimeout = 5 * time.Second
)

type Scan storage.Scan

type GeoIP struct {
	CountryCode string
	District    string
	City        string
	Latitude    float32
	Longitude   float32
}

func Start() {
	portRange := config.GetScraperConfig().PortRange
	filePath := config.GetScraperConfig().FilePath

	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err)
	}

	row := 0
	parser := csv.NewReader(file)

	for {
		record, err := parser.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Error(err)
		}

		var geoIP GeoIP

		geoIP.CountryCode = record[2]
		geoIP.District = record[3]
		geoIP.City = record[5]
		latitude, err := strconv.ParseFloat(record[7], 64)
		if err != nil {
			log.Error("CSV Invalid latitude")
		}
		geoIP.Latitude = float32(latitude)
		longitude, err := strconv.ParseFloat(record[8], 64)
		if err != nil {
			log.Error("CSV Invalid longitude")
		}
		geoIP.Longitude = float32(longitude)

		log.Infof("[ROW: %d]--- Starting --- %s to %s", row, net.ParseIP(record[0]).To4().String(), net.ParseIP(record[1]).To4().String())
		scanLoop(&geoIP, net.ParseIP(record[0]).To4(), net.ParseIP(record[1]).To4(), portRange)
		row++
	}
}

func scanLoop(geoIP *GeoIP, start_ip net.IP, end_ip net.IP, ports []string) {
	swg := sizedwaitgroup.New(nRoutines)
	for sub_a := start_ip[0]; sub_a < 255; sub_a++ {
		if sub_a > end_ip[0] {
			goto stop
		}
		for sub_b := start_ip[1]; sub_b < 255; sub_b++ {
			if sub_a == end_ip[0] && sub_b > end_ip[1] {
				goto stop
			}
			for sub_c := start_ip[2]; sub_c < 255; sub_c++ {
				if sub_a == end_ip[0] && sub_b == end_ip[1] && sub_c > end_ip[2] {
					goto stop
				}
				for sub_d := start_ip[3]; sub_d < 255; sub_d++ {
					if sub_a == end_ip[0] && sub_b == end_ip[1] && sub_c == end_ip[2] && sub_d > end_ip[3] {
						goto stop
					}
					swg.Add()
					go func(rsub_a byte, rsub_b byte, rsub_c byte, rsub_d byte) {
						defer swg.Done()
						for _, port := range ports {
							var scan Scan
							scan.IP = net.IPv4(rsub_a, rsub_b, rsub_c, rsub_d)
							scan.CountryCode = geoIP.CountryCode
							scan.District = geoIP.District
							scan.City = geoIP.City
							scan.Latitude = geoIP.Latitude
							scan.Longitude = geoIP.Longitude
							RawRequest(&scan, scan.IP.String(), port)
							if scan.Open {
								log.Info(scan, "--", scan.IP.To16())
								row := storage.Scan(scan)
								err := storage.InsertScanResult(row)
								if err != nil {
									log.Error("DB insert: ", err)
								}
							}
						}
					}(sub_a, sub_b, sub_c, sub_d)

				}
			}
		}
	}
stop:

	swg.Wait()
}

func RawRequest(scan *Scan, host string, port string) {
	portI, err := strconv.Atoi(port)
	if err != nil {
		log.Error("Wrong port format:", err)
	}

	scan.Port = uint16(portI)
	hostPort := net.JoinHostPort(host, port)

	// TCP non encrypted
	start := time.Now()
	conn, err := net.DialTimeout("tcp", hostPort, tcpTimeout)
	if err != nil {
		log.Debug("TCP Connecting error:", err)
		return
	}

	if conn != nil {
		defer conn.Close()
		log.Debug("OPENED:", hostPort)
		scan.Open = true
		scan.Latency = time.Since(start)
		conn.SetReadDeadline(time.Now().Add(tcpTimeout))

		// Select the serivce based on the Port number
		scan.ServiceInfo, err = ServiceSelector(conn, port)
		if err == nil {
			return
		}

		err = httpRequest(scan, hostPort, false)
		if err != nil {
			log.Debug("TCP HTTP error:", err)
		} else {
			if scan.HTTPstatus < 400 && scan.HTTPstatus != 0 {
				return
			}
		}
	}
	scan.TLS = false

	// TCP + TLS encrypted
	conf := &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS10,
	}
	ctx, cancel := context.WithTimeout(context.Background(), tcpTimeout)
	d := tls.Dialer{
		Config: conf,
	}
	conn, err = d.DialContext(ctx, "tcp", hostPort)
	cancel() // Ensure cancel is always called
	if err != nil {
		log.Debug("TCP+TLS Connecting error:", err)
	}

	if conn != nil {
		defer conn.Close()

		tlsConn := conn.(*tls.Conn)
		state := tlsConn.ConnectionState()
		for _, v := range state.PeerCertificates {
			if v.DNSNames != nil || v.Subject.Organization != nil {
				log.Debug("----- TCP+TLS DNSNames=", v.DNSNames, "  TCP+TLS Organization=", v.Subject.Organization)
				scan.TLScertificate.DNSNames = v.DNSNames
				scan.TLScertificate.Organizations = v.Subject.Organization

			}

		}
		log.Debug("TCP+TLS ServerName:", state.ServerName)

		scan.TLS = true
		err = httpRequest(scan, hostPort, true)
		if err != nil {
			log.Debug("TCP+TLS HTTP error:", err)
		}
	}

}

func httpRequest(scan *Scan, hostport string, secure bool) error {
	var httpProtocol string
	if secure {
		httpProtocol = "https"
	} else {
		httpProtocol = "http"
	}

	tr := &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		TLSHandshakeTimeout:   httpTimeout,
		IdleConnTimeout:       httpTimeout,
		ResponseHeaderTimeout: httpTimeout,
		ExpectContinueTimeout: httpTimeout,
	}
	client := &http.Client{Timeout: httpTimeout, Transport: tr}

	req, err := http.NewRequest("GET", httpProtocol+"://"+hostport, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	log.Debug("ENDPOINT: ", hostport, "---", resp.Header.Get("Server"), "---", resp.StatusCode)
	scan.HTTPstatus = uint16(resp.StatusCode)
	scan.HTTPserver = resp.Header.Get("Server")

	// Limit response body size to 10Mbytes
	limitedReader := &io.LimitedReader{R: resp.Body, N: 10000000}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(limitedReader)
	if err != nil {
		return err
	}

	// Find HTML page title
	title := doc.Find("title").Text()
	if len(title) >= 30 {
		title = title[:30]
	}
	log.Debug("TITLE: ", title)
	scan.HTTPtitle = title
	client.CloseIdleConnections()

	// Find FavICON
	b, err := getFavIcon(client, httpProtocol+"://"+hostport+"/favicon.ico")
	if b != nil && err == nil {
		scan.HTTPfavicon = int32(mmh3.Hash32(standBase64(b)))
	} else {
		var faviconURLPath string
		doc.Find("link").EachWithBreak(func(i int, s *goquery.Selection) bool {
			href, ok_href := s.Attr("href")
			rel, ok_rel := s.Attr("rel")
			if ok_href && ok_rel && rel == "icon" {
				faviconURLPath = strings.TrimLeft(href, "/")
				return false
			}
			return true
		})
		if faviconURLPath != "" {
			b, err := getFavIcon(client, httpProtocol+"://"+hostport+"/"+faviconURLPath)
			if b != nil && err == nil {
				scan.HTTPfavicon = int32(mmh3.Hash32(standBase64(b)))
			}
		}

	}

	return nil
}

func getFavIcon(client *http.Client, endpoint string) ([]byte, error) {
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Limit response body size to 10Mbytes
	limitedReader := &io.LimitedReader{R: resp.Body, N: 10000000}

	if resp.StatusCode < 400 {
		b, err := io.ReadAll(limitedReader)
		if err != nil {
			log.Debug(err)
		} else {
			return b, nil
		}
	}
	return nil, nil
}

func standBase64(braw []byte) []byte {
	bckd := base64.StdEncoding.EncodeToString(braw)
	var buffer bytes.Buffer
	for i := 0; i < len(bckd); i++ {
		ch := bckd[i]
		buffer.WriteByte(ch)
		if (i+1)%76 == 0 {
			buffer.WriteByte('\n')
		}
	}
	buffer.WriteByte('\n')

	return buffer.Bytes()
}

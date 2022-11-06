package scraper

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"port_scraper/internal/config"
	"port_scraper/internal/sqldb"
	"strconv"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/datadog/mmh3"
	"github.com/remeh/sizedwaitgroup"
	log "github.com/sirupsen/logrus"
)

var n_routines = config.GetGeneralConfig().N_routines

const (
	tcpTimeout  = 500 * time.Millisecond
	httpTimeout = 5 * time.Second
)

type GeoIP struct {
	CountryCode string
	District    string
	City        string
	Latitude    float32
	Longitude   float32
}

type TLScertificate struct {
	DNSNames      []string `json:"dnsNames"`
	Organizations []string `json:"organizations"`
}

type PortScraper struct {
	IP             net.IP
	Port           uint16
	Open           bool
	Latency        time.Duration
	HTTPstatus     uint16
	HTTPtitle      string
	HTTPserver     string
	HTTPfavicon    int32
	ServiceInfo    ServiceInfo
	TLS            bool
	TLScertificate TLScertificate
}

func Start() {
	scraper_config := config.GetScraperConfig()

	file, err := os.Open(scraper_config.FilePath)
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
		ipLoop(net.ParseIP(record[0]).To4(), net.ParseIP(record[1]).To4(), scraper_config.PortRange, geoIP)
		row++
	}
}

func ipLoop(start_ip net.IP, end_ip net.IP, ports []string, geoIP GeoIP) {
	swg := sizedwaitgroup.New(n_routines)
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
							var portScraper PortScraper
							portScraper.IP = net.IPv4(rsub_a, rsub_b, rsub_c, rsub_d)
							portScraper.rawRequest(portScraper.IP.String(), port)
							var service_info interface{} = nil
							var tls_certificate interface{} = nil
							if portScraper.ServiceInfo.Name != "" {
								service_info_byte, err := json.Marshal(portScraper.ServiceInfo)
								if err != nil {
									log.Error("Marshal ServiceInfo", err)
								}
								service_info = string(service_info_byte)
							}
							if len(portScraper.TLScertificate.DNSNames) != 0 || len(portScraper.TLScertificate.Organizations) != 0 {
								if len(portScraper.TLScertificate.DNSNames) == 0 { // Avoid arry: null
									portScraper.TLScertificate.DNSNames = make([]string, 0)
								}
								if len(portScraper.TLScertificate.Organizations) == 0 { // Avoid arry: null
									portScraper.TLScertificate.Organizations = make([]string, 0)
								}
								tls_certificate_byte, err := json.Marshal(portScraper.TLScertificate)
								if err != nil {
									log.Error("Marshal TLScertificate", err)
								}
								tls_certificate = string(tls_certificate_byte)
							}

							if portScraper.Open {
								log.Info(portScraper, "--", portScraper.IP.To16())
								_, err := sqldb.Exec(sqldb.DB(), "INSERT INTO scan VALUES ( NULL, INET_ATON(?), ?, ?, ?, ?, ?, ?, ?, ?, ?,?,?,?,?,?,?,NOW())",
									portScraper.IP.String(),
									portScraper.Port,
									portScraper.Open,
									portScraper.Latency,
									portScraper.HTTPstatus,
									portScraper.HTTPtitle,
									portScraper.HTTPserver,
									portScraper.HTTPfavicon,
									service_info,
									portScraper.TLS,
									tls_certificate,
									geoIP.CountryCode,
									geoIP.District,
									geoIP.City,
									geoIP.Latitude,
									geoIP.Longitude,
								)
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

func (portScraper *PortScraper) rawRequest(host string, port string) {

	portI, err := strconv.Atoi(port)
	if err != nil {
		log.Error("Wrong port format:", err)
	}

	portScraper.Port = uint16(portI)
	host_port := net.JoinHostPort(host, port)

	// TCP non encrypted
	start := time.Now()
	conn, err := net.DialTimeout("tcp", host_port, tcpTimeout)
	if err != nil {
		log.Debug("TCP Connecting error:", err)
		return
	}

	if conn != nil {
		defer conn.Close()
		log.Debug("OPENED:", host_port)
		portScraper.Open = true
		portScraper.Latency = time.Since(start)
		conn.SetReadDeadline(time.Now().Add(tcpTimeout))

		// Select the serivce based on the Port number
		portScraper.ServiceInfo, err = portScraper.ServiceSelector(conn, port)
		if err == nil {
			return
		}

		err = portScraper.httpRequest(host_port, false)
		if err != nil {
			log.Debug("TCP HTTP error:", err)
		} else {
			if portScraper.HTTPstatus < 400 && portScraper.HTTPstatus != 0 {
				return
			}
		}
	}
	portScraper.TLS = false

	// TCP + TLS encrypted
	conf := &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS10,
	}
	ctx, cancel := context.WithTimeout(context.Background(), tcpTimeout)
	d := tls.Dialer{
		Config: conf,
	}
	conn, err = d.DialContext(ctx, "tcp", host_port)
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
				portScraper.TLScertificate.DNSNames = v.DNSNames
				portScraper.TLScertificate.Organizations = v.Subject.Organization

			}

		}
		log.Debug("TCP+TLS ServerName:", state.ServerName)

		portScraper.TLS = true
		err = portScraper.httpRequest(host_port, true)
		if err != nil {
			log.Debug("TCP+TLS HTTP error:", err)
		}
	}

}

func (portScraper *PortScraper) httpRequest(host_port string, secure bool) error {
	var http_p string
	if secure {
		http_p = "https"
	} else {
		http_p = "http"
	}

	tr := &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		TLSHandshakeTimeout:   httpTimeout,
		IdleConnTimeout:       httpTimeout,
		ResponseHeaderTimeout: httpTimeout,
		ExpectContinueTimeout: httpTimeout,
	}
	client := &http.Client{Timeout: httpTimeout, Transport: tr}

	resp, err := client.Get(http_p + "://" + host_port)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Limit response body size to 10Mbytes
	limitedReader := &io.LimitedReader{R: resp.Body, N: 10000000}

	log.Debug("ENDPOINT: ", host_port, "---", resp.Header.Get("Server"), "---", resp.StatusCode)
	portScraper.HTTPstatus = uint16(resp.StatusCode)
	portScraper.HTTPserver = resp.Header.Get("Server")

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(limitedReader)
	if err != nil {
		return err
	}

	// Find the review items
	title := doc.Find("title").Text()
	if len(title) >= 30 {
		title = title[:30]
	}
	log.Debug("TITLE: ", title)
	portScraper.HTTPtitle = title
	client.CloseIdleConnections()

	//FavICON
	b, err := getFavIcon(client, http_p+"://"+host_port+"/favicon.ico")
	if b != nil && err == nil {
		portScraper.HTTPfavicon = int32(mmh3.Hash32(standBase64(b)))
	} else {
		var faviconURLPath string
		doc.Find("link").EachWithBreak(func(i int, s *goquery.Selection) bool {
			href, ok_href := s.Attr("href")
			rel, ok_rel := s.Attr("rel")
			if ok_href && ok_rel && rel == "icon" {
				faviconURLPath = href
				return false
			}
			return true
		})
		if faviconURLPath != "" {
			b, err := getFavIcon(client, http_p+"://"+host_port+faviconURLPath)
			if b != nil && err == nil {
				portScraper.HTTPfavicon = int32(mmh3.Hash32(standBase64(b)))
			}
		}

	}

	return nil
}

func getFavIcon(client *http.Client, endpoint string) ([]byte, error) {
	resp, err := client.Get(endpoint)
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

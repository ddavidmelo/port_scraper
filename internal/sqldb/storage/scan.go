package storage

import (
	"encoding/json"
	"net"
	"port_scraper/internal/sqldb"
	"time"
)

type ServiceInfo struct {
	Name    string            `json:"name"`
	Details map[string]string `json:"details"`
}

type TLScertificate struct {
	DNSNames      []string `json:"dnsNames"`
	Organizations []string `json:"organizations"`
}

type Scan struct {
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
	CountryCode    string
	District       string
	City           string
	Latitude       float32
	Longitude      float32
}

func InsertScanResult(row Scan) error {
	var serviceInfo any
	var tlsCertificate any

	if len(row.ServiceInfo.Details) != 0 {
		serviceInfoByte, err := json.Marshal(row.ServiceInfo)
		if err != nil {
			return err
		}
		serviceInfo = string(serviceInfoByte)
	}

	if len(row.TLScertificate.DNSNames) != 0 || len(row.TLScertificate.Organizations) != 0 {
		if len(row.TLScertificate.DNSNames) == 0 { // Avoid arry: null
			row.TLScertificate.DNSNames = make([]string, 0)
		}
		if len(row.TLScertificate.Organizations) == 0 { // Avoid arry: null
			row.TLScertificate.Organizations = make([]string, 0)
		}
		tlsCertificateByte, err := json.Marshal(row.TLScertificate)
		if err != nil {
			return err
		}
		tlsCertificate = string(tlsCertificateByte)
	}

	_, err := sqldb.Exec(sqldb.DB(), "INSERT INTO scan VALUES ( NULL, INET_ATON(?), ?, ?, ?, ?, ?, ?, ?, ?, ?,?,?,?,?,?,?,NOW())",
		row.IP.String(),
		row.Port,
		row.Open,
		row.Latency,
		row.HTTPstatus,
		row.HTTPtitle,
		row.HTTPserver,
		row.HTTPfavicon,
		serviceInfo,
		row.TLS,
		tlsCertificate,
		row.CountryCode,
		row.District,
		row.City,
		row.Latitude,
		row.Longitude,
	)
	return err
}

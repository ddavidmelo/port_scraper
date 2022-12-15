package scan

import (
	"bufio"
	"errors"
	"net"
	"port_scraper/internal/sqldb/storage"
	"regexp"
)

// type ServiceInfo struct {
// 	Name    string            `json:"name"`
// 	Details map[string]string `json:"details"`
// }

func ServiceSelector(conn net.Conn, port string) (storage.ServiceInfo, error) {
	switch port {
	case "22":
		return ssh(conn), nil
	case "3306":
		return mysql(conn), nil
	default:
		return storage.ServiceInfo{}, errors.New("service not defined")
	}
}

func ssh(conn net.Conn) storage.ServiceInfo {
	var serviceInfo storage.ServiceInfo
	details := make(map[string]string)
	serviceInfo.Name = "ssh"

	buf := make([]byte, 1024)
	if read, err := bufio.NewReader(conn).Read(buf); err == nil && read > 0 {
		s := string(buf[0:read])
		re := regexp.MustCompile("\x53\x53\x48\x2D([^\x00].*?)\x0D")
		match := re.FindStringSubmatch(s)
		if len(match) > 0 {
			if len(match[1]) > 50 {
				match[1] = match[1][:50]
			}
			details["ServerName"] = match[1]
		}
	}

	serviceInfo.Details = details
	return serviceInfo
}

func mysql(conn net.Conn) storage.ServiceInfo {
	var serviceInfo storage.ServiceInfo
	details := make(map[string]string)
	serviceInfo.Name = "mysql"

	buf := make([]byte, 1024)
	if read, err := bufio.NewReader(conn).Read(buf); err == nil && read > 0 {
		s := string(buf[0:read])
		re := regexp.MustCompile("\x0A([^\x00].*?)\x00")
		match := re.FindStringSubmatch(s)
		if len(match) > 0 {
			if len(match[1]) > 50 {
				match[1] = match[1][:50]
			}
			details["ServerName"] = match[1]
		}
	}

	serviceInfo.Details = details
	return serviceInfo
}

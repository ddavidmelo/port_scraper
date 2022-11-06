package scraper

import (
	"bufio"
	"errors"
	"net"
	"regexp"
)

type ServiceInfo struct {
	Name    string            `json:"name"`
	Details map[string]string `json:"details"`
}

func (portScraper *PortScraper) ServiceSelector(conn net.Conn, port string) (ServiceInfo, error) {
	switch port {
	case "22":
		return ssh(conn), nil
	case "3306":
		return mysql(conn), nil
	default:
		return ServiceInfo{}, errors.New("service not defined")
	}
}

func ssh(conn net.Conn) ServiceInfo {
	var serviceInfo ServiceInfo
	details := make(map[string]string)
	serviceInfo.Name = "ssh"

	buf := make([]byte, 1024)
	if read, err := bufio.NewReader(conn).Read(buf); err == nil && read > 0 {
		s := string(buf[0:read])
		re := regexp.MustCompile(".+\x0a([^\x00]+)\x00.+")
		match := re.FindStringSubmatch(s)
		if len(match) > 0 {
			details["ServerName"] = match[1]
		}
	}
	// fmt.Fprintf(conn, "GET / HTTP/1.1\r\n\r\n")
	// status, err := bufio.NewReader(conn).ReadString('\n')
	// if err != nil {
	// 	log.Debug("TCP HTTP/1.1 error", err)
	// } else {
	// 	if strings.Contains(status, "SSH") {
	// 		details["ServerName"] = status
	// 	}
	// 	log.Debug("TCP HTTP/1.1: ", status)
	// }

	serviceInfo.Details = details
	return serviceInfo
}

func mysql(conn net.Conn) ServiceInfo {
	var serviceInfo ServiceInfo
	details := make(map[string]string)
	serviceInfo.Name = "mysql"

	buf := make([]byte, 1024)
	if read, err := bufio.NewReader(conn).Read(buf); err == nil && read > 0 {
		s := string(buf[0:read])
		re := regexp.MustCompile(".+\x0a([^\x00]+)\x00.+")
		match := re.FindStringSubmatch(s)
		if len(match) > 0 {
			details["ServerName"] = match[1]
		}
	}

	serviceInfo.Details = details
	return serviceInfo
}

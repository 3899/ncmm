package webui

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

func isLoopbackHost(host string) bool {
	host = strings.TrimSpace(strings.TrimSuffix(host, "."))
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if zone := strings.LastIndexByte(host, '%'); zone >= 0 {
		host = host[:zone]
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isUnspecifiedHost(host string) bool {
	if strings.TrimSpace(host) == "" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsUnspecified()
}

func requestHostAllowed(listen, requestHost string) bool {
	listenHost, listenPort, err := net.SplitHostPort(listen)
	if err != nil {
		return false
	}
	host, port, err := splitRequestHost(requestHost)
	if err != nil {
		return false
	}
	if isUnspecifiedHost(listenHost) {
		return true
	}
	if port != "" && port != listenPort {
		return false
	}
	if isLoopbackHost(listenHost) {
		return isLoopbackHost(host)
	}
	return strings.EqualFold(strings.TrimSuffix(host, "."), strings.TrimSuffix(listenHost, "."))
}

func splitRequestHost(value string) (string, string, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "/\\ 	\r\n") {
		return "", "", fmt.Errorf("invalid Host header")
	}
	if host, port, err := net.SplitHostPort(value); err == nil {
		if host == "" || validatePort(port) != nil || !validHostName(host) {
			return "", "", fmt.Errorf("invalid Host header")
		}
		return host, port, nil
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		host := strings.TrimSuffix(strings.TrimPrefix(value, "["), "]")
		if net.ParseIP(strings.Split(host, "%")[0]) == nil {
			return "", "", fmt.Errorf("invalid Host header")
		}
		return host, "", nil
	}
	if strings.Contains(value, ":") || !validHostName(value) {
		return "", "", fmt.Errorf("invalid Host header")
	}
	return value, "", nil
}

func validHostName(host string) bool {
	plain := host
	if zone := strings.LastIndexByte(plain, '%'); zone >= 0 {
		plain = plain[:zone]
	}
	if net.ParseIP(plain) != nil {
		return true
	}
	host = strings.TrimSuffix(host, ".")
	if host == "" || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if r < 'a' || r > 'z' {
				if r < 'A' || r > 'Z' {
					if r < '0' || r > '9' {
						if r != '-' {
							return false
						}
					}
				}
			}
		}
	}
	return true
}

func validatePort(port string) error {
	value, err := strconv.Atoi(port)
	if err != nil || value < 1 || value > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

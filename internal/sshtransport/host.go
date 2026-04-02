package sshtransport

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// ParsedHost is the normalized host/port form used by SSH dialers and launchers.
type ParsedHost struct {
	Host    string
	Port    int
	Address string
	Display string
}

// ParseHost normalizes a user-entered SSH target and applies defaultPort when
// the input omits one.
func ParseHost(host string, defaultPort int) (ParsedHost, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return ParsedHost{}, fmt.Errorf("host is required")
	}
	if defaultPort <= 0 {
		defaultPort = defaultSSHPort
	}

	if strings.HasPrefix(host, "[") {
		if parsedHost, parsedPort, err := net.SplitHostPort(host); err == nil {
			return buildParsedHost(parsedHost, parsedPort, host)
		}
		if strings.HasSuffix(host, "]") {
			return buildParsedHost(strings.TrimSuffix(strings.TrimPrefix(host, "["), "]"), strconv.Itoa(defaultPort), host)
		}
		return ParsedHost{}, fmt.Errorf("invalid host %q", host)
	}

	if parsedHost, parsedPort, err := net.SplitHostPort(host); err == nil {
		return buildParsedHost(parsedHost, parsedPort, host)
	}

	if strings.Count(host, ":") == 1 {
		hostPart, portPart, _ := strings.Cut(host, ":")
		return buildParsedHost(hostPart, portPart, host)
	}

	if strings.Count(host, ":") > 1 {
		if ip := net.ParseIP(host); ip == nil {
			return ParsedHost{}, fmt.Errorf("invalid host %q", host)
		}
	}

	return buildParsedHost(host, strconv.Itoa(defaultPort), host)
}

func buildParsedHost(hostPart, portPart, original string) (ParsedHost, error) {
	hostPart = strings.TrimSpace(hostPart)
	if hostPart == "" {
		return ParsedHost{}, fmt.Errorf("host is required")
	}

	port, err := strconv.Atoi(portPart)
	if err != nil || port < 1 || port > 65535 {
		return ParsedHost{}, fmt.Errorf("invalid port in host %q", original)
	}

	address := net.JoinHostPort(hostPart, strconv.Itoa(port))
	return ParsedHost{
		Host:    hostPart,
		Port:    port,
		Address: address,
		Display: address,
	}, nil
}

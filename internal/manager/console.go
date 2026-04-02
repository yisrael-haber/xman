package manager

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// VMConsoleInfo resolves console launch diagnostics for the given VM.
func (m *Manager) VMConsoleInfo(vmRef string) (ConsoleLaunchInfo, error) {
	cb, err := m.getConsoleBackend()
	if err != nil {
		return ConsoleLaunchInfo{}, err
	}
	info, err := cb.ConsoleInfo(m.operationContext(), vmRef)
	if err != nil {
		return ConsoleLaunchInfo{}, err
	}
	info.DisplayURL = redactConsoleURL(info.URL)
	info.VCenterCheck = endpointCheckForURL("vCenter page", info.VCenterURL)
	info.ConsoleHostCheck = endpointCheckForHost("Console host", info.ConsoleHost, consolePortForURL(info.VCenterURL))
	return info, nil
}

// VMConsoleURL resolves a fresh browser console URL for the given VM.
func (m *Manager) VMConsoleURL(vmRef string) (string, error) {
	info, err := m.VMConsoleInfo(vmRef)
	if err != nil {
		return "", err
	}
	return info.URL, nil
}

func redactConsoleURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	if ticket := q.Get("sessionTicket"); ticket != "" {
		q.Set("sessionTicket", redactTicket(ticket))
		u.RawQuery = q.Encode()
	}
	return u.String()
}

func redactTicket(ticket string) string {
	if len(ticket) <= 12 {
		return "<redacted>"
	}
	return ticket[:6] + "..." + ticket[len(ticket)-4:]
}

func endpointCheckForURL(name, raw string) EndpointCheck {
	check := EndpointCheck{Name: name}
	u, err := url.Parse(raw)
	if err != nil {
		check.Error = fmt.Sprintf("invalid URL: %v", err)
		return check
	}
	host := u.Hostname()
	if host == "" {
		check.Error = "missing host"
		return check
	}
	port := u.Port()
	if port == "" {
		switch strings.ToLower(u.Scheme) {
		case "http":
			port = "80"
		default:
			port = "443"
		}
	}
	return endpointCheckForAddress(name, net.JoinHostPort(host, port))
}

func endpointCheckForHost(name, host string, port int) EndpointCheck {
	check := EndpointCheck{Name: name}
	host = strings.TrimSpace(host)
	if host == "" {
		check.Error = "missing host"
		return check
	}
	if port <= 0 {
		port = 443
	}
	return endpointCheckForAddress(name, net.JoinHostPort(host, fmt.Sprintf("%d", port)))
}

func endpointCheckForAddress(name, address string) EndpointCheck {
	check := EndpointCheck{Name: name, Address: address}
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		check.Error = err.Error()
		return check
	}
	_ = conn.Close()
	check.Reachable = true
	return check
}

func consolePortForURL(raw string) int {
	u, err := url.Parse(raw)
	if err != nil {
		return 443
	}
	if port := u.Port(); port != "" {
		if parsed, err := strconv.Atoi(port); err == nil {
			return parsed
		}
	}
	if strings.EqualFold(u.Scheme, "http") {
		return 80
	}
	return 443
}

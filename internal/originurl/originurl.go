package originurl

import (
	"fmt"
	"net/netip"
	"net/url"
	"strings"
)

// Validate checks that raw is an HTTP(S) origin safe to embed in shell snippets.
func Validate(raw string, name string) error {
	if strings.HasPrefix(raw, "-") {
		return fmt.Errorf("%s must not start with '-'", name)
	}
	origin, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse %s: %w", name, err)
	}
	if origin.Scheme != "http" && origin.Scheme != "https" {
		return fmt.Errorf("%s must use http or https", name)
	}
	if origin.Host == "" {
		return fmt.Errorf("%s host is required", name)
	}
	if origin.Scheme == "http" && !isLocalHost(origin.Hostname()) {
		return fmt.Errorf("%s must use https unless the host is localhost or loopback", name)
	}
	if origin.User != nil {
		return fmt.Errorf("%s must not include userinfo", name)
	}
	if origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return fmt.Errorf("%s must be an origin without path, query, or fragment", name)
	}
	if !isShellSafeHost(origin.Host) {
		return fmt.Errorf("%s host contains unsafe characters", name)
	}
	return nil
}

func isLocalHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return addr.IsLoopback()
}

func isShellSafeHost(host string) bool {
	for _, char := range host {
		if char >= 'a' && char <= 'z' {
			continue
		}
		if char >= 'A' && char <= 'Z' {
			continue
		}
		if char >= '0' && char <= '9' {
			continue
		}
		switch char {
		case '.', '-', ':', '[', ']', '%':
			continue
		default:
			return false
		}
	}
	return true
}

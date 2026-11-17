package middleware

import (
	"fmt"
	"net"
	"strings"

	"github.com/labstack/echo/v4"
)

// NewIPExtractor builds an X-Forwarded-For extractor for the given proxy addresses.
// Each entry may be a CIDR or a single IP. An empty list means the service is
// reached directly, so no extractor is installed.
func NewIPExtractor(ranges []string) (echo.IPExtractor, error) {
	if len(ranges) == 0 {
		return nil, nil
	}

	options := make([]echo.TrustOption, 0, len(ranges)+3)
	options = append(options,
		echo.TrustLoopback(false),
		echo.TrustLinkLocal(false),
		echo.TrustPrivateNet(false),
	)
	for _, raw := range ranges {
		ipRange, err := parseTrustIPRange(raw)
		if err != nil {
			return nil, err
		}
		options = append(options, echo.TrustIPRange(ipRange))
	}

	return echo.ExtractIPFromXFFHeader(options...), nil
}

func parseTrustIPRange(raw string) (*net.IPNet, error) {
	value := strings.TrimSpace(raw)
	if _, ipRange, err := net.ParseCIDR(value); err == nil {
		return ipRange, nil
	}

	ip := net.ParseIP(value)
	if ip == nil {
		return nil, fmt.Errorf("http.trustIPRange contains invalid IP or CIDR %q", raw)
	}
	if v4 := ip.To4(); v4 != nil {
		return &net.IPNet{IP: v4, Mask: net.CIDRMask(32, 32)}, nil
	}

	return &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)}, nil
}

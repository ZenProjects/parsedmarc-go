package utils

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/oschwald/geoip2-golang"
)

// DefaultString returns the default value if the string is empty
func DefaultString(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

// ParseTimestamp converts Unix timestamp string to time.Time
func ParseTimestamp(timestamp string) (time.Time, error) {
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timestamp: %w", err)
	}
	return time.Unix(ts, 0).UTC(), nil
}

// GeoLocation represents geolocation information
type GeoLocation struct {
	Country string
	City    string
	ASN     uint
	ISP     string
}

// GetGeoLocation gets geolocation information for an IP address
func GetGeoLocation(ipAddress, dbPath string) (*GeoLocation, error) {
	db, err := geoip2.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open GeoIP database: %w", err)
	}
	defer db.Close()

	ip := net.ParseIP(ipAddress)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ipAddress)
	}

	city, err := db.City(ip)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup IP: %w", err)
	}

	geo := &GeoLocation{
		Country: city.Country.Names["en"],
		City:    city.City.Names["en"],
	}

	// Try to get ISP info if available
	if city.Traits.IsAnonymousProxy {
		geo.ISP = "Anonymous Proxy"
	} else if city.Traits.IsSatelliteProvider {
		geo.ISP = "Satellite Provider"
	}

	return geo, nil
}

// GetReverseDNS performs reverse DNS lookup
func GetReverseDNS(ipAddress string, nameservers []string, timeoutSec int) (string, error) {
	c := dns.Client{
		Timeout: time.Duration(timeoutSec) * time.Second,
	}

	// Create reverse DNS query
	addr, err := dns.ReverseAddr(ipAddress)
	if err != nil {
		return "", fmt.Errorf("failed to create reverse address: %w", err)
	}

	m := new(dns.Msg)
	m.SetQuestion(addr, dns.TypePTR)

	// Try each nameserver
	for _, ns := range nameservers {
		server := ns
		if !strings.Contains(server, ":") {
			server = server + ":53"
		}

		r, _, err := c.Exchange(m, server)
		if err != nil {
			continue
		}

		if r.Rcode != dns.RcodeSuccess {
			continue
		}

		for _, ans := range r.Answer {
			if ptr, ok := ans.(*dns.PTR); ok {
				hostname := strings.TrimSuffix(ptr.Ptr, ".")
				return hostname, nil
			}
		}
	}

	return "", fmt.Errorf("no PTR records found")
}

// GetBaseDomain extracts base domain from hostname
func GetBaseDomain(hostname string) string {
	if hostname == "" {
		return ""
	}

	parts := strings.Split(hostname, ".")
	if len(parts) < 2 {
		return hostname
	}

	// Return last two parts (e.g., "example.com" from "mail.example.com")
	return strings.Join(parts[len(parts)-2:], ".")
}

// IsValidIPAddress checks if string is a valid IP address
func IsValidIPAddress(ip string) bool {
	return net.ParseIP(ip) != nil
}

// NormalizeEmail converts email to lowercase and trims spaces
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// NormalizeDomain converts domain to lowercase and trims spaces
func NormalizeDomain(domain string) string {
	return strings.ToLower(strings.TrimSpace(domain))
}

// SanitizeString removes dangerous characters from string
func SanitizeString(s string) string {
	// Remove null bytes and control characters
	result := strings.Map(func(r rune) rune {
		if r == 0 || (r > 0 && r < 32 && r != 9 && r != 10 && r != 13) {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(result)
}

// StringSliceContains checks if string slice contains a value
func StringSliceContains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
package publicsite

import (
	"net/netip"
	"net/url"
	"path"
	"regexp"
	"strings"
)

var (
	externalControlPattern = regexp.MustCompile(`[\x00-\x1f\x7f]`)
	numericHostPattern     = regexp.MustCompile(`^[0-9.]+$`)
	hexHostPattern         = regexp.MustCompile(`(?i)^0x[0-9a-f]+$`)
	unsafeIPv4Prefixes     = []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"),
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("100.64.0.0/10"),
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("169.254.0.0/16"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.0.0.0/24"),
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("192.168.0.0/16"),
		netip.MustParsePrefix("198.18.0.0/15"),
		netip.MustParsePrefix("198.51.100.0/24"),
		netip.MustParsePrefix("203.0.113.0/24"),
		netip.MustParsePrefix("224.0.0.0/4"),
	}
)

func NormalizeExternalURL(raw string, rejectLocalHost bool) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" || externalControlPattern.MatchString(value) {
		return "", false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", false
	}
	if parsed.User != nil {
		return "", false
	}
	if rejectLocalHost && IsUnsafeExternalHost(parsed.Hostname()) {
		return "", false
	}
	return value, true
}

func IsUnsafeExternalHost(host string) bool {
	normalized := strings.ToLower(strings.Trim(host, "[] \t\n\r\x00\v."))
	if normalized == "" {
		return true
	}
	if normalized == "localhost" || strings.HasSuffix(normalized, ".localhost") || strings.HasSuffix(normalized, ".local") {
		return true
	}
	if addr, err := netip.ParseAddr(normalized); err == nil {
		return isUnsafeExternalIP(addr)
	}
	if numericHostPattern.MatchString(normalized) || hexHostPattern.MatchString(normalized) {
		return true
	}
	return false
}

func ExternalImageMimeType(raw string) (string, bool) {
	normalized, ok := NormalizeExternalURL(raw, true)
	if !ok {
		return "", false
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return "", false
	}
	switch strings.ToLower(strings.TrimPrefix(path.Ext(parsed.Path), ".")) {
	case "gif":
		return "image/gif", true
	case "jpg", "jpeg":
		return "image/jpeg", true
	case "png":
		return "image/png", true
	default:
		return "", false
	}
}

func isUnsafeExternalIP(addr netip.Addr) bool {
	if addr.Is4In6() {
		addr = addr.Unmap()
	}
	if addr.Is4() {
		for _, prefix := range unsafeIPv4Prefixes {
			if prefix.Contains(addr) {
				return true
			}
		}
		return false
	}
	return !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast()
}

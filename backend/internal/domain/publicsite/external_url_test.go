package publicsite

import "testing"

func TestNormalizeExternalURLMatchesLegacyPolicy(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		reject bool
		ok     bool
	}{
		{name: "public http", raw: "http://example.com/path", reject: true, ok: true},
		{name: "public https", raw: " https://cdn.example.com/a.png ", reject: true, ok: true},
		{name: "empty", raw: " ", reject: true},
		{name: "control", raw: "https://example.com/\nnext", reject: true},
		{name: "missing host", raw: "/relative/path", reject: true},
		{name: "javascript", raw: "javascript:alert(1)", reject: true},
		{name: "userinfo", raw: "http://example.com@127.0.0.1/image.png", reject: true},
		{name: "localhost", raw: "http://localhost:8888/game", reject: true},
		{name: "local suffix", raw: "http://cache.local/image.png", reject: true},
		{name: "loopback ipv4", raw: "http://127.0.0.1/image.png", reject: true},
		{name: "mapped loopback", raw: "http://[::ffff:127.0.0.1]/image.png", reject: true},
		{name: "metadata", raw: "http://169.254.169.254/latest/meta-data/", reject: true},
		{name: "documentation range", raw: "http://203.0.113.10/image.png", reject: true},
		{name: "numeric host", raw: "http://2130706433/image.png", reject: true},
		{name: "hex host", raw: "http://0x7f000001/image.png", reject: true},
		{name: "localhost allowed when not rejected", raw: "http://localhost:8888/game", reject: false, ok: true},
		{name: "public ipv4", raw: "http://8.8.8.8/image.png", reject: true, ok: true},
		{name: "public ipv6", raw: "http://[2001:4860:4860::8888]/image.png", reject: true, ok: true},
		{name: "private ipv6", raw: "http://[fd00::1]/image.png", reject: true},
		{name: "link local ipv6", raw: "http://[fe80::1]/image.png", reject: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := NormalizeExternalURL(tt.raw, tt.reject)
			if ok != tt.ok {
				t.Fatalf("unexpected ok=%v url=%q", ok, got)
			}
			if ok && got != "http://example.com/path" && tt.name == "public http" {
				t.Fatalf("unexpected normalized URL: %q", got)
			}
		})
	}
}

func TestIsUnsafeExternalHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{host: "", want: true},
		{host: ".localhost.", want: true},
		{host: "192.168.10.4", want: true},
		{host: "8.8.8.8"},
		{host: "2001:4860:4860::8888"},
		{host: "ff02::1", want: true},
		{host: "example.com"},
	}
	for _, tt := range tests {
		if got := IsUnsafeExternalHost(tt.host); got != tt.want {
			t.Fatalf("IsUnsafeExternalHost(%q)=%v want %v", tt.host, got, tt.want)
		}
	}
}

func TestExternalImageMimeType(t *testing.T) {
	tests := []struct {
		raw  string
		mime string
		ok   bool
	}{
		{raw: "https://example.com/logo.gif", mime: "image/gif", ok: true},
		{raw: "https://example.com/logo.JPG", mime: "image/jpeg", ok: true},
		{raw: "https://example.com/logo.jpeg?x=1", mime: "image/jpeg", ok: true},
		{raw: "https://example.com/logo.png", mime: "image/png", ok: true},
		{raw: "https://example.com/logo.svg"},
		{raw: "http://127.0.0.1/logo.png"},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			mime, ok := ExternalImageMimeType(tt.raw)
			if ok != tt.ok || mime != tt.mime {
				t.Fatalf("unexpected mime=%q ok=%v", mime, ok)
			}
		})
	}
}

package link

import (
	"net/url"
	"strings"
)

// OpaqueShareSchemes are links understood by dedicated clients rather than
// Xray. 3x-ui keeps these values byte-for-byte in raw/HTML subscriptions and
// deliberately omits them from Xray JSON and Clash/Mihomo output.
var OpaqueShareSchemes = map[string]struct{}{
	"openflux": {},
	"wdtt":     {},
	"csqtt":    {},
}

// IsOpaqueShareLink reports whether raw is a structurally valid share link for
// one of the sidecar protocols supported by this fork. The payload stays
// opaque: validation must not decode or rewrite secrets owned by another app.
func IsOpaqueShareLink(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "\r\n\t") {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if _, ok := OpaqueShareSchemes[strings.ToLower(u.Scheme)]; !ok {
		return false
	}
	return u.Opaque != "" || u.Host != "" || strings.Trim(u.Path, "/") != ""
}

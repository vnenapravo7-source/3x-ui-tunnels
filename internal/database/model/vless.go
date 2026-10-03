package model

// CanonicalVlessFlow translates values accepted by older panel releases to
// the spelling understood by current Xray and strict subscription clients.
func CanonicalVlessFlow(flow string) string {
	if flow == "xtls-rprx-vision-udp443" {
		return "xtls-rprx-vision"
	}
	return flow
}

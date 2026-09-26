package link

import "testing"

func TestIsOpaqueShareLink(t *testing.T) {
	tests := []struct {
		name string
		link string
		want bool
	}{
		{"OpenFlux v1", "openflux://v1/eyJzZWNyZXQiOiJ4In0", true},
		{"WDTT", "wdtt://example.invalid:443/config", true},
		{"CSQTT", "csqtt://example.invalid:443?password=secret", true},
		{"case insensitive", "OPENFLUX://v1/payload", true},
		{"empty payload", "wdtt://", false},
		{"unsupported", "unknown://payload", false},
		{"control character", "csqtt://host/config\nsecond", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOpaqueShareLink(tt.link); got != tt.want {
				t.Fatalf("IsOpaqueShareLink(%q) = %v, want %v", tt.link, got, tt.want)
			}
		})
	}
}

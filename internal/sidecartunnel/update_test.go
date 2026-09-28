package sidecartunnel

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestParseChecksum(t *testing.T) {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte("openflux")))
	got, err := parseChecksum([]byte(digest+"  openflux-linux-amd64\n"), "openflux-linux-amd64")
	if err != nil || got != digest {
		t.Fatalf("parseChecksum() = %q, %v", got, err)
	}
	if _, err := parseChecksum([]byte(digest+"  other\n"), "openflux-linux-amd64"); err == nil {
		t.Fatal("parseChecksum accepted another asset's digest")
	}
}

func TestOpenFluxAsset(t *testing.T) {
	for _, tc := range []struct{ goos, goarch, want string }{
		{"linux", "amd64", "openflux-linux-amd64"},
		{"linux", "arm64", "openflux-linux-arm64"},
		{"linux", "arm", "openflux-linux-armv7"},
	} {
		got, err := openFluxAsset(tc.goos, tc.goarch)
		if err != nil || got != tc.want {
			t.Fatalf("openFluxAsset(%q, %q) = %q, %v", tc.goos, tc.goarch, got, err)
		}
	}
	if _, err := openFluxAsset("windows", "amd64"); err == nil {
		t.Fatal("Windows server updater must be rejected")
	}
}

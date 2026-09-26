package sidecartunnel

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestFingerprintIgnoresGeneratedCupsCode(t *testing.T) {
	inst := Instance{
		ID:       7,
		Protocol: model.OpenFlux,
		Settings: Settings{Clients: []Client{{Password: strings.Repeat("01", 32)}}},
	}
	before := inst.fingerprint()
	inst.Settings.CupsCode = "runtime-room-code"
	if after := inst.fingerprint(); after != before {
		t.Fatalf("generated Cups room changed process fingerprint: %s != %s", after, before)
	}
}

func TestValidCupsCode(t *testing.T) {
	if !validCupsCode("WyJyb29tLWlkIl0") {
		t.Fatal("valid room list was rejected")
	}
	if validCupsCode("not-a-room-list") {
		t.Fatal("invalid room code was accepted")
	}
}

func TestEnsureWDTTKeysAreValidAndStable(t *testing.T) {
	dir := t.TempDir()
	if err := ensureWDTTKeys(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "wg-keys.dat")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(first)), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected four WDTT keys, got %d", len(lines))
	}
	for _, line := range lines {
		key, err := base64.StdEncoding.DecodeString(line)
		if err != nil || len(key) != 32 {
			t.Fatalf("invalid WDTT key: %v", err)
		}
	}
	if err := ensureWDTTKeys(dir); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("WDTT keys changed on restart")
	}
}

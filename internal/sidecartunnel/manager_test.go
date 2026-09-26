package sidecartunnel

import (
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

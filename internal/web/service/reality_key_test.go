package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"

	"golang.org/x/crypto/curve25519"
)

func realityTestKeypair(t *testing.T) (string, string) {
	t.Helper()
	privateRaw := bytes.Repeat([]byte{0x42}, 32)
	publicRaw, err := curve25519.X25519(privateRaw, curve25519.Basepoint)
	if err != nil {
		t.Fatalf("derive test public key: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(privateRaw), base64.RawURLEncoding.EncodeToString(publicRaw)
}

func TestDeriveRealityPublicKey(t *testing.T) {
	privateKey, wantPublicKey := realityTestKeypair(t)
	got, err := DeriveRealityPublicKey(privateKey)
	if err != nil {
		t.Fatalf("DeriveRealityPublicKey: %v", err)
	}
	if got != wantPublicKey {
		t.Fatalf("public key = %q, want %q", got, wantPublicKey)
	}
}

func TestNormalizeRealityPublicKeyRepairsStaleMetadata(t *testing.T) {
	privateKey, wantPublicKey := realityTestKeypair(t)
	stream := `{"network":"tcp","security":"reality","realitySettings":{"privateKey":"` + privateKey + `","settings":{"publicKey":"stale-public-key","fingerprint":"chrome"}}}`

	normalized, changed, err := normalizeRealityPublicKey(stream)
	if err != nil {
		t.Fatalf("normalizeRealityPublicKey: %v", err)
	}
	if !changed {
		t.Fatal("stale public key was not repaired")
	}

	var parsed struct {
		RealitySettings struct {
			Settings struct {
				PublicKey   string `json:"publicKey"`
				Fingerprint string `json:"fingerprint"`
			} `json:"settings"`
		} `json:"realitySettings"`
	}
	if err := json.Unmarshal([]byte(normalized), &parsed); err != nil {
		t.Fatalf("decode normalized stream: %v", err)
	}
	if parsed.RealitySettings.Settings.PublicKey != wantPublicKey {
		t.Fatalf("stored public key = %q, want %q", parsed.RealitySettings.Settings.PublicKey, wantPublicKey)
	}
	if parsed.RealitySettings.Settings.Fingerprint != "chrome" {
		t.Fatalf("unrelated REALITY setting changed: fingerprint = %q", parsed.RealitySettings.Settings.Fingerprint)
	}

	again, changedAgain, err := normalizeRealityPublicKey(normalized)
	if err != nil {
		t.Fatalf("second normalizeRealityPublicKey: %v", err)
	}
	if changedAgain || again != normalized {
		t.Fatal("normalization is not idempotent")
	}
}

func TestNormalizeRealityPublicKeyRejectsInvalidPrivateKey(t *testing.T) {
	stream := `{"security":"reality","realitySettings":{"privateKey":"not-a-key","settings":{"publicKey":"keep"}}}`
	if _, _, err := normalizeRealityPublicKey(stream); err == nil {
		t.Fatal("invalid REALITY private key was accepted")
	}
}

package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	"golang.org/x/crypto/curve25519"
)

// DeriveRealityPublicKey returns the Xray REALITY public key belonging to a
// base64-encoded X25519 private key. Xray uses raw URL-safe base64 without
// padding, while imported/legacy rows may carry either base64 alphabet.
func DeriveRealityPublicKey(privateKey string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(privateKey), "=")
	if trimmed == "" {
		return "", errors.New("empty REALITY private key")
	}
	var raw []byte
	var err error
	if strings.ContainsAny(trimmed, "+/") {
		raw, err = base64.RawStdEncoding.DecodeString(trimmed)
	} else {
		raw, err = base64.RawURLEncoding.DecodeString(trimmed)
	}
	if err != nil {
		return "", err
	}
	if len(raw) != curve25519.ScalarSize {
		return "", errors.New("REALITY private key must decode to 32 bytes")
	}
	publicKey, err := curve25519.X25519(raw, curve25519.Basepoint)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(publicKey), nil
}

// normalizeRealityPublicKey replaces the share-only publicKey cached inside a
// REALITY stream with the key derived from the private key used by the server.
// xray-core ignores the cached publicKey on the inbound, but clients rely on it
// in vless:// links; allowing the pair to drift yields a listener that answers
// TCP probes while every REALITY handshake fails.
func normalizeRealityPublicKey(streamSettings string) (string, bool, error) {
	if strings.TrimSpace(streamSettings) == "" {
		return streamSettings, false, nil
	}
	var stream map[string]any
	if err := json.Unmarshal([]byte(streamSettings), &stream); err != nil {
		return streamSettings, false, err
	}
	security, _ := stream["security"].(string)
	if security != "reality" {
		return streamSettings, false, nil
	}
	reality, _ := stream["realitySettings"].(map[string]any)
	if reality == nil {
		return streamSettings, false, nil
	}
	privateKey, _ := reality["privateKey"].(string)
	if strings.TrimSpace(privateKey) == "" {
		return streamSettings, false, nil
	}
	publicKey, err := DeriveRealityPublicKey(privateKey)
	if err != nil {
		return streamSettings, false, err
	}
	settings, _ := reality["settings"].(map[string]any)
	if settings == nil {
		settings = map[string]any{}
		reality["settings"] = settings
	}
	if stored, _ := settings["publicKey"].(string); stored == publicKey {
		return streamSettings, false, nil
	}
	settings["publicKey"] = publicKey
	normalized, err := json.Marshal(stream)
	if err != nil {
		return streamSettings, false, err
	}
	return string(normalized), true, nil
}

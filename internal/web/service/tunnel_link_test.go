package service

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/url"
	"strings"
	"testing"
)

func TestBuildWDTTPlusLink(t *testing.T) {
	link, err := BuildTunnelLink(TunnelLinkRequest{
		Protocol: "wdttplus", Name: "alice phone", Host: "vpn.example.com", Password: "p&ss",
		DTLSPort: 56000, WGPort: 56001, LocalPort: 9000,
		Hashes: []string{"https://vk.com/call/join/abc?x=1", "abc", "x+y"},
	})
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Scheme != "wdtt" || u.Host != "connect" || q.Get("password") != "p&ss" || q.Get("hashes") != "abc,x+y" {
		t.Fatalf("unexpected WDTT-Plus link: %s", link)
	}
}

func TestBuildCSQTTLinkUsesLiteralPlusSeparator(t *testing.T) {
	link, err := BuildTunnelLink(TunnelLinkRequest{
		Protocol: "csqtt", Name: "office-alice", Host: "2001:db8::1", PeerPort: 56000, Password: "p@ss", Hashes: []string{"a+b", "second"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(link, "hashes=a%2Bb+second") {
		t.Fatalf("hash separator/escaping is wrong: %s", link)
	}
	u, err := url.Parse(link)
	if err != nil || u.Query().Get("name") != "office-alice" {
		t.Fatalf("CSQTT link lost its display name: %s", link)
	}
}

func TestBuildOpenFluxLinkMatchesV1Payload(t *testing.T) {
	secret := strings.Repeat("01", 32)
	link, err := BuildTunnelLink(TunnelLinkRequest{
		Protocol: "openflux", Name: "office", Negotiate: true, Secret: secret,
		Context: "direct", Transports: []TunnelLinkTransport{{Type: "direct", Dial: "vpn.example.com:443"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded := strings.TrimPrefix(link, "openflux://v1/")
	packed, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	r := flate.NewReader(bytes.NewReader(packed))
	raw, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatal(err)
	}
	var share openFluxShare
	if err := json.Unmarshal(raw, &share); err != nil {
		t.Fatal(err)
	}
	if share.Name != "office" || share.Secret != secret || len(share.Transports) != 1 || share.Transports[0].Dial != "vpn.example.com:443" {
		t.Fatalf("unexpected OpenFlux payload: %#v", share)
	}
}

func TestBuildOpenFluxLinkUsesCanonicalCoreNormalization(t *testing.T) {
	secret := strings.Repeat("01", 32)
	link, err := BuildTunnelLink(TunnelLinkRequest{
		Protocol: "openflux", Name: " profile ", Negotiate: true, Codec: "batched", Secret: secret,
		Transports: []TunnelLinkTransport{
			{Type: "mailru", Name: "mailru", URL: " https://cloud.mail.ru/public/low ", Priority: 25},
			{Type: "yandex", URL: " https://docs.yandex.ru/edit/d/high ", Priority: 75},
			{Type: "direct", Dial: "vpn.example.com:443", Priority: 100},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded := strings.TrimPrefix(link, "openflux://v1/")
	packed, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	r := flate.NewReader(bytes.NewReader(packed))
	raw, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatal(err)
	}
	var share openFluxShare
	if err := json.Unmarshal(raw, &share); err != nil {
		t.Fatal(err)
	}
	if share.Name != "profile" || share.Codec != "" || share.Context != "https://docs.yandex.ru/edit/d/high" {
		t.Fatalf("non-canonical share: %#v", share)
	}
	if share.Transports[0].Name != "" {
		t.Fatalf("redundant transport name was retained: %#v", share.Transports[0])
	}
}

func TestBuildTunnelLinkRejectsIncompleteConfig(t *testing.T) {
	for _, req := range []TunnelLinkRequest{
		{Protocol: "wdttplus", Host: "example.com", DTLSPort: 56000, WGPort: 56001, LocalPort: 9000},
		{Protocol: "csqtt", Host: "bad host", PeerPort: 56000, Password: "x"},
		{Protocol: "openflux", Negotiate: true, Transports: []TunnelLinkTransport{{Type: "direct", Dial: "example.com:443"}}},
	} {
		if _, err := BuildTunnelLink(req); err == nil {
			t.Fatalf("expected validation error for %#v", req)
		}
	}
}

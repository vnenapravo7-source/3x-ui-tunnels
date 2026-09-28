package service

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/sidecartunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

type TunnelLinkTransport struct {
	Type     string `json:"type"`
	Name     string `json:"name,omitempty"`
	URL      string `json:"url,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Dial     string `json:"dial,omitempty"`
}

// TunnelLinkRequest is the deliberately small, client-side configuration
// surface for the sidecars supported by this fork. It produces import links;
// it does not install or execute third-party binaries.
type TunnelLinkRequest struct {
	Protocol   string                `json:"protocol"`
	Name       string                `json:"name,omitempty"`
	Host       string                `json:"host,omitempty"`
	Password   string                `json:"password,omitempty"`
	DTLSPort   int                   `json:"dtlsPort,omitempty"`
	WGPort     int                   `json:"wgPort,omitempty"`
	LocalPort  int                   `json:"localPort,omitempty"`
	PeerPort   int                   `json:"peerPort,omitempty"`
	Hashes     []string              `json:"hashes,omitempty"`
	Negotiate  bool                  `json:"negotiate,omitempty"`
	Codec      string                `json:"codec,omitempty"`
	Secret     string                `json:"secret,omitempty"`
	Context    string                `json:"context,omitempty"`
	Transports []TunnelLinkTransport `json:"transports,omitempty"`
}

type openFluxShare struct {
	Name       string                `json:"name,omitempty"`
	Negotiate  bool                  `json:"negotiate,omitempty"`
	Codec      string                `json:"codec,omitempty"`
	Secret     string                `json:"secret,omitempty"`
	Context    string                `json:"context,omitempty"`
	Transports []TunnelLinkTransport `json:"transports"`
}

const openFluxContextPlaceholder = "http://#"

func canonicalOpenFluxContext(explicit string, transports []TunnelLinkTransport) string {
	if explicit = strings.TrimSpace(explicit); explicit != "" {
		return explicit
	}
	best := -1
	for i, transport := range transports {
		if transport.URL == "" || transport.URL == openFluxContextPlaceholder {
			continue
		}
		switch transport.Type {
		case "cupsonline", "direct", "oneme":
			continue
		}
		if best < 0 || transport.Priority > transports[best].Priority {
			best = i
		}
	}
	if best >= 0 {
		return transports[best].URL
	}
	return openFluxContextPlaceholder
}

var tunnelDNSName = regexp.MustCompile(`(?i)^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$`)

func normalizeTunnelHost(raw string) (string, error) {
	host := strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(raw, "]"), "["))
	if host == "" || len(host) > 253 || strings.ContainsAny(host, "/\\@?# ") {
		return "", common.NewError("a public IP or DNS name is required")
	}
	if net.ParseIP(host) != nil {
		return host, nil
	}
	if !tunnelDNSName.MatchString(host) || strings.Contains(host, "..") {
		return "", common.NewError("invalid tunnel host")
	}
	return host, nil
}

func validTunnelPort(port int, field string) error {
	if port < 1 || port > 65535 {
		return common.NewError(field + " must be between 1 and 65535")
	}
	return nil
}

func cleanTunnelHashes(values []string, limit int) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, raw := range values {
		hash := strings.TrimSpace(raw)
		if i := strings.LastIndex(hash, "/"); i >= 0 {
			hash = hash[i+1:]
		}
		if i := strings.Index(hash, "?"); i >= 0 {
			hash = hash[:i]
		}
		hash = strings.TrimSpace(hash)
		if hash == "" || seen[hash] {
			continue
		}
		seen[hash] = true
		out = append(out, hash)
		if limit > 0 && len(out) == limit {
			break
		}
	}
	return out
}

func buildWDTTPlusLink(req TunnelLinkRequest) (string, error) {
	host, err := normalizeTunnelHost(req.Host)
	if err != nil {
		return "", err
	}
	for _, item := range []struct {
		port  int
		field string
	}{{req.DTLSPort, "dtlsPort"}, {req.WGPort, "wgPort"}, {req.LocalPort, "localPort"}} {
		if err := validTunnelPort(item.port, item.field); err != nil {
			return "", err
		}
	}
	if strings.TrimSpace(req.Password) == "" {
		return "", common.NewError("password is required")
	}
	q := url.Values{}
	q.Set("v", "1")
	q.Set("host", host)
	q.Set("dtls", strconv.Itoa(req.DTLSPort))
	q.Set("wg", strconv.Itoa(req.WGPort))
	q.Set("local", strconv.Itoa(req.LocalPort))
	q.Set("password", req.Password)
	if hashes := cleanTunnelHashes(req.Hashes, 6); len(hashes) > 0 {
		q.Set("hashes", strings.Join(hashes, ","))
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		q.Set("name", name)
	}
	return "wdtt://connect?" + q.Encode(), nil
}

func buildCSQTTLink(req TunnelLinkRequest) (string, error) {
	host, err := normalizeTunnelHost(req.Host)
	if err != nil {
		return "", err
	}
	if err := validTunnelPort(req.PeerPort, "peerPort"); err != nil {
		return "", err
	}
	if strings.TrimSpace(req.Password) == "" {
		return "", common.NewError("password is required")
	}
	q := url.Values{}
	q.Set("v", "2")
	q.Set("host", host)
	q.Set("peer", strconv.Itoa(req.PeerPort))
	q.Set("password", req.Password)
	if name := strings.TrimSpace(req.Name); name != "" {
		q.Set("name", name)
	}
	if hashes := cleanTunnelHashes(req.Hashes, 6); len(hashes) > 0 {
		// CSQTT assigns a literal '+' as the separator. Values.Encode would
		// escape it as %2B, so append the already escaped individual hashes.
		base := "csqtt://connect?" + q.Encode()
		parts := make([]string, 0, len(hashes))
		for _, hash := range hashes {
			parts = append(parts, url.QueryEscape(hash))
		}
		return base + "&hashes=" + strings.Join(parts, "+"), nil
	}
	return "csqtt://connect?" + q.Encode(), nil
}

func buildOpenFluxLink(req TunnelLinkRequest) (string, error) {
	codec := strings.TrimSpace(req.Codec)
	if codec == "" {
		codec = "batched"
	}
	if codec != "batched" && codec != "legacy" {
		return "", common.NewError("OpenFlux codec must be batched or legacy")
	}
	secret := strings.TrimSpace(req.Secret)
	if secret != "" {
		decoded, err := hex.DecodeString(secret)
		if err != nil || len(decoded) != 32 {
			return "", common.NewError("OpenFlux secret must be 64 hexadecimal characters")
		}
	}
	if req.Negotiate && secret == "" {
		return "", common.NewError("OpenFlux Session mode requires a secret")
	}
	if len(req.Transports) == 0 {
		return "", common.NewError("at least one OpenFlux transport is required")
	}
	allowed := map[string]bool{"direct": true, "cupsonline": true, "yandex": true, "vyandex": true, "boards": true, "mailru": true}
	transports := make([]TunnelLinkTransport, 0, len(req.Transports))
	for _, item := range req.Transports {
		item.Type = strings.ToLower(strings.TrimSpace(item.Type))
		item.Name = strings.TrimSpace(item.Name)
		item.URL = strings.TrimSpace(item.URL)
		item.Dial = strings.TrimSpace(item.Dial)
		if item.Name == item.Type {
			item.Name = ""
		}
		if !allowed[item.Type] {
			return "", common.NewError("unsupported OpenFlux transport: " + item.Type)
		}
		if item.Type == "direct" {
			if !req.Negotiate {
				return "", common.NewError("OpenFlux direct transport requires Session mode")
			}
			host, port, err := net.SplitHostPort(item.Dial)
			if err != nil {
				return "", common.NewError("OpenFlux direct dial must be host:port")
			}
			cleanHost, err := normalizeTunnelHost(host)
			if err != nil {
				return "", err
			}
			portNumber, _ := strconv.Atoi(port)
			if err := validTunnelPort(portNumber, "direct port"); err != nil {
				return "", err
			}
			item.Dial = net.JoinHostPort(cleanHost, port)
			item.URL = ""
		} else if item.URL == "" {
			return "", common.NewError("OpenFlux transport URL or room code is required")
		}
		transports = append(transports, item)
	}
	if len(transports) == 1 {
		transports[0].Priority = 0
	}
	contextValue := strings.TrimSpace(req.Context)
	if secret == "" {
		contextValue = ""
	} else if contextValue == "" {
		contextValue = canonicalOpenFluxContext("", transports)
	}
	share := openFluxShare{
		Name:       strings.TrimSpace(req.Name),
		Negotiate:  req.Negotiate,
		Secret:     secret,
		Context:    contextValue,
		Transports: transports,
	}
	if codec != "batched" {
		share.Codec = codec
	}
	raw, err := json.Marshal(share)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if link, coreErr := sidecartunnel.MakeOpenFluxLink(ctx, raw); coreErr == nil {
		return link, nil
	} else if !errors.Is(coreErr, sidecartunnel.ErrOpenFluxLinkToolUnavailable) {
		return "", coreErr
	}
	// Development/tests may not have a sidecar installed. Keep an exact v0.2
	// encoder fallback so those environments still produce canonical links.
	var packed bytes.Buffer
	w, err := flate.NewWriter(&packed, flate.BestCompression)
	if err != nil {
		return "", err
	}
	if _, err := w.Write(raw); err != nil {
		_ = w.Close()
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return "openflux://v1/" + base64.RawURLEncoding.EncodeToString(packed.Bytes()), nil
}

func BuildTunnelLink(req TunnelLinkRequest) (string, error) {
	switch strings.ToLower(strings.TrimSpace(req.Protocol)) {
	case "wdtt", "wdttplus", "wdtt-plus":
		return buildWDTTPlusLink(req)
	case "csqtt":
		return buildCSQTTLink(req)
	case "openflux":
		return buildOpenFluxLink(req)
	default:
		return "", fmt.Errorf("unsupported tunnel protocol %q", req.Protocol)
	}
}

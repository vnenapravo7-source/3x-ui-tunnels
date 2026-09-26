package sub

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestGetSubsIncludesManagedSidecarLinks(t *testing.T) {
	initSubDB(t)
	db := database.GetDB()
	const subID = "sidecar-sub"
	const openFluxKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cases := []struct {
		protocol model.Protocol
		port     int
		password string
		settings string
		prefix   string
	}{
		{model.OpenFlux, 39119, openFluxKey, `{"codec":"batched","negotiate":true,"transports":[{"type":"direct","priority":100}]}`, "openflux://"},
		{model.WDTT, 19222, "wdtt-secret", `{"wgPort":56001,"localPort":9000}`, "wdtt://"},
		{model.CSQTT, 54789, "csqtt-secret", `{}`, "csqtt://"},
	}
	for _, tc := range cases {
		email := string(tc.protocol) + "@example.com"
		inbound := &model.Inbound{Listen: "203.0.113.8", Port: tc.port, Protocol: tc.protocol, Enable: true, Settings: tc.settings}
		if err := db.Create(inbound).Error; err != nil {
			t.Fatalf("create %s inbound: %v", tc.protocol, err)
		}
		client := &model.ClientRecord{Email: email, SubID: subID, Enable: true, Password: tc.password}
		if err := db.Create(client).Error; err != nil {
			t.Fatalf("create %s client: %v", tc.protocol, err)
		}
		if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: inbound.Id}).Error; err != nil {
			t.Fatalf("attach %s client: %v", tc.protocol, err)
		}
	}

	links, emails, _, _, err := NewSubService("").GetSubs(subID, "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	if len(links) != len(cases) || len(emails) != len(cases) {
		t.Fatalf("got %d links and %d emails, want %d: %v", len(links), len(emails), len(cases), links)
	}
	for i, tc := range cases {
		if !strings.HasPrefix(links[i], tc.prefix) {
			t.Errorf("%s link = %q, want prefix %q", tc.protocol, links[i], tc.prefix)
		}
	}
}

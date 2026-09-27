package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestAddInboundRejectsSecondCSQTTAndKeepsClientsOnFirst(t *testing.T) {
	setupConflictDB(t)
	svc := &InboundService{}
	first := &model.Inbound{
		Tag:      "csqtt-main",
		Listen:   "",
		Port:     54789,
		Protocol: model.CSQTT,
		Settings: `{"clients":[{"email":"alice","password":"alice-secret"},{"email":"bob","password":"bob-secret"}]}`,
	}
	if _, _, err := svc.AddInbound(first); err != nil {
		t.Fatalf("create first CSQTT inbound: %v", err)
	}

	second := &model.Inbound{
		Tag:      "csqtt-second",
		Listen:   "",
		Port:     54790,
		Protocol: model.CSQTT,
		Settings: `{"clients":[{"email":"charlie","password":"charlie-secret"}]}`,
	}
	if _, _, err := svc.AddInbound(second); err == nil || !strings.Contains(err.Error(), "add clients to the existing CSQTT inbound") {
		t.Fatalf("second CSQTT inbound was not rejected with client guidance: %v", err)
	}

	clients, err := svc.ListClientsForInbound(first.Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(clients) != 2 || clients[0].Password == clients[1].Password {
		t.Fatalf("first CSQTT inbound did not retain two distinct clients: %#v", clients)
	}
}

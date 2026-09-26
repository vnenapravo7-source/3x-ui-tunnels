package service

import (
	"context"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func isSidecarTunnelProtocol(protocol model.Protocol) bool {
	return protocol == model.OpenFlux || protocol == model.WDTT || protocol == model.CSQTT
}

func (s *InboundService) applyLocalSidecarTunnel(inboundID int) {
	inbound, err := s.GetInbound(inboundID)
	if err != nil || inbound == nil || !isSidecarTunnelProtocol(inbound.Protocol) || inbound.NodeID != nil {
		return
	}
	rt, err := s.runtimeFor(inbound)
	if err != nil {
		return
	}
	payload := inbound
	if inbound.Enable {
		if built, buildErr := s.buildInboundForLocalRuntime(database.GetDB(), inbound); buildErr == nil {
			payload = built
		}
	}
	if err := rt.UpdateInbound(context.Background(), inbound, payload); err != nil {
		logger.Warningf("%s: apply failed for inbound %d: %v", inbound.Protocol, inboundID, err)
	}
}

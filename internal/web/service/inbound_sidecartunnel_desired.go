package service

import (
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/sidecartunnel"
)

func (s *InboundService) DesiredSidecarTunnelInstances() ([]sidecartunnel.Instance, error) {
	var inbounds []*model.Inbound
	if err := database.GetDB().Where("protocol IN ? AND enable = ? AND node_id IS NULL", []model.Protocol{model.OpenFlux, model.WDTT, model.CSQTT}, true).Find(&inbounds).Error; err != nil {
		return nil, err
	}
	instances := make([]sidecartunnel.Instance, 0, len(inbounds))
	for _, inbound := range inbounds {
		built, err := s.buildInboundForLocalRuntime(database.GetDB(), inbound)
		if err != nil {
			continue
		}
		if inst, ok := sidecartunnel.InstanceFromInbound(built); ok {
			instances = append(instances, inst)
		}
	}
	return instances, nil
}

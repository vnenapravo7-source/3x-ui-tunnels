package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/sidecartunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type SidecarTunnelJob struct {
	inboundService service.InboundService
}

func NewSidecarTunnelJob() *SidecarTunnelJob { return new(SidecarTunnelJob) }

func (j *SidecarTunnelJob) Run() {
	desired, err := j.inboundService.DesiredSidecarTunnelInstances()
	if err != nil {
		logger.Warning("sidecar tunnel job: get desired instances failed:", err)
		return
	}
	sidecartunnel.GetManager().Reconcile(desired)
}

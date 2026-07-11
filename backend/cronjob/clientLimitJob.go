package cronjob

import (
	"s-ui/logger"
	"s-ui/service"
)

type ClientLimitJob struct {
	service.RuntimeConfigService
}

func NewClientLimitJob() *ClientLimitJob {
	return &ClientLimitJob{}
}

func (s *ClientLimitJob) Run() {
	if _, err := s.RuntimeConfigService.Reconcile(false, true); err != nil {
		logger.Warning("Client limit enforcement failed: ", err)
	}
}

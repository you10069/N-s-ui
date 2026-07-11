package cronjob

import (
	"s-ui/logger"
	"s-ui/service"
	"time"
)

type ClientResetJob struct {
	service.ClientService
	location *time.Location
}

func NewClientResetJob(location *time.Location) *ClientResetJob {
	return &ClientResetJob{location: location}
}

func (s *ClientResetJob) Run() {
	now := time.Now()
	if s.location != nil {
		now = now.In(s.location)
	}
	resetAny, err := s.ClientService.ResetDueClients(now)
	if err != nil {
		logger.Warning("Monthly client traffic reset failed: ", err)
		return
	}
	if resetAny {
		if _, err = service.NewRuntimeConfigService().Reconcile(false, true); err != nil {
			logger.Warning("Restore users after monthly reset failed: ", err)
		}
	}
}

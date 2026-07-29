package application

import (
	"argus/internal/observability"
	"argus/internal/testsupport"
)

// testHarness wires a real Service to the shared in-memory fakes so tests can
// drive use cases end to end and then assert on persisted state.
type testHarness struct {
	service          *Service
	users            *testsupport.UserStore
	tokens           *testsupport.AuthTokenStore
	recovery         *testsupport.RecoveryDelivery
	projects         *testsupport.ProjectStore
	routes           *testsupport.RouteStore
	incidents        *testsupport.RouteIncidentStore
	imports          *testsupport.ImportStore
	outbox           *testsupport.OutboxStore
	projectIncidents *testsupport.ProjectIncidentStore
}

func newTestHarness() *testHarness {
	s := testsupport.NewStores()
	service := NewService(s.Legacy, s.Legacy, s.Legacy, s.Legacy, s.Legacy, s.Outbox, observability.NewLogStore(100),
		s.Users, s.Tokens, s.PasswordRecovery, s.RecoveryDelivery, s.Projects, s.Routes, s.Incidents, s.Imports, s.TelemetryCredentials, s.TelemetryIngress, s.TelemetryMappings, s.SLOs, s.Heartbeats)
	service.SetPrivateAgentStore(s.PrivateAgents)
	service.SetPrivateAgentResultStore(s.PrivateAgentResults)
	service.SetPrivateAgentAssignmentStore(s.PrivateAgentAssignments)
	service.SetProjectIncidentStore(s.ProjectIncidents)
	return &testHarness{service: service,
		users:            s.Users,
		tokens:           s.Tokens,
		recovery:         s.RecoveryDelivery,
		projects:         s.Projects,
		routes:           s.Routes,
		incidents:        s.Incidents,
		imports:          s.Imports,
		outbox:           s.Outbox,
		projectIncidents: s.ProjectIncidents,
	}
}

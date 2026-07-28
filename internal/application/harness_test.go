package application

import (
	"argus/internal/observability"
	"argus/internal/testsupport"
)

// testHarness wires a real Service to the shared in-memory fakes so tests can
// drive use cases end to end and then assert on persisted state.
type testHarness struct {
	service   *Service
	users     *testsupport.UserStore
	tokens    *testsupport.AuthTokenStore
	projects  *testsupport.ProjectStore
	routes    *testsupport.RouteStore
	incidents *testsupport.RouteIncidentStore
	imports   *testsupport.ImportStore
	outbox    *testsupport.OutboxStore
}

func newTestHarness() *testHarness {
	s := testsupport.NewStores()
	return &testHarness{
		service: NewService(s.Legacy, s.Legacy, s.Legacy, s.Legacy, s.Legacy, s.Outbox, observability.NewLogStore(100),
			s.Users, s.Tokens, s.Projects, s.Routes, s.Incidents, s.Imports, s.TelemetryCredentials, s.TelemetryIngress),
		users:     s.Users,
		tokens:    s.Tokens,
		projects:  s.Projects,
		routes:    s.Routes,
		incidents: s.Incidents,
		imports:   s.Imports,
		outbox:    s.Outbox,
	}
}

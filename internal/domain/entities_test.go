package domain

import "testing"

func TestNormalizeMonitor(t *testing.T) {
	m, err := NormalizeMonitor(Monitor{URL: "https://example.com", CheckIntervalSeconds: 30, MonitorType: MonitorTypeKeyword, ExpectedKeyword: strPtr("ok")})
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if m.MonitorType != MonitorTypeKeyword {
		t.Fatalf("unexpected monitor type: %s", m.MonitorType)
	}
}

func TestIncidentPolicy(t *testing.T) {
	if !IncidentPolicy(false, "down").ShouldOpen {
		t.Fatal("expected open transition")
	}
	if !IncidentPolicy(true, "up").ShouldResolve {
		t.Fatal("expected resolve transition")
	}
}

func strPtr(v string) *string { return &v }

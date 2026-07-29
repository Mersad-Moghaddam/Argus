package application

import (
	"context"
	"testing"

	"argus/internal/models"
)

func TestCreateAlertChannelRequiresSafeHTTPSWebhookTarget(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	for _, target := range []string{
		"http://hooks.example.test/alert",
		"https://user:secret@hooks.example.test/alert",
		"https://hooks.example.test/alert#fragment",
		"mailto:ops@example.test",
	} {
		if _, err := h.service.CreateAlertChannel(ctx, models.AlertChannel{Name: "ops", ChannelType: "webhook", Target: target, Enabled: true}); err == nil {
			t.Fatalf("target %q was accepted", target)
		}
	}
	if _, err := h.service.CreateAlertChannel(ctx, models.AlertChannel{Name: "ops", ChannelType: "webhook", Target: "https://hooks.example.test/alert", Enabled: true}); err != nil {
		t.Fatalf("valid webhook rejected: %v", err)
	}
}

package worker

import "testing"

func TestValidateTargetBlocksMetadata(t *testing.T) {
	if err := validateTarget("http://169.254.169.254/latest"); err == nil {
		t.Fatal("expected metadata endpoint to be blocked")
	}
}

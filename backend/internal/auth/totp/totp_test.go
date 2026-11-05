package totp

import (
	"testing"
	"time"
)

func TestVerifyCurrentAndAdjacentWindow(t *testing.T) {
	const secret = "JBSWY3DPEHPK3PXP"
	now := time.Unix(1_700_000_000, 0)
	code, err := generate(secret, now)
	if err != nil {
		t.Fatalf("generate() error = %v", err)
	}
	if !Verify(secret, code, now.Add(30*time.Second)) {
		t.Fatal("Verify() rejected adjacent-window code")
	}
	if Verify(secret, "000000", now) && code != "000000" {
		t.Fatal("Verify() accepted unrelated code")
	}
}

func TestMatchCounterReturnsAcceptedWindow(t *testing.T) {
	tests := []struct {
		name       string
		codeOffset time.Duration
		verifyAt   time.Time
		wantStep   int64
	}{
		{
			name:     "current step",
			verifyAt: time.Unix(1_700_000_000, 0),
			wantStep: time.Unix(1_700_000_000, 0).Unix() / 30,
		},
		{
			name:       "previous adjacent step",
			codeOffset: -30 * time.Second,
			verifyAt:   time.Unix(1_700_000_000, 0),
			wantStep:   time.Unix(1_700_000_000, 0).Add(-30*time.Second).Unix() / 30,
		},
	}

	const secret = "JBSWY3DPEHPK3PXP"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := generate(secret, tt.verifyAt.Add(tt.codeOffset))
			if err != nil {
				t.Fatalf("generate() error = %v", err)
			}
			step, ok := MatchCounter(secret, code, tt.verifyAt)
			if !ok {
				t.Fatal("MatchCounter() rejected valid code")
			}
			if step != tt.wantStep {
				t.Fatalf("MatchCounter() step = %d, want %d", step, tt.wantStep)
			}
		})
	}
}

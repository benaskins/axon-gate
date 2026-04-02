package gate

import (
	"testing"
	"time"

	rule "github.com/benaskins/axon-rule"
)

func TestTokenMatches(t *testing.T) {
	c := ResolutionCandidate{
		Approval: Approval{Token: "secret-token"},
		Token:    "secret-token",
	}
	if v := rule.New(ResolutionCandidate.TokenMatches).Check(c); !v.OK {
		t.Error("expected pass for matching token")
	}

	c.Token = "wrong-token"
	v := rule.New(ResolutionCandidate.TokenMatches).Check(c)
	if v.OK {
		t.Error("expected fail for mismatched token")
	}
	if _, ok := v.Context.(TokenMismatch); !ok {
		t.Errorf("expected TokenMismatch context, got %T", v.Context)
	}
}

func TestNotExpired(t *testing.T) {
	c := ResolutionCandidate{
		Approval: Approval{ExpiresAt: time.Now().Add(1 * time.Hour)},
		Now:      time.Now(),
	}
	if v := rule.New(ResolutionCandidate.NotExpired).Check(c); !v.OK {
		t.Error("expected pass for non-expired approval")
	}

	c.Approval.ExpiresAt = time.Now().Add(-1 * time.Hour)
	v := rule.New(ResolutionCandidate.NotExpired).Check(c)
	if v.OK {
		t.Error("expected fail for expired approval")
	}
	ctx, ok := v.Context.(ApprovalExpired)
	if !ok {
		t.Fatalf("expected ApprovalExpired context, got %T", v.Context)
	}
	if ctx.ExpiresAt != c.Approval.ExpiresAt {
		t.Error("expected ExpiresAt in violation context")
	}
}

func TestOwnerMatches(t *testing.T) {
	tests := []struct {
		name           string
		approvalUser   string
		sessionUser    string
		expectPass     bool
	}{
		{"matching users", "alice", "alice", true},
		{"empty approval user allows any", "", "alice", true},
		{"empty session user allows any", "alice", "", true},
		{"both empty", "", "", true},
		{"mismatched users", "alice", "bob", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := ResolutionCandidate{
				Approval: Approval{Username: tt.approvalUser},
				Username: tt.sessionUser,
			}
			v := rule.New(ResolutionCandidate.OwnerMatches).Check(c)
			if v.OK != tt.expectPass {
				t.Errorf("expected OK=%v, got OK=%v", tt.expectPass, v.OK)
			}
			if !v.OK {
				ctx, ok := v.Context.(OwnerMismatch)
				if !ok {
					t.Fatalf("expected OwnerMismatch context, got %T", v.Context)
				}
				if ctx.Expected != tt.approvalUser || ctx.Actual != tt.sessionUser {
					t.Errorf("expected Expected=%s Actual=%s, got Expected=%s Actual=%s",
						tt.approvalUser, tt.sessionUser, ctx.Expected, ctx.Actual)
				}
			}
		})
	}
}

func TestIsPending(t *testing.T) {
	c := ResolutionCandidate{
		Approval: Approval{Status: StatusPending},
	}
	if v := rule.New(ResolutionCandidate.IsPending).Check(c); !v.OK {
		t.Error("expected pass for pending approval")
	}

	c.Approval.Status = StatusApproved
	v := rule.New(ResolutionCandidate.IsPending).Check(c)
	if v.OK {
		t.Error("expected fail for already-resolved approval")
	}
	ctx, ok := v.Context.(NotPending)
	if !ok {
		t.Fatalf("expected NotPending context, got %T", v.Context)
	}
	if ctx.Status != StatusApproved {
		t.Errorf("expected approved in context, got %s", ctx.Status)
	}
}

func TestCanResolve_AllPass(t *testing.T) {
	c := ResolutionCandidate{
		Approval: Approval{
			Token:    "valid-token",
			Username: "alice",
			Status:   StatusPending,
			ExpiresAt: time.Now().Add(1 * time.Hour),
		},
		Token:    "valid-token",
		Username: "alice",
		Now:      time.Now(),
	}

	violations := CanResolve.Evaluate(c)
	if !violations.IsValid() {
		t.Errorf("expected no violations, got %v", violations.Codes())
	}
}

func TestCanResolve_MultipleViolations(t *testing.T) {
	c := ResolutionCandidate{
		Approval: Approval{
			Token:     "valid-token",
			Username:  "alice",
			Status:    StatusApproved,
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		},
		Token:    "wrong-token",
		Username: "bob",
		Now:      time.Now(),
	}

	violations := CanResolve.Evaluate(c)
	codes := violations.Codes()

	if len(codes) != 4 {
		t.Fatalf("expected 4 violations, got %d: %v", len(codes), codes)
	}

	expected := map[string]bool{
		"TokenMismatch":   true,
		"ApprovalExpired": true,
		"OwnerMismatch":   true,
		"NotPending":      true,
	}
	for _, code := range codes {
		if !expected[code] {
			t.Errorf("unexpected violation code: %s", code)
		}
	}
}

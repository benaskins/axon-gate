package gate

import (
	"crypto/subtle"
	"time"

	rule "github.com/benaskins/axon-rule"
)

// ResolutionCandidate holds all data needed to evaluate whether an approval
// can be resolved. Built from pre-fetched approval + request context.
type ResolutionCandidate struct {
	Approval Approval
	Token    string
	Username string
	Now      time.Time
}

type TokenMismatch struct{}

type ApprovalExpired struct {
	ExpiresAt time.Time
}

type OwnerMismatch struct {
	Expected string
	Actual   string
}

type NotPending struct {
	Status ApprovalStatus
}

// TokenMatches uses constant-time comparison to prevent timing attacks.
func (c ResolutionCandidate) TokenMatches() rule.Verdict {
	if subtle.ConstantTimeCompare([]byte(c.Token), []byte(c.Approval.Token)) == 1 {
		return rule.Pass()
	}
	return rule.FailWith(TokenMismatch{})
}

func (c ResolutionCandidate) NotExpired() rule.Verdict {
	if !c.Now.After(c.Approval.ExpiresAt) {
		return rule.Pass()
	}
	return rule.FailWith(ApprovalExpired{ExpiresAt: c.Approval.ExpiresAt})
}

// OwnerMatches treats empty username on either side as a wildcard.
func (c ResolutionCandidate) OwnerMatches() rule.Verdict {
	if c.Username == "" || c.Approval.Username == "" || c.Username == c.Approval.Username {
		return rule.Pass()
	}
	return rule.FailWith(OwnerMismatch{Expected: c.Approval.Username, Actual: c.Username})
}

func (c ResolutionCandidate) IsPending() rule.Verdict {
	if c.Approval.Status == StatusPending {
		return rule.Pass()
	}
	return rule.FailWith(NotPending{Status: c.Approval.Status})
}

// CanResolve is the complete business rule for approval resolution.
var CanResolve = rule.AllOf(
	rule.New(ResolutionCandidate.TokenMatches),
	rule.New(ResolutionCandidate.NotExpired),
	rule.New(ResolutionCandidate.OwnerMatches),
	rule.New(ResolutionCandidate.IsPending),
)

package gate

import (
	"errors"
	"time"

	"github.com/benaskins/axon"
)

var (
	ErrNotFound        = axon.ErrNotFound
	ErrAlreadyResolved = errors.New("approval already resolved")
)

type ApprovalStatus string

const (
	StatusPending  ApprovalStatus = "pending"
	StatusApproved ApprovalStatus = "approved"
	StatusDenied   ApprovalStatus = "denied"
)

// Approval represents a deploy approval request with status tracking and expiry.
type Approval struct {
	ID         string         `json:"id"`
	Service    string         `json:"service"`
	Commit     string         `json:"commit"`
	Branch     string         `json:"branch"`
	Summary    string         `json:"summary"`
	Agent      string         `json:"agent"`
	Username   string         `json:"username"`
	Token      string         `json:"-"`
	Status     ApprovalStatus `json:"status"`
	CreatedAt  time.Time      `json:"created_at"`
	ExpiresAt  time.Time      `json:"expires_at"`
	ResolvedAt *time.Time     `json:"resolved_at"`
	ResolvedBy string         `json:"resolved_by,omitempty"`
}

// ApprovalRequest is the input for creating a new approval.
type ApprovalRequest struct {
	Service  string `json:"service"`
	Commit   string `json:"commit"`
	Branch   string `json:"branch"`
	Summary  string `json:"summary"`
	Agent    string `json:"agent"`
	Username string `json:"username"`
}

package gate

import "time"

type ApprovalStatus string

const (
	StatusPending  ApprovalStatus = "pending"
	StatusApproved ApprovalStatus = "approved"
	StatusDenied   ApprovalStatus = "denied"
)

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
	ResolvedAt *time.Time     `json:"resolved_at"`
	ResolvedBy string         `json:"resolved_by,omitempty"`
}

type ApprovalRequest struct {
	Service  string `json:"service"`
	Commit   string `json:"commit"`
	Branch   string `json:"branch"`
	Summary  string `json:"summary"`
	Agent    string `json:"agent"`
	Username string `json:"username"`
}

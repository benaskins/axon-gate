package gate

import "context"

// ApprovalStore defines the persistence interface for approvals.
type ApprovalStore interface {
	Create(ctx context.Context, req ApprovalRequest) (*Approval, error)
	Get(ctx context.Context, id string) (*Approval, error)
	Resolve(ctx context.Context, id string, status ApprovalStatus, resolvedBy string) error
}

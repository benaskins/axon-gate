package gate

// ApprovalStore defines the persistence interface for approvals.
type ApprovalStore interface {
	Create(req ApprovalRequest) (*Approval, error)
	Get(id string) *Approval
	Resolve(id string, status ApprovalStatus, resolvedBy string) bool
}

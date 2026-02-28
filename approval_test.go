package gate_test

import (
	"testing"

	gate "github.com/benaskins/axon-gate"
	"github.com/benaskins/axon-gate/gatetest"
)

func TestApprovalStore_CreateAndGet(t *testing.T) {
	store := gatetest.NewMemoryApprovalStore()

	approval, err := store.Create(gate.ApprovalRequest{
		Service:  "chat",
		Commit:   "abc1234",
		Branch:   "task/fix-login-123",
		Summary:  "Fixed login bug",
		Agent:    "aurora",
		Username: "benaskins",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if approval.ID == "" {
		t.Error("expected non-empty ID")
	}
	if approval.Token == "" {
		t.Error("expected non-empty token")
	}
	if approval.Status != gate.StatusPending {
		t.Errorf("expected pending, got %s", approval.Status)
	}
	if approval.Service != "chat" {
		t.Errorf("expected chat, got %s", approval.Service)
	}

	got := store.Get(approval.ID)
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.ID != approval.ID {
		t.Errorf("expected %s, got %s", approval.ID, got.ID)
	}
}

func TestApprovalStore_GetNotFound(t *testing.T) {
	store := gatetest.NewMemoryApprovalStore()
	if got := store.Get("nonexistent"); got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestApprovalStore_Resolve(t *testing.T) {
	store := gatetest.NewMemoryApprovalStore()

	approval, _ := store.Create(gate.ApprovalRequest{
		Service: "chat",
		Commit:  "abc1234",
	})

	ok := store.Resolve(approval.ID, gate.StatusApproved, "benaskins")
	if !ok {
		t.Fatal("Resolve returned false")
	}

	got := store.Get(approval.ID)
	if got.Status != gate.StatusApproved {
		t.Errorf("expected approved, got %s", got.Status)
	}
	if got.ResolvedBy != "benaskins" {
		t.Errorf("expected benaskins, got %s", got.ResolvedBy)
	}
	if got.ResolvedAt == nil {
		t.Error("expected non-nil ResolvedAt")
	}
}

func TestApprovalStore_ResolveAlreadyResolved(t *testing.T) {
	store := gatetest.NewMemoryApprovalStore()

	approval, _ := store.Create(gate.ApprovalRequest{
		Service: "chat",
		Commit:  "abc1234",
	})

	store.Resolve(approval.ID, gate.StatusApproved, "benaskins")

	// Second resolve should fail
	ok := store.Resolve(approval.ID, gate.StatusDenied, "other")
	if ok {
		t.Error("expected false for already-resolved approval")
	}

	// Status should still be approved
	got := store.Get(approval.ID)
	if got.Status != gate.StatusApproved {
		t.Errorf("expected approved, got %s", got.Status)
	}
}

func TestApprovalStore_ResolveNotFound(t *testing.T) {
	store := gatetest.NewMemoryApprovalStore()
	ok := store.Resolve("nonexistent", gate.StatusApproved, "benaskins")
	if ok {
		t.Error("expected false for nonexistent approval")
	}
}

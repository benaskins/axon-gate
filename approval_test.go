package gate_test

import (
	"context"
	"errors"
	"testing"

	gate "github.com/benaskins/axon-gate"
	"github.com/benaskins/axon-gate/gatetest"
)

func TestApprovalStore_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	store := gatetest.NewMemoryApprovalStore()

	approval, err := store.Create(ctx, gate.ApprovalRequest{
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

	got, err := store.Get(ctx, approval.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ID != approval.ID {
		t.Errorf("expected %s, got %s", approval.ID, got.ID)
	}
}

func TestApprovalStore_GetNotFound(t *testing.T) {
	store := gatetest.NewMemoryApprovalStore()
	_, err := store.Get(context.Background(), "nonexistent")
	if !errors.Is(err, gate.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestApprovalStore_Resolve(t *testing.T) {
	ctx := context.Background()
	store := gatetest.NewMemoryApprovalStore()

	approval, _ := store.Create(ctx, gate.ApprovalRequest{
		Service: "chat",
		Commit:  "abc1234",
	})

	if err := store.Resolve(ctx, approval.ID, gate.StatusApproved, "benaskins"); err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	got, _ := store.Get(ctx, approval.ID)
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
	ctx := context.Background()
	store := gatetest.NewMemoryApprovalStore()

	approval, _ := store.Create(ctx, gate.ApprovalRequest{
		Service: "chat",
		Commit:  "abc1234",
	})

	store.Resolve(ctx, approval.ID, gate.StatusApproved, "benaskins")

	// Second resolve should fail
	err := store.Resolve(ctx, approval.ID, gate.StatusDenied, "other")
	if !errors.Is(err, gate.ErrAlreadyResolved) {
		t.Errorf("expected ErrAlreadyResolved, got %v", err)
	}

	// Status should still be approved
	got, _ := store.Get(ctx, approval.ID)
	if got.Status != gate.StatusApproved {
		t.Errorf("expected approved, got %s", got.Status)
	}
}

func TestApprovalStore_ResolveNotFound(t *testing.T) {
	store := gatetest.NewMemoryApprovalStore()
	err := store.Resolve(context.Background(), "nonexistent", gate.StatusApproved, "benaskins")
	if !errors.Is(err, gate.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

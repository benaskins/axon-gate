package gatetest

import (
	"context"
	"fmt"
	"sync"
	"time"

	gate "github.com/benaskins/axon-gate"
)

type MemoryApprovalStore struct {
	mu        sync.RWMutex
	approvals map[string]*gate.Approval
}

func NewMemoryApprovalStore() *MemoryApprovalStore {
	return &MemoryApprovalStore{
		approvals: make(map[string]*gate.Approval),
	}
}

func (s *MemoryApprovalStore) Create(_ context.Context, req gate.ApprovalRequest) (*gate.Approval, error) {
	token, err := gate.GenerateToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	id, err := gate.GenerateID()
	if err != nil {
		return nil, fmt.Errorf("generate id: %w", err)
	}

	now := time.Now()
	approval := &gate.Approval{
		ID:        id,
		Service:   req.Service,
		Commit:    req.Commit,
		Branch:    req.Branch,
		Summary:   req.Summary,
		Agent:     req.Agent,
		Username:  req.Username,
		Token:     token,
		Status:    gate.StatusPending,
		CreatedAt: now,
		ExpiresAt: now.Add(1 * time.Hour),
	}

	s.mu.Lock()
	s.approvals[id] = approval
	s.mu.Unlock()

	return approval, nil
}

func (s *MemoryApprovalStore) Get(_ context.Context, id string) (*gate.Approval, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.approvals[id]
	if !ok {
		return nil, gate.ErrNotFound
	}
	// Return a copy to prevent data races from concurrent access
	copy := *a
	return &copy, nil
}

func (s *MemoryApprovalStore) Resolve(_ context.Context, id string, status gate.ApprovalStatus, resolvedBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, ok := s.approvals[id]
	if !ok {
		return gate.ErrNotFound
	}
	if a.Status != gate.StatusPending {
		return gate.ErrAlreadyResolved
	}

	now := time.Now()
	a.Status = status
	a.ResolvedAt = &now
	a.ResolvedBy = resolvedBy
	return nil
}

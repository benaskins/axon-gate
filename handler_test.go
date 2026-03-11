package gate_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/benaskins/axon"
	gate "github.com/benaskins/axon-gate"
	"github.com/benaskins/axon-gate/gatetest"
)

func newTestHandler(t *testing.T) (*gate.Handler, *gatetest.MemoryApprovalStore) {
	t.Helper()

	// Mock signal API
	signalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	t.Cleanup(signalServer.Close)

	// Mock auth service
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil || cookie.Value != "valid-session" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{
			"user_id":  "user_123",
			"username": "benaskins",
		})
	}))
	t.Cleanup(authServer.Close)

	store := gatetest.NewMemoryApprovalStore()
	signal := gate.NewSignalClient(signalServer.URL, "+1234567890")
	authClient := axon.NewAuthClientPlain(authServer.URL)
	t.Cleanup(authClient.Close)

	h := gate.NewHandler(store, signal, authClient, "http://deploy-gate.example.com", "https://auth.example.com/login")
	return h, store
}

func TestCreateApproval(t *testing.T) {
	h, _ := newTestHandler(t)

	body := `{"service":"chat","commit":"abc1234","branch":"task/test","summary":"test deploy","agent":"aurora","username":"benaskins"}`
	req := httptest.NewRequest("POST", "/api/approvals", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.CreateApproval(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["id"] == "" {
		t.Error("expected non-empty id")
	}
	if resp["status"] != "pending" {
		t.Errorf("expected pending, got %s", resp["status"])
	}
}

func TestCreateApproval_MissingFields(t *testing.T) {
	h, _ := newTestHandler(t)

	body := `{"summary":"no service or commit"}`
	req := httptest.NewRequest("POST", "/api/approvals", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.CreateApproval(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetApproval(t *testing.T) {
	h, store := newTestHandler(t)
	ctx := context.Background()

	// Create an approval first
	approval, _ := store.Create(ctx, gate.ApprovalRequest{
		Service: "chat",
		Commit:  "abc1234",
	})

	req := httptest.NewRequest("GET", "/api/approvals/"+approval.ID, nil)
	req.SetPathValue("id", approval.ID)
	w := httptest.NewRecorder()

	h.GetApproval(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp gate.Approval
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.ID != approval.ID {
		t.Errorf("expected %s, got %s", approval.ID, resp.ID)
	}
}

func TestGetApproval_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest("GET", "/api/approvals/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.GetApproval(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestSendNotification(t *testing.T) {
	h, _ := newTestHandler(t)

	body := `{"message":"Deployed chat (abc1234) to production"}`
	req := httptest.NewRequest("POST", "/api/notifications", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.SendNotification(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestShowApprovalPage_InvalidToken(t *testing.T) {
	h, store := newTestHandler(t)
	ctx := context.Background()

	approval, _ := store.Create(ctx, gate.ApprovalRequest{
		Service:  "chat",
		Commit:   "abc1234",
		Username: "benaskins",
	})

	req := httptest.NewRequest("GET", "/approve/"+approval.ID+"?token=wrong", nil)
	req.SetPathValue("id", approval.ID)
	req.AddCookie(&http.Cookie{Name: "session", Value: "valid-session"})
	w := httptest.NewRecorder()

	h.ShowApprovalPage(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestShowApprovalPage_NoSession_RedirectsToLogin(t *testing.T) {
	h, store := newTestHandler(t)
	ctx := context.Background()

	approval, _ := store.Create(ctx, gate.ApprovalRequest{
		Service:  "chat",
		Commit:   "abc1234",
		Username: "benaskins",
	})

	req := httptest.NewRequest("GET", "/approve/"+approval.ID+"?token="+approval.Token, nil)
	req.SetPathValue("id", approval.ID)
	w := httptest.NewRecorder()

	h.ShowApprovalPage(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if location == "" {
		t.Fatal("expected Location header")
	}
	if !strings.Contains(location, "auth.example.com/login") {
		t.Errorf("expected redirect to auth login, got %s", location)
	}
	if !strings.Contains(location, "redirect=") {
		t.Errorf("expected redirect param in URL, got %s", location)
	}
}

func TestProcessApproval_Approve(t *testing.T) {
	h, store := newTestHandler(t)
	ctx := context.Background()

	approval, _ := store.Create(ctx, gate.ApprovalRequest{
		Service:  "chat",
		Commit:   "abc1234",
		Username: "benaskins",
	})

	body := "token=" + approval.Token + "&action=approve"
	req := httptest.NewRequest("POST", "/approve/"+approval.ID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", approval.ID)
	req.AddCookie(&http.Cookie{Name: "session", Value: "valid-session"})
	w := httptest.NewRecorder()

	h.ProcessApproval(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	got, _ := store.Get(ctx, approval.ID)
	if got.Status != gate.StatusApproved {
		t.Errorf("expected approved, got %s", got.Status)
	}
	if got.ResolvedBy != "benaskins" {
		t.Errorf("expected benaskins, got %s", got.ResolvedBy)
	}
}

func TestProcessApproval_Deny(t *testing.T) {
	h, store := newTestHandler(t)
	ctx := context.Background()

	approval, _ := store.Create(ctx, gate.ApprovalRequest{
		Service:  "chat",
		Commit:   "abc1234",
		Username: "benaskins",
	})

	body := "token=" + approval.Token + "&action=deny"
	req := httptest.NewRequest("POST", "/approve/"+approval.ID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", approval.ID)
	req.AddCookie(&http.Cookie{Name: "session", Value: "valid-session"})
	w := httptest.NewRecorder()

	h.ProcessApproval(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	got, _ := store.Get(ctx, approval.ID)
	if got.Status != gate.StatusDenied {
		t.Errorf("expected denied, got %s", got.Status)
	}
}

func TestProcessApproval_WrongUser(t *testing.T) {
	h, store := newTestHandler(t)
	ctx := context.Background()

	// Approval belongs to a different user
	approval, _ := store.Create(ctx, gate.ApprovalRequest{
		Service:  "chat",
		Commit:   "abc1234",
		Username: "other_user",
	})

	body := "token=" + approval.Token + "&action=approve"
	req := httptest.NewRequest("POST", "/approve/"+approval.ID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", approval.ID)
	req.AddCookie(&http.Cookie{Name: "session", Value: "valid-session"})
	w := httptest.NewRecorder()

	h.ProcessApproval(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}

	// Should still be pending
	got, _ := store.Get(ctx, approval.ID)
	if got.Status != gate.StatusPending {
		t.Errorf("expected pending, got %s", got.Status)
	}
}

func TestShowApprovalPage_Expired(t *testing.T) {
	h, store := newTestHandler(t)
	ctx := context.Background()

	approval, _ := store.Create(ctx, gate.ApprovalRequest{
		Service:  "chat",
		Commit:   "abc1234",
		Username: "benaskins",
	})

	// Set expiry to the past
	approval.ExpiresAt = time.Now().Add(-1 * time.Hour)

	req := httptest.NewRequest("GET", "/approve/"+approval.ID+"?token="+approval.Token, nil)
	req.SetPathValue("id", approval.ID)
	req.AddCookie(&http.Cookie{Name: "session", Value: "valid-session"})
	w := httptest.NewRecorder()

	h.ShowApprovalPage(w, req)

	if w.Code != http.StatusGone {
		t.Errorf("expected 410, got %d", w.Code)
	}
}

func TestProcessApproval_Expired(t *testing.T) {
	h, store := newTestHandler(t)
	ctx := context.Background()

	approval, _ := store.Create(ctx, gate.ApprovalRequest{
		Service:  "chat",
		Commit:   "abc1234",
		Username: "benaskins",
	})

	// Set expiry to the past
	approval.ExpiresAt = time.Now().Add(-1 * time.Hour)

	body := "token=" + approval.Token + "&action=approve"
	req := httptest.NewRequest("POST", "/approve/"+approval.ID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", approval.ID)
	req.AddCookie(&http.Cookie{Name: "session", Value: "valid-session"})
	w := httptest.NewRecorder()

	h.ProcessApproval(w, req)

	if w.Code != http.StatusGone {
		t.Errorf("expected 410, got %d", w.Code)
	}

	// Should still be pending
	got, _ := store.Get(ctx, approval.ID)
	if got.Status != gate.StatusPending {
		t.Errorf("expected pending, got %s", got.Status)
	}
}

func TestProcessApproval_AlreadyResolved(t *testing.T) {
	h, store := newTestHandler(t)
	ctx := context.Background()

	approval, _ := store.Create(ctx, gate.ApprovalRequest{
		Service:  "chat",
		Commit:   "abc1234",
		Username: "benaskins",
	})

	// Resolve the approval first
	store.Resolve(ctx, approval.ID, gate.StatusApproved, "benaskins")

	body := "token=" + approval.Token + "&action=deny"
	req := httptest.NewRequest("POST", "/approve/"+approval.ID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", approval.ID)
	req.AddCookie(&http.Cookie{Name: "session", Value: "valid-session"})
	w := httptest.NewRecorder()

	h.ProcessApproval(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (resolved page), got %d: %s", w.Code, w.Body.String())
	}

	// Should still be approved (not changed to denied)
	got, _ := store.Get(ctx, approval.ID)
	if got.Status != gate.StatusApproved {
		t.Errorf("expected approved, got %s", got.Status)
	}
}

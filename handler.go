package gate

import (
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/benaskins/axon"
	rule "github.com/benaskins/axon-rule"
)

//go:embed all:templates
var templateFS embed.FS

var templates = template.Must(template.ParseFS(templateFS, "templates/*.html"))

// Handler serves the approval API endpoints and embedded web UI.
type Handler struct {
	store      ApprovalStore
	signal     *SignalClient
	authClient *axon.AuthClient
	baseURL    string
	loginURL   string
}

func NewHandler(store ApprovalStore, signal *SignalClient, authClient *axon.AuthClient, baseURL, loginURL string) *Handler {
	return &Handler{
		store:      store,
		signal:     signal,
		authClient: authClient,
		baseURL:    baseURL,
		loginURL:   loginURL,
	}
}

// CreateApproval handles POST /api/approvals — create approval, send Signal message.
func (h *Handler) CreateApproval(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
	var req ApprovalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		axon.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Service == "" || req.Commit == "" {
		axon.WriteError(w, http.StatusBadRequest, "service and commit are required")
		return
	}

	approval, err := h.store.Create(r.Context(), req)
	if err != nil {
		slog.Error("failed to create approval", "error", err)
		axon.WriteError(w, http.StatusInternalServerError, "failed to create approval")
		return
	}

	// Send Signal message with approval link
	approvalURL := h.baseURL + "/approve/" + approval.ID + "?token=" + approval.Token
	message := "Deploy approval requested\n\n" +
		"Service: " + approval.Service + "\n" +
		"Commit: " + approval.Commit + "\n" +
		"Branch: " + approval.Branch + "\n" +
		"Agent: " + approval.Agent + "\n" +
		"Summary: " + approval.Summary + "\n\n" +
		approvalURL

	if err := h.signal.Send(message); err != nil {
		slog.Error("failed to send signal message", "error", err, "approval_id", approval.ID)
		// Don't fail the request — the approval is created, caller can poll
	}

	axon.WriteJSON(w, http.StatusAccepted, map[string]string{
		"id":     approval.ID,
		"status": string(approval.Status),
	})
}

// GetApproval handles GET /api/approvals/{id} — poll approval status.
// Returns only the status to avoid leaking metadata to unauthenticated callers.
func (h *Handler) GetApproval(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	approval, err := h.store.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			axon.WriteError(w, http.StatusNotFound, "approval not found")
			return
		}
		slog.Error("failed to get approval", "error", err, "id", id)
		axon.WriteError(w, http.StatusInternalServerError, "failed to get approval")
		return
	}

	axon.WriteJSON(w, http.StatusOK, map[string]string{
		"id":     approval.ID,
		"status": string(approval.Status),
	})
}

// SendNotification handles POST /api/notifications — send informational Signal message.
func (h *Handler) SendNotification(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		axon.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Message == "" {
		axon.WriteError(w, http.StatusBadRequest, "message is required")
		return
	}

	if err := h.signal.Send(req.Message); err != nil {
		slog.Error("failed to send notification", "error", err)
		axon.WriteError(w, http.StatusInternalServerError, "failed to send notification")
		return
	}

	axon.WriteJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// ShowApprovalPage handles GET /approve/{id} — serve approval page.
func (h *Handler) ShowApprovalPage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token := r.URL.Query().Get("token")

	approval, _, ok := h.validateApproval(w, r, id, token)
	if !ok {
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "approve.html", map[string]any{
		"Approval": approval,
		"Token":    token,
	}); err != nil {
		slog.Error("failed to render approval page", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
}

// ProcessApproval handles POST /approve/{id} — process approval decision.
func (h *Handler) ProcessApproval(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := r.ParseForm(); err != nil {
		h.renderError(w, http.StatusBadRequest, "Invalid form data")
		return
	}

	token := r.FormValue("token")
	action := r.FormValue("action")

	approval, username, ok := h.validateApproval(w, r, id, token)
	if !ok {
		return
	}

	var status ApprovalStatus
	var signalMsg string

	switch action {
	case "approve":
		status = StatusApproved
		signalMsg = "Approved \u2014 deploying " + approval.Service + " (" + approval.Commit + ")..."
	case "deny":
		status = StatusDenied
		signalMsg = "Denied \u2014 deploy of " + approval.Service + " (" + approval.Commit + ") cancelled."
	default:
		h.renderError(w, http.StatusBadRequest, "Invalid action")
		return
	}

	if err := h.store.Resolve(r.Context(), id, status, username); err != nil {
		if errors.Is(err, ErrAlreadyResolved) {
			h.renderError(w, http.StatusConflict, "Approval already resolved")
			return
		}
		slog.Error("failed to resolve approval", "error", err, "id", id)
		h.renderError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	if err := h.signal.Send(signalMsg); err != nil {
		slog.Error("failed to send approval confirmation", "error", err, "approval_id", id)
	}

	// Re-read to get updated state
	approval, _ = h.store.Get(r.Context(), id)
	h.renderResolved(w, approval)
}

// validateApproval fetches the approval, checks token, expiry, session,
// owner, and pending status. Returns the approval, session username, and
// whether validation passed. Writes the HTTP response on failure.
//
// Token and expiry are checked before session validation so users with
// bad links aren't forced to log in first.
func (h *Handler) validateApproval(w http.ResponseWriter, r *http.Request, id, token string) (*Approval, string, bool) {
	approval, err := h.store.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			h.renderError(w, http.StatusNotFound, "Approval not found")
			return nil, "", false
		}
		slog.Error("failed to get approval", "error", err, "id", id)
		h.renderError(w, http.StatusInternalServerError, "Internal server error")
		return nil, "", false
	}

	candidate := ResolutionCandidate{
		Approval: *approval,
		Token:    token,
		Now:      time.Now(),
	}

	if !rule.New(ResolutionCandidate.TokenMatches).Check(candidate).OK {
		h.renderError(w, http.StatusForbidden, "Invalid approval token")
		return nil, "", false
	}
	if !rule.New(ResolutionCandidate.NotExpired).Check(candidate).OK {
		h.renderError(w, http.StatusGone, "Approval expired")
		return nil, "", false
	}

	session, err := h.validateSession(r)
	if err != nil {
		h.redirectToLogin(w, r)
		return nil, "", false
	}

	username := session.Username()
	candidate.Username = username
	if !rule.New(ResolutionCandidate.OwnerMatches).Check(candidate).OK {
		h.renderError(w, http.StatusForbidden, "Only "+approval.Username+" can approve this deploy")
		return nil, "", false
	}

	if !rule.New(ResolutionCandidate.IsPending).Check(candidate).OK {
		h.renderResolved(w, approval)
		return nil, "", false
	}

	return approval, username, true
}

func (h *Handler) validateSession(r *http.Request) (*axon.SessionInfo, error) {
	cookie, err := r.Cookie("session")
	if err != nil {
		return nil, axon.ErrUnauthorized
	}
	return h.authClient.ValidateSession(cookie.Value)
}

func (h *Handler) redirectToLogin(w http.ResponseWriter, r *http.Request) {
	currentURL := h.baseURL + r.URL.RequestURI()
	http.Redirect(w, r, h.loginURL+"?redirect="+url.QueryEscape(currentURL), http.StatusFound)
}

func (h *Handler) renderError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := templates.ExecuteTemplate(w, "error.html", map[string]string{
		"Message": message,
	}); err != nil {
		slog.Error("failed to render error page", "error", err)
		// Status header already sent; write plain text fallback
		_, _ = w.Write([]byte(message))
	}
}

func (h *Handler) renderResolved(w http.ResponseWriter, approval *Approval) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "resolved.html", map[string]any{
		"Approval": approval,
	}); err != nil {
		slog.Error("failed to render resolved page", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

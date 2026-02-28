package gate

import (
	"embed"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/benaskins/axon"
)

//go:embed all:templates
var templateFS embed.FS

var templates = template.Must(template.ParseFS(templateFS, "templates/*.html"))

type Handler struct {
	store      ApprovalStore
	signal     *SignalClient
	authClient *axon.AuthClient
	baseURL    string
}

func NewHandler(store ApprovalStore, signal *SignalClient, authClient *axon.AuthClient, baseURL string) *Handler {
	return &Handler{
		store:      store,
		signal:     signal,
		authClient: authClient,
		baseURL:    baseURL,
	}
}

// CreateApproval handles POST /api/approvals — create approval, send Signal message.
func (h *Handler) CreateApproval(w http.ResponseWriter, r *http.Request) {
	var req ApprovalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		axon.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Service == "" || req.Commit == "" {
		axon.WriteError(w, http.StatusBadRequest, "service and commit are required")
		return
	}

	approval, err := h.store.Create(req)
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
func (h *Handler) GetApproval(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	approval := h.store.Get(id)
	if approval == nil {
		axon.WriteError(w, http.StatusNotFound, "approval not found")
		return
	}

	axon.WriteJSON(w, http.StatusOK, approval)
}

// SendNotification handles POST /api/notifications — send informational Signal message.
func (h *Handler) SendNotification(w http.ResponseWriter, r *http.Request) {
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

	approval := h.store.Get(id)
	if approval == nil {
		h.renderError(w, http.StatusNotFound, "Approval not found")
		return
	}

	if token != approval.Token {
		h.renderError(w, http.StatusForbidden, "Invalid approval token")
		return
	}

	// Validate session — redirect to login if unauthenticated
	session, err := h.validateSession(r)
	if err != nil {
		h.redirectToLogin(w, r)
		return
	}

	// Owner check — match on username if available, otherwise any authenticated user can approve
	username := session.Username()
	if username != "" && approval.Username != "" && username != approval.Username {
		h.renderError(w, http.StatusForbidden, "Only "+approval.Username+" can approve this deploy")
		return
	}

	if approval.Status != StatusPending {
		h.renderResolved(w, approval)
		return
	}

	templates.ExecuteTemplate(w, "approve.html", map[string]any{
		"Approval": approval,
		"Token":    token,
	})
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

	approval := h.store.Get(id)
	if approval == nil {
		h.renderError(w, http.StatusNotFound, "Approval not found")
		return
	}

	if token != approval.Token {
		h.renderError(w, http.StatusForbidden, "Invalid approval token")
		return
	}

	session, err := h.validateSession(r)
	if err != nil {
		h.redirectToLogin(w, r)
		return
	}

	username := session.Username()
	if username != "" && approval.Username != "" && username != approval.Username {
		h.renderError(w, http.StatusForbidden, "Only "+approval.Username+" can approve this deploy")
		return
	}

	if approval.Status != StatusPending {
		h.renderResolved(w, approval)
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

	if !h.store.Resolve(id, status, username) {
		h.renderError(w, http.StatusConflict, "Approval already resolved")
		return
	}

	if err := h.signal.Send(signalMsg); err != nil {
		slog.Error("failed to send approval confirmation", "error", err, "approval_id", id)
	}

	// Re-read to get updated state
	approval = h.store.Get(id)
	h.renderResolved(w, approval)
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
	loginURL := "https://auth.studio.internal/login?redirect=" + url.QueryEscape(currentURL)
	http.Redirect(w, r, loginURL, http.StatusFound)
}

func (h *Handler) renderError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	templates.ExecuteTemplate(w, "error.html", map[string]string{
		"Message": message,
	})
}

func (h *Handler) renderResolved(w http.ResponseWriter, approval *Approval) {
	templates.ExecuteTemplate(w, "resolved.html", map[string]any{
		"Approval": approval,
	})
}

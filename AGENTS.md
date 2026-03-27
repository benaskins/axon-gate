---
module: github.com/benaskins/axon-gate
kind: service
---

# axon-gate

Deploy approval gate with Signal notifications and a review UI.

## Architecture

Agents or CI pipelines create an approval via `POST /api/approvals`. The handler generates a time-limited token, persists the approval, and sends a Signal notification with a link to the review page. The operator opens the embedded web UI, reviews the deployment summary, and approves or denies. The caller polls `GET /api/approvals/{id}` until a decision is made.

### Handler routes

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/approvals` | Create approval and send Signal notification |
| GET | `/api/approvals/{id}` | Poll approval status |
| POST | `/api/notifications` | Send informational Signal message |
| GET | `/approve/{id}` | Serve approval review page (embedded HTML) |
| POST | `/approve/{id}` | Process approve/deny decision |

### Embedded templates

HTML templates in `templates/` are embedded via `//go:embed` and rendered server-side:

- `templates/approve.html` -- approval review page
- `templates/resolved.html` -- post-decision confirmation
- `templates/error.html` -- error display

## Build & Test

```bash
go test ./...
go vet ./...
```

## Key Files

- `types.go` -- Approval, ApprovalRequest, ApprovalStatus types
- `store.go` -- ApprovalStore interface (Create, Get, Resolve)
- `handler.go` -- HTTP handlers for approval workflow and embedded templates
- `signal.go` -- SignalClient for sending notifications via Signal REST API
- `token.go` -- secure token and ID generation
- `doc.go` -- package documentation
- `gatetest/store.go` -- MemoryApprovalStore for testing
- `example/main.go` -- minimal composition root example

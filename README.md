# axon-gate

> Domain package · Part of the [lamina](https://github.com/benaskins/lamina-mono) workspace

Deploy approval gate with Signal notifications. Creates time-limited approval requests, notifies operators via Signal, and serves a web UI for reviewing and approving deployments. Agents or CI pipelines request approval via the API, then poll for a decision while a human reviews and responds through the approval page.

## Getting started

```
go get github.com/benaskins/axon-gate@latest
```

Requires Go 1.24+.

axon-gate is a domain package — it provides HTTP handlers and domain types that you wire into your own composition root. See [`example/main.go`](example/main.go) for a minimal setup.

```go
store := gatetest.NewMemoryApprovalStore()
signal := gate.NewSignalClient(signalAPIURL, recipientNumber)
handler := gate.NewHandler(store, signal, authClient, baseURL, loginURL)

mux.HandleFunc("POST /api/approvals", handler.CreateApproval)
mux.HandleFunc("GET /api/approvals/{id}", handler.GetApproval)
mux.HandleFunc("POST /api/notifications", handler.SendNotification)
mux.HandleFunc("GET /approve/{id}", handler.ShowApprovalPage)
mux.HandleFunc("POST /approve/{id}", handler.ProcessApproval)
```

## Key types

- `Approval`, `ApprovalRequest` — approval domain types with status tracking and expiry
- `ApprovalStore` — persistence interface (Create, Get, Resolve)
- `SignalClient` — sends notifications via Signal REST API
- `Handler` — HTTP handler with API endpoints and an embedded web UI
- `gatetest.MemoryApprovalStore` — in-memory store for testing

## License

MIT

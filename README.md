# axon-gate

A deploy approval gate service. Creates time-limited approval requests, notifies operators via Signal, and serves a web UI for reviewing and approving deployments. Part of [lamina](https://github.com/benaskins/lamina) — each axon package can be used independently.

## Install

```
go get github.com/benaskins/axon-gate@latest
```

Requires Go 1.24+.

## Usage

```go
store := gatetest.NewMemoryApprovalStore()
signal := gate.NewSignalClient(signalAPIURL, recipientNumber)
handler := gate.NewHandler(store, signal, authClient, baseURL, loginURL)

mux.HandleFunc("POST /api/approvals", handler.CreateApproval)
mux.HandleFunc("GET /api/approvals/{id}", handler.GetApproval)
mux.HandleFunc("GET /approve/{id}", handler.ShowApprovalPage)
mux.HandleFunc("POST /approve/{id}", handler.ProcessApproval)
```

### Key types

- `Approval`, `ApprovalRequest` — approval domain types
- `ApprovalStore` — persistence interface
- `SignalClient` — Signal messaging client
- `Handler` — HTTP handler with API and web UI endpoints

### Sub-packages

- `gatetest` — in-memory mock store for testing

## License

MIT — see [LICENSE](LICENSE).

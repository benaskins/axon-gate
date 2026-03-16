# axon-gate

Deploy approval gate with Signal notifications and a review UI.

## Build & Test

```bash
go test ./...
go vet ./...
```

## Key Files

- `handler.go` — HTTP handlers for approval workflow
- `signal.go` — Signal messenger integration for notifications
- `approval_test.go` — approval flow tests

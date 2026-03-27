@AGENTS.md

## Conventions
- Handler pattern follows axon conventions
- HTML templates in `templates/` are embedded via `//go:embed` and rendered server-side
- Use `gatetest/store.go` (MemoryApprovalStore) for all tests
- Signal notifications sent via REST API client (`signal.go`)
- Approval flow: create -> notify -> review UI -> approve/deny -> poll result

## Constraints
- Depends on axon only — no other axon-* imports
- Embedded HTML templates, not a SPA — keep server-side rendering
- Do not add external notification providers beyond Signal without approval
- Token generation in `token.go` is security-sensitive — do not simplify

## Testing
- `go test ./...` — all tests use in-memory store, no external services needed
- `go vet ./...` — lint
- See `example/main.go` for minimal composition root reference

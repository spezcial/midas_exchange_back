# Wave 1 — Stage E (Backend): CRIT-1 Webhook Hardening

Branch: `security/e-webhook-hardening`
Plan reference: `SECURITY_HARDENING_PLAN.md` §3a — "CRIT-1 — Timing-attack и fail-open в верификации webhook-секрета"

## Files changed

| Path | Change |
|---|---|
| `internal/api/webhook/cryptogate_handler.go` | Added `crypto/subtle` import. Rewrote `verifySecret` to be fail-closed on empty secret and to use `subtle.ConstantTimeCompare` for the header check. |
| `cmd/server/main.go` | Added a fail-fast guard inside the crypto-gate wiring block: when `cfg.Server.Env == "production"` and `cfg.CryptoGate.WebhookSecret == ""`, the process logs an error and `os.Exit(1)` *before* `server.ListenAndServe`. |
| `internal/api/webhook/cryptogate_handler_test.go` | New file. Table-driven tests for `verifySecret` covering: empty secret (fail-closed), correct match, equal-length mismatch, shorter-vs-longer and longer-vs-shorter inputs (no panic), missing header, empty token vs configured secret. Plus a dedicated `defer recover()` test confirming no panic on different-length inputs. |

No other files were touched.

## Where the fail-fast lives

`cmd/server/main.go`, inside the `if cfg.CryptoGate.BaseURL != "" && cfg.CryptoGate.Token != ""` block (the only place where the `/cg/deposit` and `/cg/withdraw` routes get mounted via `routes.go`). The check runs:

1. After `config.Load()` and its built-in `validate()` (which already enforces the same condition when crypto-gate is enabled — kept as belt-and-suspenders).
2. Before `cgService` is constructed.
3. Therefore well before `server.ListenAndServe` and before the webhook routes are mounted in `setupRouter`.

The check is intentionally kept inside the crypto-gate wiring branch so a non-production deployment that simply does not configure crypto-gate (no `CRYPTO_GATE_URL` / `CRYPTO_GATE_TOKEN`) is not forced to also set a webhook secret — only deployments that actually expose the webhook endpoint must supply the secret.

## Test results

All commands run from `/Users/gtrondin/Development/midas-exchange/midas_exchange_back`:

- `go vet ./...` — clean (no output, exit 0).
- `go build ./...` — clean (no output, exit 0).
- `go test ./internal/api/webhook/... -race -v` — PASS.
  - `TestVerifySecret` (8 sub-tests) — PASS.
  - `TestVerifySecret_DifferentLengthsNoPanic` — PASS.
- `go test ./...` — PASS (`internal/api/webhook` and `internal/service` packages had tests; both green; remaining packages have no test files).

## Acceptance criteria status

- [x] Backend startup with `CRYPTO_GATE_WEBHOOK_SECRET=""` in production-mode → fatal exit. Triggered both by `config.validate()` (when `CRYPTO_GATE_URL` is set) and by the explicit `os.Exit(1)` in `main.go`.
- [x] Request to `/cg/deposit` without a correct `X-TOKEN` → 401. The handler still returns `http.Error(w, "unauthorized", http.StatusUnauthorized)` whenever `verifySecret` returns false; with the new logic that includes the previous fail-open dev path (empty configured secret).
- [x] Constant-time check confirmed by tests on two different-length inputs (`"shorter"` vs `"longvalue"`, both directions) without panic.
- [x] `go vet ./...`, `go build ./...`, `go test ./...` all green.

## Blockers

None.

## Notes / follow-ups outside scope of CRIT-1

- The existing `pkg/config/config.go` `validate()` already enforces non-empty `CRYPTO_GATE_WEBHOOK_SECRET` and `CRYPTO_GATE_PLATFORM` in production when `CRYPTO_GATE_URL` is set. This is left as-is — the new guard in `main.go` is additive, not a replacement.
- Behavior change for local dev: previously a developer could leave `CRYPTO_GATE_WEBHOOK_SECRET` empty and webhooks would be accepted. Now they will be rejected with 401. Local dev that needs to exercise the webhook must set the secret (and pass `X-TOKEN: <secret>`). This intentional break is the whole point of CRIT-1.

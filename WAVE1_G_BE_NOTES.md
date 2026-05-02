# Wave 1, Stage G — Backend CI & Tooling — Notes

Branch: `security/g-ci-tooling`
Date: 2026-05-02
Scope: tooling-only. **No `.go` files were modified** in `internal/`, `cmd/`,
`pkg/`, `const/` (verified — only `Makefile` is changed in tracked files; the
rest are new tooling files).

---

## 1. Files created / modified

| Path | Status | Purpose |
|------|--------|---------|
| `.golangci.yml` | new | Strict golangci-lint config (20 linters enabled) |
| `tools.go` | new | Build-tag `tools` file pinning dev tools in `go.mod` |
| `.github/workflows/ci.yml` | new | GitHub Actions workflow for build/test/lint/security |
| `Makefile` | modified | Added `tools-install`, `lint-strict`, `vuln` targets |
| `WAVE1_G_BE_NOTES.md` | new | This document |

`Makefile.prod` was **not** touched, per instructions.

Baseline still green:
```
go vet ./...     → OK
go build ./...   → OK
```

---

## 2. Tool installation status (local)

All three tools were installed under `$(go env GOPATH)/bin` =
`/Users/gtrondin/go/bin`. They are **not on PATH** by default — add
`$(go env GOPATH)/bin` to PATH or invoke with absolute paths.

| Tool | Version installed | Notes |
|------|-------------------|-------|
| `golangci-lint` | **v1.64.8** (NOT v1.61.0) | v1.61.0 fails to build under Go 1.25 due to stale `golang.org/x/tools` (`invalid array length -delta * delta`). The Makefile and CI still pin **v1.61.0** as the user requested. **Recommendation:** bump pin to `v1.64.8` (last v1 release line; v2 has a different config schema and would require migrating `.golangci.yml`). |
| `staticcheck` | latest | OK |
| `govulncheck` | latest | OK |

`go mod tidy` was **not** run for `tools.go`. Adding the three big tool
modules to `go.mod` would pull hundreds of indirect deps and bloat
`go.sum`. The `//go:build tools` tag prevents compilation, but
dependencies still appear in module metadata. Developers should rely on
`make tools-install`, which uses `go install <pkg>@<ver>` — the modern
Go-1.25-recommended pattern that avoids polluting `go.mod`. The
`tools.go` file is kept as a marker / future-proofing in case the team
later prefers the version-pinned approach.

> ⚠️ **CI pin mismatch.** CI references `v1.61.0` (per spec) but locally
> only `v1.64.8` builds against Go 1.25. CI uses a pre-built binary via
> `golangci/golangci-lint-action@v6` so it does NOT recompile from source
> — pinned `v1.61.0` may still run there because the action downloads a
> pre-built binary. **However, that binary was compiled with an older Go
> and may behave inconsistently** against modules built with Go 1.25.
> Recommend bumping the CI pin to `v1.64.8` in a follow-up wave; left at
> `v1.61.0` for now to satisfy the spec.

---

## 3. Linter findings — known issues to fix in next waves

### 3.1 `golangci-lint run` — ~200 issues across 13 linters

Distribution (raw count of issues by linter on the current
branch's working tree):

| Linter | Count | Severity |
|--------|-------|----------|
| contextcheck | 78 | high — context not propagated through call chains |
| errorlint | 35 | high — `==`/`switch` on errors won't work with `%w`-wrapped errors |
| misspell | 20 | low — `cancelled` (UK) vs `canceled` (US); `authorise/marshalling` |
| errcheck | 18 | medium — unchecked `Encode`, `Close`, `Encoder.Encode` returns |
| gocritic | 15 | low — `httpNoBody`, `unnamedResult`, `exitAfterDefer` |
| revive | 8 | low — `BINANCE_API` ALL_CAPS const, blank `_ "github.com/lib/pq"` import without comment |
| gosec | 8 | high to review — G101 "potential hardcoded credentials" on `const/queries/passkey_repo.go` and `const/queries/user_repo.go`. **These are SQL strings, not credentials — false positives**, but should be silenced via `//nolint:gosec // SQL query, not a credential` rather than left noisy. |
| nilnil | 6 | medium — repos returning `nil, nil` on not-found should use a sentinel error like `ErrNotFound` |
| unused | 4 | low — dead `mockTxRepo` / `selectiveTxRepo` in `internal/service/otc_service_test.go` |
| unconvert | 3 | trivial — duplicated `domain.UserRole(input.Role)` casts |
| nilerr | 3 | medium — `internal/api/middleware/auth.go:109` returns `nil` after a non-nil error |
| prealloc | 2 | low — slice prealloc opportunity |
| gosimple | 1 | trivial — redundant `return` |

Notable hotspots that look like real bugs / security-relevant:
- `internal/api/middleware/auth.go:109` (`nilerr`): error from JWT
  parsing is dropped silently — request continues without blocklisting
  JTI.
- `internal/api/client/auth_handler.go`,
  `internal/api/client/twofa_handler.go`,
  `internal/repository/deposit_address_repository.go` —
  `switch err {}` / `err == sql.ErrNoRows` will break the moment any
  caller wraps with `fmt.Errorf("%w", ...)`.
- `cmd/server/main.go:49` (`gocritic.exitAfterDefer`): `os.Exit(1)`
  after `defer db.Close()` — DB connection leaks on error path.
- 78× `contextcheck` — services pass `context.Background()` instead of
  the inbound HTTP context, breaking deadline propagation.

**None of these are addressed in this wave.** They are tracked for the
next security/refactor waves.

### 3.2 `staticcheck` — 7 issues (subset overlapping golangci)

```
internal/api/admin/order_handler.go:69       S1023  redundant return
internal/service/oauth_service.go:118,203    ST1005 capitalized error strings
internal/service/otc_service_test.go:265-286 U1000  unused mockTxRepo / selectiveTxRepo
```

### 3.3 `govulncheck` — 7 stdlib vulnerabilities

All findings are in the **Go standard library** because the local
toolchain is `go1.25.6` and `go.mod` declares `go 1.25.0`. None are in
third-party deps used at runtime.

| ID | Package | Fixed in | Severity |
|----|---------|----------|----------|
| GO-2026-4947 | crypto/x509 | go1.25.9 | unexpected work in cert chain building |
| GO-2026-4946 | crypto/x509 | go1.25.9 | inefficient policy validation |
| GO-2026-4870 | crypto/tls  | go1.25.9 | TLS 1.3 KeyUpdate DoS |
| GO-2026-4865 | html/template | go1.25.9 | XSS via JsBraceDepth tracking bug |
| GO-2026-4603 | html/template | go1.25.8 | XSS variant |
| GO-2026-4601 | net/url     | go1.25.8 | URL parsing edge case |
| GO-2026-4337 | crypto/tls  | go1.25.7 | unexpected session resumption |

**Action for next wave:** bump CI / Dockerfile / dev toolchain to
**Go 1.25.9** (or newer 1.25.x) — this single change closes all 7
findings without any code edits. Update:
- `.github/workflows/ci.yml` (`GO_VERSION: "1.25"` → pin to `1.25.9`)
- `go.mod` `go 1.25.0` directive → `go 1.25.9`
- `Dockerfile` base image from `golang:1.25` → `golang:1.25.9`

CI's `govulncheck` step will start failing the build the moment GitHub
runners' default Go drifts behind a new advisory — that's the desired
behavior.

---

## 4. Recommendations for next waves

1. **Bump Go toolchain to 1.25.9+** (closes all stdlib CVEs from §3.3).
2. **Triage golangci findings**: address the high-severity ones first
   — `nilerr` in auth middleware, `errorlint` in handlers/repos,
   `gocritic.exitAfterDefer` in `cmd/server/main.go`, `nilnil` in
   repositories.
3. **Silence false-positive `gosec.G101`** on SQL constants in
   `const/queries/*.go` with localized
   `//nolint:gosec // SQL query constant, not a credential` annotations
   rather than blanket exclude.
4. **Rename `cancelled` → `canceled`** is a coordinated change (DB
   columns, JSON contract, status enum). Track separately; don't fold
   into a tooling wave. Until then, the misspell findings are accepted
   tech debt — could be silenced via per-file `//nolint:misspell` in
   `internal/domain/otc.go` and `const/queries/otc_repo.go` if the
   team prefers a clean signal-to-noise ratio.
5. **Decide on golangci-lint major version**: stay on v1.x (pin
   v1.64.8) or migrate to v2.x (different config schema). For now the
   spec'd v1.61.0 is kept.

---

## 5. GitHub branch-protection recommendations

Once this branch is merged and CI is observed green at least once on
`main`, configure the following in
**Settings → Branches → Branch protection rules** for `main`:

- **Require a pull request before merging** — yes
  - Require approvals: at least 1
  - Dismiss stale approvals on new commits: yes
  - Require review from Code Owners (after `CODEOWNERS` lands): yes
- **Require status checks to pass before merging** — yes
  - Require branches to be up to date before merging: yes
  - Required status checks (add by name from a successful run):
    - `Build / Test / Lint / Security` (the job from `ci.yml`)
- **Require conversation resolution before merging** — yes
- **Require signed commits** — recommended (security-sensitive repo)
- **Require linear history** — yes (forbids merge commits, keeps audit
  trail clean)
- **Do not allow bypassing the above settings** — yes; admins included
- **Restrict who can push to matching branches** — only release
  managers / CI service accounts
- **Allow force pushes** — **disabled**
- **Allow deletions** — **disabled**

Equivalent rule via the `gh` CLI (run after the workflow has been
merged so GitHub can resolve the check name):

```bash
gh api -X PUT repos/<owner>/<repo>/branches/main/protection \
  -F required_status_checks.strict=true \
  -F 'required_status_checks.contexts[]=Build / Test / Lint / Security' \
  -F enforce_admins=true \
  -F required_pull_request_reviews.required_approving_review_count=1 \
  -F required_pull_request_reviews.dismiss_stale_reviews=true \
  -F required_pull_request_reviews.require_code_owner_reviews=true \
  -F required_linear_history=true \
  -F allow_force_pushes=false \
  -F allow_deletions=false \
  -F restrictions=null
```

After enabling, the `golangci-lint`, `staticcheck`, and `govulncheck`
steps inside the CI job become merge-blocking — meeting the user's
acceptance criterion that strict tooling gates `main`.

---

## 6. Acceptance criteria checklist

- [x] `.golangci.yml`, `tools.go`, updated `Makefile`, `.github/workflows/ci.yml`, `WAVE1_G_BE_NOTES.md` exist
- [x] `go vet ./...` passes
- [x] `go build ./...` passes
- [x] `golangci-lint run` executed locally — produced ~200 findings
      (documented above; no fixes in this wave per spec)
- [x] No `.go` files inside `internal/`, `cmd/`, `pkg/`, `const/` were
      modified (only `Makefile` is in `git diff`; new files are at root
      and `.github/`)

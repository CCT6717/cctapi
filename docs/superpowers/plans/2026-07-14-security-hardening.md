# CCTAPI Security Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove unsafe default credentials, constrain browser access to token APIs, harden session requests against cookie theft and cross-site mutation, sanitize internal errors, and move the Go dependency baseline to versions without known reachable vulnerabilities.

**Architecture:** Keep browser sessions same-origin and apply credential-free CORS only to OpenAI-compatible token endpoints. Centralize session-cookie and CSRF policy in small, directly tested helpers. Treat application hardening and dependency remediation as separate commits so regressions can be isolated.

**Tech Stack:** Go, Gin, gin-contrib/sessions, gorilla cookie store, GORM, GitHub Actions, govulncheck, Vitest, Playwright.

---

## Task 1: Isolate The Implementation Branch

**Files:**
- Verify only: `.gitignore`, `AGENTS.md`

- [ ] **Step 1: Verify the main worktree state**

Run: `git status --short --branch`

Expected: `main` is ahead only by the approved design/plan commits; `.workbuddy/` may be untracked and must remain untouched.

- [ ] **Step 2: Create an isolated worktree**

Run: `git worktree add D:\ct\worktrees\security-hardening -b security/hardening`

Expected: the new worktree starts from the approved plan commit.

- [ ] **Step 3: Run the narrow baseline**

Run: `D:\ct\tools\go\bin\go.exe test ./middleware ./router ./model ./relay/adaptor/zhipu ./relay/adaptor/xunfei -count=1`

Expected: PASS before behavior changes.

## Task 2: Scope CORS To Credential-Free Token Endpoints

**Files:**
- Modify: `middleware/cors.go`
- Modify: `router/dashboard.go`
- Modify: `router/relay.go`
- Create: `router/cors_test.go`

- [ ] **Step 1: Write failing router-level CORS tests**

Add table-driven tests that register representative `/api`, `/api/fallback`, `/v1`, `/v1/models`, and dashboard billing routes. Assert:

```go
// Session-backed routes must not opt into cross-origin access.
if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
    t.Fatalf("unexpected CORS header %q", got)
}

// Token endpoints may be called cross-origin, but never with cookies.
if got := response.Header().Get("Access-Control-Allow-Origin"); got != "*" {
    t.Fatalf("got %q, want wildcard token CORS", got)
}
if got := response.Header().Get("Access-Control-Allow-Credentials"); got != "" {
    t.Fatalf("credentialed CORS must remain disabled")
}
```

Run: `D:\ct\tools\go\bin\go.exe test ./router -run CORS -count=1`

Expected: FAIL because relay CORS currently leaks to every route and advertises credentials.

- [ ] **Step 2: Replace the generic middleware with token API CORS**

Implement `TokenAPICORS()` with:

```go
config.AllowAllOrigins = true
config.AllowCredentials = false
config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
config.AllowHeaders = []string{
    "Authorization", "Content-Type", "Accept", "Origin",
    "X-API-Key", "Anthropic-Version", "Anthropic-Beta", "OpenAI-Beta",
}
```

Do not use `AllowHeaders: ["*"]` and do not advertise credentials.

- [ ] **Step 3: Attach CORS only to token route groups**

Apply `TokenAPICORS()` to the dashboard billing group, `/v1/models`, and `/v1`. Remove the engine-wide `router.Use(middleware.CORS())`. Do not add CORS to `/api` or `/api/fallback`.

- [ ] **Step 4: Verify scoped behavior**

Run: `D:\ct\tools\go\bin\go.exe test ./router -run CORS -count=1`

Expected: PASS.

## Task 3: Harden Session Cookies And Browser Mutations

**Files:**
- Create: `common/config/security.go`
- Create: `common/config/security_test.go`
- Create: `middleware/csrf.go`
- Create: `middleware/csrf_test.go`
- Modify: `main.go`

- [ ] **Step 1: Write failing cookie-policy tests**

Cover `SESSION_COOKIE_SECURE=true`, `false`, `auto` with HTTPS `SERVER_ADDRESS`, `auto` with localhost HTTP, and invalid values. Assert the resolved options include:

```go
sessions.Options{
    Path:     "/",
    MaxAge:   30 * 24 * 60 * 60,
    HttpOnly: true,
    SameSite: http.SameSiteLaxMode,
}
```

Invalid explicit values must return an error instead of silently weakening the cookie.

Run: `D:\ct\tools\go\bin\go.exe test ./common/config -run SessionCookie -count=1`

Expected: FAIL because the resolver does not exist.

- [ ] **Step 2: Implement the cookie-policy resolver**

Parse the mode without global init side effects. Default to `auto`; resolve HTTPS from `config.ServerAddress`. Return an error for unsupported values. Keep localhost HTTP usable by resolving `auto` to `Secure=false` there.

- [ ] **Step 3: Write failing CSRF middleware tests**

Table-test unsafe session requests:

| Case | Expected |
|---|---|
| `Sec-Fetch-Site: cross-site` | 403 |
| mismatched `Origin` | 403 |
| same-origin `Origin` | allowed |
| no Origin, mismatched Referer | 403 |
| same-origin Referer | allowed |
| no browser provenance headers | allowed for compatibility |
| no session cookie | allowed |
| GET/HEAD/OPTIONS with session cookie | allowed |

Run: `D:\ct\tools\go\bin\go.exe test ./middleware -run CSRF -count=1`

Expected: FAIL because the middleware does not exist.

- [ ] **Step 4: Implement the CSRF guard**

Normalize scheme/host using `net/url` and `Request.Host`. Apply only when the named session cookie is present and the method is unsafe. Reject obvious cross-site fetch metadata first, then validate `Origin`, then `Referer`. Return a generic 403 JSON response. Log only the rejection reason and request ID, never cookies or authorization values.

- [ ] **Step 5: Wire both policies into server initialization**

After creating the cookie store, assign the resolved options and install `sessions.Sessions("session", store)` followed immediately by `middleware.SessionCSRF("session")`. Fatal at startup if cookie configuration is invalid.

- [ ] **Step 6: Verify cookie and CSRF behavior**

Run: `D:\ct\tools\go\bin\go.exe test ./common/config ./middleware -count=1`

Expected: PASS.

## Task 4: Remove The Default Root Password

**Files:**
- Modify: `common/config/config.go`
- Modify: `model/main.go`
- Create: `model/main_test.go`

- [ ] **Step 1: Write failing root-bootstrap tests**

Extract password validation so it can be tested without a persistent database. Cover missing, shorter than 12 characters, `123456`, and valid strong values. Add an in-memory SQLite test proving an empty user table fails without a valid password and creates root with the supplied password when valid.

Run: `D:\ct\tools\go\bin\go.exe test ./model -run RootAccount -count=1`

Expected: FAIL because root still uses the hard-coded password.

- [ ] **Step 2: Read `INITIAL_ROOT_PASSWORD` from the environment**

Add `InitialRootPassword` beside the existing initial root token settings. Do not print it and do not add it to option APIs.

- [ ] **Step 3: Make empty-database bootstrap fail closed**

Use `errors.Is(err, gorm.ErrRecordNotFound)` to distinguish an empty database from an actual database error. Require a password of at least 12 characters and reject the legacy default. Hash only the supplied value. Check every `Create` error and return it.

Existing databases with at least one user must not require the environment variable and existing root credentials must remain unchanged.

- [ ] **Step 4: Verify root bootstrap behavior**

Run: `D:\ct\tools\go\bin\go.exe test ./model -run RootAccount -count=1`

Expected: PASS.

## Task 5: Sanitize Error Paths And Remove Dangerous Dead Code

**Files:**
- Modify: `middleware/auth.go`
- Modify: `middleware/auth_test.go`
- Modify: `common/helper/helper.go`
- Modify: `relay/adaptor/xunfei/main.go`
- Modify or create: `relay/adaptor/xunfei/main_test.go`

- [ ] **Step 1: Add a test for internal token lookup errors**

Introduce the smallest test seam around `CacheIsUserEnabled` needed to force an internal failure. Assert the response is a generic 500 message and the raw database error is absent.

Run: `D:\ct\tools\go\bin\go.exe test ./middleware -run TokenAuthInternalError -count=1`

Expected: FAIL because `err.Error()` currently reaches the client.

- [ ] **Step 2: Sanitize the response and retain server diagnostics**

Log the internal error using the project logger, return a stable generic message, and preserve the status code.

- [ ] **Step 3: Remove the unused trusted-HTML helper**

Delete `UnescapeHTML` and the now-unused `html/template` import. Confirm there are no callers with `rg -n "UnescapeHTML|html/template"`.

- [ ] **Step 4: Replace direct Xunfei stdout logging**

Change URL parsing to return safely on failure and use `logger.SysError` rather than `fmt.Println(err)`. Add a focused malformed-URL test if the helper signature changes.

- [ ] **Step 5: Run focused application tests**

Run: `D:\ct\tools\go\bin\go.exe test ./middleware ./common/helper ./relay/adaptor/xunfei -count=1`

Expected: PASS.

- [ ] **Step 6: Commit application-layer hardening**

Run:

```text
git add middleware router common/config common/helper model relay/adaptor/xunfei main.go
git commit -m "fix(security): harden browser sessions and bootstrap"
```

## Task 6: Upgrade JWT And Vulnerable Go Dependencies

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `relay/adaptor/zhipu/main.go`
- Modify or create: `relay/adaptor/zhipu/main_test.go`

- [ ] **Step 1: Pin the Go language baseline**

Set the module Go version to the supported local toolchain line and use Go 1.26.5 or newer for release/CI builds. Do not rebuild the release binary with the known-vulnerable Go 1.22.12 toolchain.

- [ ] **Step 2: Migrate JWT v3 to v5**

Replace `github.com/golang-jwt/jwt` with `github.com/golang-jwt/jwt/v5` at version `v5.2.2` or newer. Update the Zhipu import and add a test that parses the generated token with the expected HMAC method and claims.

- [ ] **Step 3: Upgrade vulnerable direct and transitive modules**

Bring these minimums to fixed versions, allowing `go get` to select compatible newer releases:

```text
github.com/aws/aws-sdk-go-v2/service/bedrockruntime >= v1.50.4
github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream >= v1.7.8
github.com/jackc/pgx/v5 >= v5.9.2
golang.org/x/image >= v0.43.0
golang.org/x/net >= v0.55.0
google.golang.org/grpc >= v1.79.3
```

Run `go mod tidy` after upgrades. Resolve compatibility errors at call sites rather than suppressing checks.

- [ ] **Step 4: Run focused and full Go tests**

Run:

```text
D:\ct\tools\go\bin\go.exe test ./relay/adaptor/zhipu -count=1
D:\ct\tools\go\bin\go.exe test ./... -count=1
D:\ct\tools\go\bin\go.exe vet ./...
```

Expected: PASS.

## Task 7: Add A Pinned Vulnerability Gate To CI

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Update CI toolchains**

Use an explicit patched Go version consistently in the Go and E2E jobs. The application build and scanner must run on the same supported release line.

- [ ] **Step 2: Add a pinned govulncheck step**

Install a fixed `golang.org/x/vuln/cmd/govulncheck` version and run `govulncheck ./...` after tests. Do not use `@latest` in CI.

- [ ] **Step 3: Validate workflow syntax and scan locally**

Run:

```text
D:\ct\tools\go\bin\go.exe run golang.org/x/vuln/cmd/govulncheck@<pinned-version> ./...
git diff --check
```

Expected: no reachable vulnerability findings and no whitespace errors.

- [ ] **Step 4: Commit dependency and CI remediation**

Run:

```text
git add go.mod go.sum relay/adaptor/zhipu .github/workflows/ci.yml
git commit -m "chore(security): refresh Go dependencies and scanning"
```

## Task 8: Full Verification And Runtime Acceptance

**Files:**
- Modify generated assets only if a normal build changes tracked output: `web/build/default/**`
- Update: `AGENTS.md` only with verified final state

- [ ] **Step 1: Run the complete backend gate**

Run:

```text
D:\ct\tools\go\bin\go.exe test ./... -count=1
D:\ct\tools\go\bin\go.exe test -race ./fallback ./controller ./middleware ./common ./router -count=1
D:\ct\tools\go\bin\go.exe vet ./...
```

Expected: PASS.

- [ ] **Step 2: Run the complete frontend gate**

From `web/default`, run:

```text
npm run lint
npm test
npm run build
npm run build-storybook -- --quiet
```

Expected: PASS with zero lint errors or warnings.

- [ ] **Step 3: Build the Windows binary with patched Go and CGO**

Set `CGO_ENABLED=1`, put `D:\ct\tools\w64devkit\bin` on PATH, and build `one-api.exe` with the patched Go toolchain. Confirm `go version -m one-api.exe` reports the expected Go and dependency versions.

- [ ] **Step 4: Start an isolated acceptance server**

Use a temporary database and `INITIAL_ROOT_PASSWORD` on a non-production port first. Verify:

```text
empty DB without INITIAL_ROOT_PASSWORD -> startup fails
empty DB with strong INITIAL_ROOT_PASSWORD -> startup succeeds
login Set-Cookie -> HttpOnly and SameSite=Lax present
cross-site session mutation -> 403
same-origin session mutation -> reaches its normal route response
/api preflight -> no CORS opt-in
/v1 preflight -> wildcard origin, no credential header
```

- [ ] **Step 5: Run Playwright and live 3008 smoke checks**

Run `npm run test:e2e`, then restart the real `3008` service using its existing database. Verify `/`, `/api/status`, `/fallback/free-pool`, and `/v1/models` respond as expected. Do not change existing root credentials.

- [ ] **Step 6: Inspect the final diff**

Run:

```text
git status --short
git diff --check
git diff --stat main...HEAD
git log --oneline main..HEAD
```

Expected: only planned security, dependency, CI, test, generated build, and verified handoff changes.

## Task 9: Review, Merge, Push, And Confirm CI

**Files:**
- Update: `AGENTS.md`

- [ ] **Step 1: Request a focused code review**

Review for route-scope regressions, reverse-proxy origin handling, empty-database startup behavior, accidental secret logging, dependency incompatibilities, and missing negative tests. Fix all high-confidence findings and rerun affected gates.

- [ ] **Step 2: Record the verified state**

Update `AGENTS.md` with the new security commits, required `INITIAL_ROOT_PASSWORD` behavior for new installs, cookie mode environment variable, build toolchain, and exact verification results.

- [ ] **Step 3: Commit final verification metadata**

Run: `git commit -am "docs: record security hardening verification"`

- [ ] **Step 4: Merge into main and push**

From `D:\ct\project`, fast-forward or merge `security/hardening` after confirming the main worktree has no new conflicting user changes. Push `main` to `origin`.

- [ ] **Step 5: Confirm remote CI**

Use the connected GitHub remote to confirm the pushed commit and all required workflow jobs. Report any external CI failure separately from local verification.

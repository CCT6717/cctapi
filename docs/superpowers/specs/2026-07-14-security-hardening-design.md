# CCTAPI Security Hardening Design

Date: 2026-07-14

## Objective

Remove the confirmed default-credential and session-hardening weaknesses, make
CORS behavior match the authentication boundary, add browser CSRF defenses
without breaking local or token-based clients, and upgrade the running binary
away from dependencies and a Go toolchain with reachable vulnerabilities.

This work takes priority over the pending Kilo model-level 429 rotation. The
Kilo work resumes after the security patch passes runtime acceptance.

## Confirmed Baseline

- An empty database creates `root` with the fixed password `123456`.
- The session cookie does not explicitly set `Secure`, `HttpOnly`, or
  `SameSite`.
- The CORS middleware is registered at engine scope and currently emits
  `Access-Control-Allow-Origin: *` together with
  `Access-Control-Allow-Credentials: true` for admin and relay routes.
- Login already has `CriticalRateLimit`; this design preserves it.
- `ValidateUserToken` already converts most storage failures to stable client
  messages, but selected middleware branches still return internal errors.
- `UnescapeHTML` returns trusted `template.HTML` and has no callers.
- The current `one-api.exe` was built with Go 1.22.12. Official source and
  binary `govulncheck` scans report reachable standard-library and dependency
  vulnerabilities.

## Compatibility Constraints

- `http://127.0.0.1:3008` must remain usable for local development and
  acceptance.
- Same-origin browser login and the existing OAuth callbacks must continue to
  work.
- OpenAI-compatible `/v1` clients must continue to work cross-origin with
  bearer or `x-api-key` authentication.
- Existing session-less scripts must not be forced to implement a CSRF token.
- The existing database and current root account must not be modified.
- No secret, generated initial password, session cookie, or provider key may be
  committed or written to acceptance evidence.

## Chosen Approach

Use route-scoped credential-free CORS for token APIs, explicit session-cookie
options, and a browser request-origin guard for session-authenticated unsafe
requests. This provides a meaningful CSRF boundary without requiring a
cross-cutting frontend token migration.

A synchronizer-token design was rejected for this slice because it would
require changing every browser mutation, OAuth-adjacent behavior, test fixture,
and automation client. A configuration-only change was rejected because it
would leave the browser write boundary implicit.

## CORS Boundary

Replace the current engine-wide `middleware.CORS()` registration with a
credential-free token API middleware:

- Attach it directly to `/v1`, `/v1/models`, and the token-authenticated
  dashboard billing groups.
- Do not attach CORS middleware to `/api`, `/api/fallback`, or browser pages.
- Allow all origins only for token APIs.
- Set `AllowCredentials` to `false`.
- Explicitly allow the methods and headers required by OpenAI-compatible
  clients, including `Authorization`, `Content-Type`, and `x-api-key`.
- Handle preflight before token authentication.

This keeps public API compatibility while making browser admin APIs
same-origin.

## Session Cookie Policy

Configure the cookie store before installing the session middleware:

- `Path=/`
- `MaxAge=30 days`
- `HttpOnly=true`
- `SameSite=Lax`
- `Secure` resolved from `SESSION_COOKIE_SECURE`

`SESSION_COOKIE_SECURE` accepts `true`, `false`, or `auto` and defaults to
`auto`. In auto mode it is enabled when `SERVER_ADDRESS` uses HTTPS and disabled
for the local HTTP acceptance server. Production documentation must require an
explicit `true` value when TLS terminates at a reverse proxy and the public
address is not otherwise visible to the process.

`SameSite=Lax` is selected instead of Strict to preserve OAuth top-level return
navigation. `SameSite=None` is not supported for the admin session.

## Browser CSRF Guard

Install a CSRF guard after session middleware and before route registration.
It applies only when all of the following are true:

- the method is not `GET`, `HEAD`, or `OPTIONS`;
- the request carries the session cookie.

Requests without the session cookie remain token or anonymous traffic and do
not enter the CSRF guard. When both a session cookie and an API token are
present, the request is treated as session traffic because the existing admin
authentication helper prefers the session.

The guard follows these rules:

1. Reject `Sec-Fetch-Site: cross-site`.
2. If `Origin` is present, require its host to match `Request.Host`.
3. Otherwise, if `Referer` is present, require its host to match
   `Request.Host`.
4. If all browser-origin headers are absent, allow the request for compatibility
   with non-browser session automation and log the compatibility path at debug
   level.
5. Return a stable JSON `403` response without reflecting the supplied origin.

Host comparison normalizes case and default ports and rejects malformed URL
headers. A sibling subdomain is not considered same-origin.

This is deliberate defense in depth rather than a claim that CORS itself is a
CSRF control.

## Initial Root Account

When no user exists, `CreateRootAccountIfNeed` must read
`INITIAL_ROOT_PASSWORD` and refuse startup when the value is missing, shorter
than 12 characters, or equal to the former default `123456`.

The startup error names the required environment variable but never prints its
value. Existing databases bypass this requirement, so the currently configured
root account is unaffected. `INITIAL_ROOT_TOKEN` and
`INITIAL_ROOT_ACCESS_TOKEN` retain their current behavior.

A generated password printed to logs was rejected because log retention turns
the credential into another secret-distribution channel. A first-run setup UI
is outside this focused patch.

## Error Disclosure And Dangerous Helpers

- Replace raw internal errors returned from token-auth middleware with a stable
  generic client message and structured server logging.
- Preserve specific user-actionable token states such as invalid, expired, or
  exhausted.
- Remove the unused `UnescapeHTML` helper and its `html/template` dependency.
- Replace the identified `fmt.Println(err)` relay error output with the project
  logger without changing upstream response behavior.

## Dependency And Toolchain Upgrade

Upgrade to versions that clear the reachable findings reported by the current
Go vulnerability database, including:

- Go build and CI toolchain: Go 1.26.5 or newer patched release in the same
  supported line.
- `github.com/golang-jwt/jwt/v5`: a release containing the header-allocation
  fix.
- AWS SDK eventstream and Bedrock Runtime: at least the fixed versions reported
  by `govulncheck`.
- `golang.org/x/net`, `golang.org/x/image`, `github.com/jackc/pgx/v5`, and
  `google.golang.org/grpc`: at least the fixed versions reported by
  `govulncheck`.

Use `go get` and `go mod tidy` rather than hand-editing indirect requirements.
Adapt JWT call sites to the v5 API and keep token claims and signing behavior
unchanged.

Add a pinned `govulncheck` invocation to CI. The gate fails on reachable symbol
findings. Informational module-only findings may be documented separately but
must not be silently ignored.

## Testing Strategy

Write failing tests before implementation for each behavior:

- Admin `/api` responses do not expose CORS headers to an untrusted origin.
- `/v1` preflight succeeds without credential support and allows required token
  headers.
- Login session cookies include `HttpOnly`, `SameSite=Lax`, and the configured
  `Secure` value.
- Cross-site session-authenticated unsafe requests return JSON 403.
- Same-origin browser writes, safe requests, and session-less token requests
  continue to pass.
- Empty-database root creation rejects missing, short, and legacy default
  passwords and accepts a valid environment-provided password.
- Internal middleware failures return a generic response while preserving the
  server-side error log.

Then run:

- focused Go tests for middleware, router, model, and authentication;
- `go test ./... -count=1`;
- scoped race tests used by CI;
- frontend lint, Vitest, Vite build, Storybook build, and Playwright;
- source and binary `govulncheck`;
- a rebuilt `one-api.exe` on port 3008;
- real login, cookie-header, admin mutation, `/v1` preflight, and fallback page
  smoke checks.

## Rollout And Recovery

Commit application hardening separately from dependency upgrades so failures
are attributable. Rebuild the executable only after both commits pass tests.
Keep the previous executable until the new process passes port-3008 acceptance.

If a compatibility regression appears, revert the narrow application-hardening
commit rather than disabling all security controls. Do not restore the fixed
default password or credentialed wildcard CORS as a workaround.

## Acceptance Criteria

- A fresh database cannot create a predictable root credential.
- The local HTTP login flow works and emits the explicit non-production cookie
  policy.
- HTTPS production configuration emits a Secure session cookie.
- Untrusted origins cannot use the admin session for browser writes or read
  admin responses through CORS.
- Cross-origin bearer-token `/v1` clients remain functional.
- Existing OAuth and current-database startup behavior remain functional.
- Full test, build, race, browser, and runtime smoke suites pass.
- Source and rebuilt-binary vulnerability scans contain no reachable findings
  addressed by this scope; any remaining finding has a documented applicability
  decision and follow-up owner.

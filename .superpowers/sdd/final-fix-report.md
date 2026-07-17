# Precise Attempt Observability Final Fix Report

Date: 2026-07-17

## Status

- Status: complete
- Worktree: `D:\ct\worktrees\attempt-observability`
- Branch: `codex/attempt-observability`
- Starting head: `0b59f147507d1ffa383be060276c10016308bda7`
- Fix commit: `ed3e14e17dca9e6bda77058f383855c6ad346b86`
- Push / PR: not performed

## Implemented Fixes

- Normalized zero attempt timestamps before process-local trace recording and persistence.
- Recorded every relay attempt outcome exactly once, including routine success, while keeping success out of SQLite.
- Added a real Kilo 429-to-success relay integration assertion covering the complete process-local chain and failure-only persistence.
- Sorted request-chain steps by plan index, upstream attempt index, timestamp, and ID with stable exact ties.
- Kept top deployment/provider/model aggregates at five and raised error-category/outcome aggregates to ten.
- Replaced wildcard skip matching with the exact five backend skip outcomes and excluded `skippedXfoo`.
- Removed the legacy approximate `Top 3 失败模型` block from `MetricsPanel` without changing switch counts or switch-log UI.
- Added Chinese labels for concurrency, channel, and model-state skips; removed the unsupported cooldown label.
- Added the optional deferred-unmount hook regression without changing hook production behavior.

## TDD RED Evidence

The first controller RED attempt exposed a test-fixture compile error (`ctxkey.RequestId` did not exist). The test was corrected to use the production key `helper.RequestIdKey`, then all behavioral RED cases were rerun before production changes.

### Zero timestamp

```powershell
$env:CGO_ENABLED='1'
$env:PATH='D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
& 'D:\ct\tools\go\bin\go.exe' test ./fallback -run '^TestRecordAttemptEventIfWorthyNormalizesTimestampBeforeTraceAndPersistence$' -count=1
```

Result: failed as expected because the process-local trace timestamp was `0001-01-01 00:00:00 UTC`.

### Complete live relay chain

```powershell
& 'D:\ct\tools\go\bin\go.exe' test ./controller -run '^TestRelayWithFallbackRotatesKiloModels$/^rate_limit_rotates_to_next_Kilo_model_without_provider_penalty$' -count=1
```

Result: failed as expected because the chain contained only the persisted Kilo model 429 step and omitted terminal success.

### Route-step order

```powershell
& 'D:\ct\tools\go\bin\go.exe' test ./fallback -run '^TestRecentAttemptChainsSortStepsByRouteOrder$' -count=1
```

Result: failed as expected for reversed timestamps and equal-timestamp upstream attempts because timestamp/insertion order overrode route indices.

### Aggregate limits and exact skip set

```powershell
& 'D:\ct\tools\go\bin\go.exe' test ./fallback -run '^TestAttemptObservabilityKeepsTenDetailsAndUsesExactSkipOutcomes$' -count=1
```

Result: failed as expected because error categories were truncated to five; the regression also covers nine valid outcomes and exclusion of `skippedXfoo`.

### Legacy ranking and outcome labels

```powershell
cd web/default
npm test -- src/pages/Fallback/panels/MetricsPanel.test.jsx
```

Result: 2 expected failures: the legacy heading/sentinel rendered, and the three required skip outcomes rendered as `未知结果`.

## Focused GREEN Evidence

- Timestamp regression: passed.
- Real Kilo 429-to-success relay regression: passed; chain has two ordered steps and SQLite has only the 429 event.
- Route-order regression: passed for plan index, upstream attempt index, and stable exact ties.
- Aggregate/skip regression: passed with seven categories, nine valid outcomes, five exact skips, and no `skippedXfoo`.
- `MetricsPanel.test.jsx`: 7/7 passed.
- `useAttemptObservability.test.jsx`: 4/4 passed, including deferred resolution after unmount.

## Final Verification

```powershell
$env:CGO_ENABLED='1'
$env:PATH='D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
& 'D:\ct\tools\go\bin\go.exe' test ./fallback ./controller -count=1
& 'D:\ct\tools\go\bin\go.exe' test -race ./fallback ./controller -count=1

cd web/default
npm test -- src/pages/Fallback/hooks/useAttemptObservability.test.jsx src/pages/Fallback/panels/MetricsPanel.test.jsx src/pages/Fallback/Fallback.test.jsx
npm run lint

cd ../..
git diff --check
```

Results:

- Go focused packages: `fallback` and `controller` passed, 0 failures.
- Go race: `fallback` and `controller` passed, 0 races and 0 failures.
- Frontend focused suite: 3 files passed, 15/15 tests passed.
- ESLint: passed with 0 errors and 0 warnings.
- `git diff --check`: passed with no whitespace errors; Git emitted only the repository's LF-to-CRLF conversion notices.

## Changed Files

- `controller/relay.go`
- `controller/relay_fallback_model_rotation_test.go`
- `fallback/attempt_event.go`
- `fallback/attempt_event_test.go`
- `fallback/attempt_observability.go`
- `fallback/attempt_observability_test.go`
- `fallback/attempt_trace.go`
- `fallback/attempt_trace_test.go`
- `web/default/src/pages/Fallback/hooks/useAttemptObservability.test.jsx`
- `web/default/src/pages/Fallback/panels/MetricsPanel.js`
- `web/default/src/pages/Fallback/panels/MetricsPanel.test.jsx`
- `.superpowers/sdd/final-fix-report.md`

## Self-Review And Concerns

- Reviewed the complete owned diff against every requirement in `final-fix-brief.md`; no missing requirement, unrelated change, unsafe payload exposure, or blocking issue was found.
- The frontend test output still contains pre-existing Semantic UI React deprecation warnings (`defaultProps` and `findDOMNode`). They are non-blocking and outside this fix wave; ESLint remains clean.
- Verification was intentionally scoped to the exact package/file/race/lint commands required by the brief; no full-repository build or E2E run was requested for this wave.

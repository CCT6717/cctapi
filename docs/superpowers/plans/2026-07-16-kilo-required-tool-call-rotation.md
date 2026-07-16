# Kilo Required Tool Call Rotation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rotate to another compatible Kilo model when a non-streaming request explicitly requires a tool call but the upstream returns HTTP 200 without any tool call.

**Architecture:** Extend the existing buffered Kilo tools-response validator. Preserve current argument validation and pass a request-derived tool-choice contract that distinguishes optional, prohibited, required, and explicitly selected functions. Reuse the existing model capability false-positive isolation, metrics, event recording, and fallback path.

**Tech Stack:** Go, Gin, existing fallback relay integration tests.

## Global Constraints

- Apply only to non-streaming Kilo attempts whose request includes tools.
- Do not reject missing tool calls for `tool_choice: "auto"`, `"none"`, or an omitted tool choice.
- Do not add provider failure, rate-limit score, provider cooldown, or same-deployment switch events.
- Do not settle quota or persist provider success until the buffered tool response passes validation; rejected attempts must return pre-consumed quota.
- Do not expose raw upstream response bodies or credentials.

---

### Task 1: Lock the required-tool behavior with relay tests

**Files:**
- Modify: `controller/relay_fallback_model_rotation_test.go`

**Interfaces:**
- Consumes: `relayWithFallbackUsing`, `newRelayRotationRequiredToolsContext`, `newRelayRotationAutoToolsContext`.
- Produces: regression coverage for required-tool rotation and auto-tool pass-through.

- [ ] **Step 1: Write a failing required-tool test**

Create a Kilo attempt that returns a valid chat completion without `tool_calls`, followed by a second Kilo model with valid JSON tool arguments. Assert that only the second response reaches the client, the first model is isolated, and provider accounting remains unpenalized.

- [ ] **Step 2: Write the auto-tool compatibility test**

Send the same no-tool response with `tool_choice: "auto"`. Assert that the first Kilo model succeeds without isolation or replay.

- [ ] **Step 3: Verify RED**

Run:

```powershell
$env:CGO_ENABLED='1'
$env:PATH='D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH
& 'D:\ct\tools\go\bin\go.exe' test ./controller -run 'TestRelayWithFallbackRotatesKiloModels/missing_required_Kilo_tool_call' -count=1
```

Expected: FAIL because the current validator accepts a response with no tool calls.

### Task 2: Extend request-aware tool validation

**Files:**
- Modify: `controller/relay.go`
- Test: `controller/relay_fallback_model_rotation_test.go`

**Interfaces:**
- Produces: `parseToolChoice(body []byte) toolChoiceContract` and request-aware tool-call validation.

- [ ] **Step 1: Parse explicit tool-call requirements**

Preserve `required`, `none`, selected-function names, Responses flat function selection, and Anthropic `any`/`tool` forms. Treat omitted and `auto` as optional.

- [ ] **Step 2: Enforce required calls per supported response schema**

For Chat Completions require each choice to satisfy the contract, for Responses validate `function_call` output items, and for Anthropic validate `tool_use` content blocks. Reject calls under `none`, reject mismatched selected names, and continue validating every emitted argument object.

- [ ] **Step 3: Reuse existing isolation and rotation flow**

Pass the request-derived flag into buffered response validation. Keep the safe `model_capability_false_positive` event and provider-neutral accounting unchanged.

- [ ] **Step 4: Delay quota settlement until validation**

Store a one-shot commit/rollback callback for buffered Kilo tool attempts. Commit usage only after validation succeeds; return pre-consumed quota before rotating when validation fails.

- [ ] **Step 5: Verify GREEN**

Run focused controller tests, scoped race tests, full Go tests, and `go build ./...` with the repository CGO toolchain.

### Task 3: Deploy and perform targeted acceptance

**Files:**
- Create: `docs/evidence/soak-kilo-required-tool-call-rotation-2026-07-16.json`

**Interfaces:**
- Consumes: rebuilt `one-api.exe`, restored production routing configuration, and `scripts/soak-test.py`.
- Produces: sanitized evidence showing valid tools responses and, when reproduced upstream, model capability false-positive rotation.

- [ ] **Step 1: Rebuild and restart port 3008**

Build the default frontend first, then rebuild the Go binary and restart the local server.

- [ ] **Step 2: Run a temporary Kilo-first tools-only soak**

Back up `data/fallback.json`, temporarily prefer Kilo and the known weak tools model, run paced requests, and restore the original file in a `finally` block.

- [ ] **Step 3: Verify evidence safety and runtime restoration**

Require zero credential/raw-body markers, confirm the original routing configuration is restored, and confirm port 3008 returns HTTP 200.

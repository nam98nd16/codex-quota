# OpenCode Quota Event Auto-Switch Plan

## Goal

Make `Auto switch exhausted` use OpenCode quota-exhaustion events as the primary trigger, so `cq` can switch records immediately when OpenCode reports the active record is exhausted.

The implementation must also preserve a safe fallback for users who have not installed the OpenCode plugin yet.

## Key Behavioral Requirements

- `Auto switch exhausted` remains the master setting.
- When OpenCode emits an exhausted/quota event for the currently applied OpenCode record, `cq` must force-switch to the best replacement record.
- The forced event path must not depend on the quota API reporting `0%`, because OpenCode can correctly report exhaustion while the quota API still shows `1%` or `2%`.
- If the OpenCode plugin is not installed or events are unavailable, fallback behavior must still switch when the currently applied record has `<= 3%` quota left.
- Existing manual smart-switch behavior must continue to work.
- Existing periodic refresh behavior must remain available as a compatibility fallback.

## Recommended Mode Model

Keep the existing setting:

- `Auto switch exhausted`: `on | off`

Add a new setting:

- `Auto switch trigger`: `event + fallback | event only | legacy refresh only`

Recommended/default:

- `event + fallback`

Mode behavior:

- `event + fallback`: OpenCode exhausted events force a switch immediately; legacy refresh fallback switches at `<= 3%` when events are unavailable or missed.
- `event only`: OpenCode exhausted events force a switch immediately; legacy low-quota fallback is disabled.
- `legacy refresh only`: ignore OpenCode plugin events; switch from refresh data at `<= 3%`.

Compatibility:

- Existing settings with `auto_switch_exhausted: true` and no trigger value should normalize to `event + fallback`.
- Existing settings with `auto_switch_exhausted: false` remain off.
- Invalid or missing trigger values should normalize to `event + fallback`.

## Architecture

Use a global OpenCode plugin as the event source and a small local `cq` listener as the receiver.

Flow:

```text
OpenCode provider error
  -> OpenCode event bus
  -> global codex-quota OpenCode plugin
  -> POST localhost cq listener
  -> Bubble Tea message
  -> forced switch or fallback refresh path
  -> ApplyToTargetsCmd
```

Important rule:

- The OpenCode plugin must never directly edit auth files or choose a replacement record.
- The plugin only reports that OpenCode observed a quota/exhaustion condition.
- `cq` remains the single authority for selecting and applying replacement records.

## Event Semantics

OpenCode has no official dedicated `quota.exhausted` event.

Use these supported event surfaces:

- Plugin `event` hook.
- `session.error` events.
- `message.updated` events where assistant message `info.error` exists.

Prefer `message.updated` when available because assistant messages include `providerID` and `modelID`.

Use `session.error` as fallback because it is explicitly emitted by OpenCode session processing for provider failures.

## Plugin Classifier

The global plugin should classify provider errors as quota/exhaustion signals.

Strong signals:

- `error.name === "APIError"`
- `statusCode === 429`
- error text includes one or more quota/exhaustion terms

Quota/exhaustion terms:

- `quota`
- `exhausted`
- `exceeded`
- `usage limit`
- `rate limit`
- `rate_limit_exceeded`
- `insufficient_quota`
- `billing`
- `credits`

Provider filtering:

- If `providerID` is known and is not OpenAI/OpenCode-compatible for this tool's managed record, ignore by default.
- If `providerID` is missing but the error is strongly quota-like, forward the event; `cq` will only act if an OpenCode-applied managed record exists.

## Local Listener

Add a new package, for example:

- `internal/opencodehook`

Responsibilities:

- Start a local HTTP server while the `cq` TUI is running.
- Bind only to `127.0.0.1`.
- Use a random per-run bearer token.
- Write listener state to the existing `codex-quota` config directory.
- Remove the state file on clean shutdown if it still belongs to this process.

State file example:

```json
{
  "version": 1,
  "url": "http://127.0.0.1:54321",
  "token": "random-token",
  "pid": 12345,
  "started_at": "2026-06-09T00:00:00Z"
}
```

Endpoint:

```text
POST /v1/opencode/quota-signal
Authorization: Bearer <token>
```

Payload example:

```json
{
  "source": "opencode",
  "event_type": "message.updated",
  "session_id": "ses_...",
  "provider_id": "openai",
  "model_id": "...",
  "error_name": "APIError",
  "status_code": 429,
  "message": "quota exceeded",
  "response_body": "...",
  "received_at": "2026-06-09T00:00:00Z"
}
```

Security:

- Reject requests without the bearer token.
- Limit request body size.
- Ignore malformed payloads.
- Never expose the listener on non-loopback interfaces.

## TUI Integration

Add a Bubble Tea message, for example:

```go
type OpenCodeQuotaSignalMsg struct {
    SessionID    string
    ProviderID   string
    ModelID      string
    ErrorName    string
    StatusCode   int
    Message      string
    ResponseBody string
    ReceivedAt   time.Time
}
```

When received:

- If `AutoSwitchExhausted` is off, ignore.
- If trigger mode is `legacy refresh only`, ignore.
- Find the currently applied OpenCode record.
- If no OpenCode-applied managed record exists, ignore.
- If the OpenCode-applied record is already loading or switching, debounce.
- Force-switch to the best replacement record without requiring quota API confirmation.

This event path intentionally differs from the normal refresh path:

- Refresh path uses quota API data and threshold checks.
- Event path trusts OpenCode's exhaustion signal because the quota API can incorrectly show `1%` or `2%` remaining.

## Forced Event Switch Logic

Add a dedicated function, for example:

```go
func (m *Model) forceAutoSwitchAppliedOpenCodeAccount(reason string) tea.Cmd
```

Behavior:

- Require `m.Settings.AutoSwitchExhausted == true`.
- Require trigger mode to allow event handling.
- Resolve the account currently applied to OpenCode.
- Exclude all currently applied records from replacement candidates, preserving current split-target behavior.
- Pick replacement using existing `bestReplacementAccounts` ranking.
- Select the replacement in the UI.
- Apply to OpenCode, and ideally to the same targets as current auto-switch behavior if that remains intended.
- Reuse `ApplyToTargetsCmd` and `syncAndFetchActiveAccount`.
- Set a notice such as `OpenCode reported quota exhausted; switched account`.

Avoid regressions:

- Do not alter `bestReplacementAccounts` ranking unless needed.
- Do not change manual smart-switch behavior.
- Do not require the exhausted sticky state to be updated before switching.
- Do not require `isCompactAccountExhausted` for event-forced switches.

## Fallback Threshold Logic

Current behavior switches only when the account is considered exhausted.

Change fallback behavior for auto-switch to use a threshold:

```text
switch when watched quota window left percent <= 3.0
```

Apply this threshold only to automatic fallback switching, not necessarily to UI labels.

Suggested constants:

```go
const autoSwitchFallbackThresholdPercent = 3.0
```

Fallback check:

- For the currently applied record, inspect `watchedAutoSwitchWindow(data)`.
- If no watched window exists, preserve existing behavior or avoid switching.
- If `LeftPercent <= 3.0`, switch to replacement.
- If `LeftPercent > 3.0`, keep the current account.

This handles the known quota display bug where `1%` or `2%` can actually mean exhausted.

## Refactoring Existing Switch Code

Current switch path:

```text
DataMsg -> maybeAutoSwitchAfterRefresh -> isCompactAccountExhausted -> bestReplacementAccounts -> ApplyToTargetsCmd
```

Refactor into shared primitives:

- `autoSwitchReplacementForAppliedAccount(accountKey string) *config.Account`
- `applyAutoSwitchReplacement(replacement *config.Account) tea.Cmd`
- `shouldAutoSwitchAfterRefresh(accountKey string) bool`
- `forceAutoSwitchAppliedOpenCodeAccount(reason string) tea.Cmd`

Keep the existing `maybeAutoSwitchAfterRefresh` function as the refresh/fallback path.

Updated refresh/fallback behavior:

- Manual smart-switch keeps existing confirmed-exhausted behavior.
- Automatic smart-switch uses `<= 3%` threshold when trigger mode is `event + fallback` or `legacy refresh only`.
- Automatic smart-switch is skipped in `event only` unless it is responding to an OpenCode plugin event.

## Plugin Installation Commands

Add CLI commands:

```text
cq opencode-plugin install
cq opencode-plugin status
cq opencode-plugin uninstall
```

Install behavior:

- Create `~/.config/opencode/plugins/` if missing.
- Write `codex-quota.js`.
- Print that OpenCode must be restarted.

Status behavior:

- Report whether the plugin file exists.
- Report whether a `cq` listener state file exists.
- Optionally report whether the plugin has recently sent a `plugin-started` heartbeat.

Uninstall behavior:

- Remove only the managed `codex-quota.js` plugin file.
- Do not edit unrelated OpenCode config.

## Global Plugin Source Shape

Plugin file path:

```text
~/.config/opencode/plugins/codex-quota.js
```

Plugin behavior:

- Export an async plugin function.
- Return an `event` hook.
- On each event, classify quota-like errors.
- Read the `cq` state file.
- POST to `cq` listener with bearer token.
- Use short timeout.
- Swallow failures silently when `cq` is not running.

OpenCode loads plugin files at startup, so after installation the user must restart OpenCode.

## UI Copy

Settings help text should explain:

- `Auto switch exhausted` uses OpenCode exhausted events when the plugin is installed.
- If plugin events are unavailable, fallback switches at `<= 3%` quota.
- Install with `cq opencode-plugin install` and restart OpenCode.

Example:

```text
Auto switch exhausted uses OpenCode exhausted events when available.
Fallback switches when the applied record has <= 3% quota.
Run `cq opencode-plugin install`, then restart OpenCode.
```

## Test Plan

Settings tests:

- Missing trigger mode normalizes to `event + fallback`.
- Invalid trigger mode normalizes to `event + fallback`.
- Existing `AutoSwitchExhausted` values are preserved.

Plugin classifier tests:

- `APIError 429 quota exceeded` sends signal.
- `APIError 429 rate limit exceeded` sends signal.
- `APIError 500` does not send signal.
- Non-OpenAI provider with quota text is ignored when provider is known.

Listener tests:

- Valid token accepts signal and sends `OpenCodeQuotaSignalMsg`.
- Missing token rejects request.
- Wrong token rejects request.
- Oversized body rejects request.
- Malformed JSON rejects request.

Switch tests:

- Event signal force-switches even when quota API data shows `2%` remaining.
- Event signal force-switches even when quota API data is missing.
- Event signal is ignored when `AutoSwitchExhausted` is off.
- Event signal is ignored in `legacy refresh only` mode.
- Event signal does not switch when no replacement account exists.
- Fallback switches when watched quota is `3%`.
- Fallback switches when watched quota is `1%` or `2%`.
- Fallback does not switch when watched quota is `3.1%`.
- `event only` does not switch from fallback threshold.
- Manual smart-switch behavior remains unchanged.

CLI tests:

- `cq opencode-plugin install` writes the managed plugin file.
- `cq opencode-plugin status` reports installed/not installed.
- `cq opencode-plugin uninstall` removes only the managed plugin file.
- Help text includes plugin commands.

## Implementation Order

1. Add trigger mode setting and normalization.
2. Add fallback threshold helper for `<= 3%` auto-switch decisions.
3. Refactor existing switch code into shared replacement/apply helpers.
4. Add forced OpenCode event switch path that bypasses quota confirmation.
5. Add local listener and state file.
6. Wire listener to Bubble Tea via `program.Send`.
7. Add plugin installer/status/uninstall commands.
8. Embed/write the OpenCode plugin source.
9. Update settings UI text.
10. Add tests.
11. Update README with setup and behavior notes.

## Non-Goals

- Do not depend on discovering the OpenCode TUI server port.
- Do not require OpenCode to use a fixed `--port`.
- Do not make the plugin directly edit OpenCode auth files.
- Do not remove legacy refresh behavior.
- Do not change account ranking unless a separate bug requires it.

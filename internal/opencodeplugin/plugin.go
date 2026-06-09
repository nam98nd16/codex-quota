package opencodeplugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deLiseLINO/codex-quota/internal/opencodehook"
)

const pluginFileName = "codex-quota.js"

type Status struct {
	PluginPath string
	Installed  bool
	StatePath  string
	ListenerUp bool
}

func Install() (string, error) {
	path, err := PluginPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(pluginSource), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func Uninstall() (string, bool, error) {
	path, err := PluginPath()
	if err != nil {
		return "", false, err
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return path, false, nil
		}
		return path, false, err
	}
	return path, true, nil
}

func CheckStatus() (Status, error) {
	pluginPath, err := PluginPath()
	if err != nil {
		return Status{}, err
	}
	statePath, err := opencodehook.StatePath()
	if err != nil {
		return Status{}, err
	}
	status := Status{PluginPath: pluginPath, StatePath: statePath}
	if _, err := os.Stat(pluginPath); err == nil {
		status.Installed = true
	} else if !os.IsNotExist(err) {
		return Status{}, err
	}
	if _, err := os.Stat(statePath); err == nil {
		status.ListenerUp = true
	} else if !os.IsNotExist(err) {
		return Status{}, err
	}
	return status, nil
}

func PluginPath() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("CQ_OPENCODE_PLUGIN_DIR")); dir != "" {
		return filepath.Join(cleanHomePath(dir), pluginFileName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", fmt.Errorf("failed to locate home directory")
	}
	return filepath.Join(home, ".config", "opencode", "plugins", pluginFileName), nil
}

func cleanHomePath(path string) string {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "~") {
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			rest := strings.TrimLeft(strings.TrimPrefix(path, "~"), "/\\")
			if rest == "" {
				return home
			}
			return filepath.Join(home, rest)
		}
	}
	return filepath.Clean(path)
}

const pluginSource = `import { appendFile, mkdir, readFile, rename, stat } from "node:fs/promises"
import os from "node:os"
import path from "node:path"

const DEBUG_LOG_MAX_BYTES = 512 * 1024
const WATCHED_EVENT_TYPES = new Set([
  "message.updated",
  "session.error",
  "session.next.retried",
  "session.next.step.failed",
  "session.status",
])

const QUOTA_TERMS = [
  "quota",
  "exhausted",
  "exceeded",
  "usage limit",
  "rate limit",
  "rate_limit_exceeded",
  "insufficient_quota",
  "billing",
  "credits",
]

const STRONG_QUOTA_PHRASES = [
  "usage limit has been reached",
  "usage limit reached",
  "quota exhausted",
  "quota exceeded",
  "insufficient_quota",
]

function statePath() {
  if (process.env.CQ_OPENCODE_HOOK_STATE) return process.env.CQ_OPENCODE_HOOK_STATE
  if (process.env.CQ_CONFIG_HOME) return path.join(process.env.CQ_CONFIG_HOME, "codex-quota", "opencode-hook.json")
  if (process.env.XDG_CONFIG_HOME) return path.join(process.env.XDG_CONFIG_HOME, "codex-quota", "opencode-hook.json")
  const home = os.homedir()
  if (process.platform === "darwin") return path.join(home, "Library", "Application Support", "codex-quota", "opencode-hook.json")
  if (process.platform === "win32") return path.join(process.env.APPDATA || path.join(home, "AppData", "Roaming"), "codex-quota", "opencode-hook.json")
  return path.join(home, ".config", "codex-quota", "opencode-hook.json")
}

function debugPath() {
  if (process.env.CQ_OPENCODE_DEBUG_LOG) return process.env.CQ_OPENCODE_DEBUG_LOG
  return path.join(path.dirname(statePath()), "opencode-plugin-debug.log")
}

async function debugLog(type, details = {}) {
  try {
    const file = debugPath()
    await mkdir(path.dirname(file), { recursive: true })
    try {
      const info = await stat(file)
      if (info.size > DEBUG_LOG_MAX_BYTES) await rename(file, file + ".1")
    } catch {
      // Missing or concurrently rotated log files are fine.
    }
    await appendFile(file, JSON.stringify({ time: new Date().toISOString(), type, ...details }) + "\n")
  } catch {
    // Diagnostics must never affect OpenCode.
  }
}

function shouldLogEvent(event) {
  return event && typeof event === "object" && WATCHED_EVENT_TYPES.has(event.type)
}

function textOf(value) {
  if (value == null) return ""
  if (typeof value === "string") return value
  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

function errorSummary(error) {
  if (!error || typeof error !== "object") return { message: textOf(error) }
  const data = error.data && typeof error.data === "object" ? error.data : {}
  return {
    name: error.name || "",
    type: error.type || "",
    status_code: Number(data.statusCode || error.statusCode || 0),
    message: textOf(data.message || error.message || ""),
    response_body: textOf(data.responseBody || error.responseBody || ""),
  }
}

function eventSummary(event) {
  if (!event || typeof event !== "object") return { event_type: typeof event }
  const props = event.properties || {}
  const summary = {
    event_type: event.type || "",
    property_keys: props && typeof props === "object" ? Object.keys(props) : [],
  }
  if (props.sessionID) summary.session_id = props.sessionID
  if (props.status) summary.status = props.status
  if (props.error) summary.error = errorSummary(props.error)
  if (props.info) {
    summary.info = {
      role: props.info.role || "",
      session_id: props.info.sessionID || props.sessionID || "",
      provider_id: props.info.providerID || "",
      model_id: props.info.modelID || "",
      has_error: !!props.info.error,
      error: props.info.error ? errorSummary(props.info.error) : undefined,
    }
  }
  return summary
}

function eventSignal(event) {
  if (!event || typeof event !== "object") return null
  const type = event.type
  const props = event.properties || {}
  if (type === "message.updated") {
    const info = props.info || {}
    if (info.role !== "assistant" || !info.error) return null
    return {
      source: "opencode",
      event_type: type,
      session_id: info.sessionID || "",
      provider_id: info.providerID || "",
      model_id: info.modelID || "",
      ...errorFields(info.error),
    }
  }
  if (type === "session.error" && props.error) {
    return {
      source: "opencode",
      event_type: type,
      session_id: props.sessionID || "",
      provider_id: "",
      model_id: "",
      ...errorFields(props.error),
    }
  }
  if (type === "session.next.retried" && props.error) {
    return {
      source: "opencode",
      event_type: type,
      session_id: props.sessionID || "",
      provider_id: "",
      model_id: "",
      ...errorFields(props.error),
    }
  }
  if (type === "session.next.step.failed" && props.error) {
    return {
      source: "opencode",
      event_type: type,
      session_id: props.sessionID || "",
      provider_id: "",
      model_id: "",
      ...errorFields(props.error),
    }
  }
  if (type === "session.status" && props.status?.type === "retry") {
    return {
      source: "opencode",
      event_type: type,
      session_id: props.sessionID || "",
      provider_id: props.status.action?.provider || "",
      model_id: "",
      error_name: "",
      status_code: 0,
      message: textOf(props.status.message || ""),
      response_body: textOf(props.status.action || ""),
    }
  }
  return null
}

function errorFields(error) {
  const data = error && typeof error === "object" && error.data ? error.data : {}
  return {
    error_name: error?.name || error?.type || "",
    status_code: Number(data.statusCode || error?.statusCode || 0),
    message: textOf(data.message || error?.message || ""),
    response_body: textOf(data.responseBody || error?.responseBody || ""),
  }
}

function isQuotaSignal(signal) {
  if (!signal) return false
  const provider = (signal.provider_id || "").toLowerCase()
  if (provider && provider !== "openai" && provider !== "opencode") return false
  const haystack = (signal.message + "\n" + signal.response_body).toLowerCase()
  if (signal.status_code === 429) return QUOTA_TERMS.some((term) => haystack.includes(term))
  if (signal.event_type === "session.next.retried") return STRONG_QUOTA_PHRASES.some((term) => haystack.includes(term))
  if (signal.event_type === "session.next.step.failed") return STRONG_QUOTA_PHRASES.some((term) => haystack.includes(term))
  if (signal.event_type === "session.status") return STRONG_QUOTA_PHRASES.some((term) => haystack.includes(term))
  return haystack.includes("exhausted") && haystack.includes("quota")
}

async function notify(signal) {
  const raw = await readFile(statePath(), "utf8")
  const state = JSON.parse(raw)
  if (!state.url || !state.token) return { ok: false, status: 0, reason: "missing listener state" }
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 500)
  try {
    const target = new URL("/v1/opencode/quota-signal", state.url)
    const response = await fetch(target, {
      method: "POST",
      headers: {
        "authorization": "Bearer " + state.token,
        "content-type": "application/json",
      },
      body: JSON.stringify({ ...signal, received_at: new Date().toISOString() }),
      signal: controller.signal,
    })
    return { ok: response.ok, status: response.status, url: target.toString() }
  } finally {
    clearTimeout(timeout)
  }
}

export const CodexQuotaPlugin = async () => {
  await debugLog("plugin.initialized", { state_path: statePath(), debug_path: debugPath() })
  return {
    event: async ({ event }) => {
      if (shouldLogEvent(event)) await debugLog("event.received", eventSummary(event))
      const signal = eventSignal(event)
      const quota = isQuotaSignal(signal)
      if (signal) await debugLog("signal.classified", { quota, signal })
      if (!quota) return
      try {
        const result = await notify(signal)
        await debugLog("notify.finished", { result, signal })
      } catch (error) {
        await debugLog("notify.failed", { error: textOf(error?.message || error), signal })
        // cq may not be running; never block or fail opencode because of the hook.
      }
    },
  }
}
`

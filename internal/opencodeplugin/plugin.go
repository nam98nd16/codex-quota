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

const pluginSource = `import { readFile } from "node:fs/promises"
import os from "node:os"
import path from "node:path"

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

function statePath() {
  if (process.env.CQ_OPENCODE_HOOK_STATE) return process.env.CQ_OPENCODE_HOOK_STATE
  if (process.env.CQ_CONFIG_HOME) return path.join(process.env.CQ_CONFIG_HOME, "codex-quota", "opencode-hook.json")
  if (process.env.XDG_CONFIG_HOME) return path.join(process.env.XDG_CONFIG_HOME, "codex-quota", "opencode-hook.json")
  const home = os.homedir()
  if (process.platform === "darwin") return path.join(home, "Library", "Application Support", "codex-quota", "opencode-hook.json")
  if (process.platform === "win32") return path.join(process.env.APPDATA || path.join(home, "AppData", "Roaming"), "codex-quota", "opencode-hook.json")
  return path.join(home, ".config", "codex-quota", "opencode-hook.json")
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
  return null
}

function errorFields(error) {
  const data = error.data || {}
  return {
    error_name: error.name || "",
    status_code: Number(data.statusCode || 0),
    message: textOf(data.message || error.message || ""),
    response_body: textOf(data.responseBody || ""),
  }
}

function isQuotaSignal(signal) {
  if (!signal) return false
  const provider = (signal.provider_id || "").toLowerCase()
  if (provider && provider !== "openai" && provider !== "opencode") return false
  const haystack = (signal.message + "\n" + signal.response_body).toLowerCase()
  if (signal.status_code === 429) return QUOTA_TERMS.some((term) => haystack.includes(term))
  return haystack.includes("exhausted") && haystack.includes("quota")
}

async function notify(signal) {
  const raw = await readFile(statePath(), "utf8")
  const state = JSON.parse(raw)
  if (!state.url || !state.token) return
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 500)
  try {
    await fetch(new URL("/v1/opencode/quota-signal", state.url), {
      method: "POST",
      headers: {
        "authorization": "Bearer " + state.token,
        "content-type": "application/json",
      },
      body: JSON.stringify({ ...signal, received_at: new Date().toISOString() }),
      signal: controller.signal,
    })
  } finally {
    clearTimeout(timeout)
  }
}

export const CodexQuotaPlugin = async () => {
  return {
    event: async ({ event }) => {
      const signal = eventSignal(event)
      if (!isQuotaSignal(signal)) return
      try {
        await notify(signal)
      } catch {
        // cq may not be running; never block or fail opencode because of the hook.
      }
    },
  }
}
`

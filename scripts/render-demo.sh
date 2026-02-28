#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
real_home="${HOME}"
tmp_home="$(mktemp -d -t cq-demo-home-XXXXXX)"

cleanup() {
	if [[ -n "${server_pid:-}" ]]; then
		kill "${server_pid}" >/dev/null 2>&1 || true
		wait "${server_pid}" >/dev/null 2>&1 || true
	fi
	rm -rf "${tmp_home}" || true
}
trap cleanup EXIT

config_dir="${tmp_home}/Library/Application Support/codex-quota"
opencode_dir="${tmp_home}/opencode"
mkdir -p "${config_dir}" "${opencode_dir}" "${tmp_home}/.codex"
cp "${repo_root}/scripts/demo-accounts.json" "${config_dir}/accounts.json"
cat >"${config_dir}/ui_state.json" <<'EOF'
{
  "compact_mode": true,
  "exhausted_account_keys": [
    "acct_demo_free_gamma",
    "acct_demo_free_delta",
    "acct_demo_free_zeta",
    "acct_demo_free_theta"
  ]
}
EOF

cat >"${opencode_dir}/auth.json" <<'EOF'
{
  "openai": {
    "access": "demo_access_token_free_active",
    "refresh": "demo_refresh_token_free_active",
    "accountId": "acct_demo_free_active",
    "email": "demo+active@example.test",
    "expires": 4102444800000
  }
}
EOF

cat >"${tmp_home}/.codex/auth.json" <<'EOF'
{
  "tokens": {
    "access_token": "demo_access_token_free_active",
    "refresh_token": "demo_refresh_token_free_active",
    "account_id": "acct_demo_free_active"
  },
  "last_refresh": "2026-02-28T00:00:00Z"
}
EOF

server_port="${CQ_DEMO_PORT:-18080}"
server_delay_ms="${CQ_DEMO_DELAY_MS:-200}"
python3 "${repo_root}/scripts/mock-usage-server.py" --host "127.0.0.1" --port "${server_port}" --delay-ms "${server_delay_ms}" >/dev/null 2>&1 &
server_pid="$!"

export HOME="${tmp_home}"
export OPENCODE_AUTH_PATH="${opencode_dir}/auth.json"
export OPENCODE_DATA_DIR="${opencode_dir}"
export CODEX_AUTH_PATH="${tmp_home}/.codex/auth.json"
export CQ_USAGE_URL="http://127.0.0.1:${server_port}/backend-api/wham/usage"
export PATH="${real_home}/.local/bin:${real_home}/go/bin:${PATH}"

cd "${repo_root}"
vhs demo.tape

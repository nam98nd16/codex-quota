# CQ (Codex Quota Monitor)

A TUI for monitoring Codex quota written in Go using [Bubble Tea](https://github.com/charmbracelet/bubbletea).

![Demo](demo.gif)

## Features

- Multiple Codex accounts
- Accounts from local app storage, OpenCode, and Codex auth files
- OAuth authentication via browser
- Apply active account to OpenCode or Codex auth
- Two view modes: compact for many accounts, tabs for focused viewing when you have just a few.

## Installation

```bash
go install github.com/deLiseLINO/codex-quota/cmd/cq@latest
```

**Note:** Make sure your Go bin directory is available in `PATH`.

Build from source:

```bash
git clone https://github.com/deLiseLINO/codex-quota.git
cd codex-quota
go install ./cmd/cq
```

## Usage

Run the app:

```bash
cq
```

On first launch press `n` to add an account via OAuth.

## Controls

- `r` — refresh active account
- `R` — refresh all accounts
- `i` — toggle additional info
- `n` — add account (OAuth)
- `Enter` / `o` — apply active account to codex or opencode
- `x` — delete active account (with source selection)
- `v` — switch view mode (tabs/compact)
- `↑` `↓` `←` `→` (or `h` `j` `k` `l`) — switch active account
- `q` / `Ctrl+C` — quit
- `Esc` — close modal/info/error/notice (or quit if nothing is open)

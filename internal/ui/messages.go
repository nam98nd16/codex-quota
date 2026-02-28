package ui

import (
	"time"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

type DataMsg struct {
	AccountKey      string
	Data            api.UsageData
	Account         *config.Account
	ReloadAccounts  bool
	ReloadActiveKey string
}

type ErrMsg struct {
	AccountKey string
	Err        error
}

type AccountsMsg struct {
	ActiveKey               string
	Accounts                []*config.Account
	Notice                  string
	SourcesByAccountID      map[string][]string
	ActiveSourcesByIdentity map[string][]string
}

type NoticeMsg struct {
	Text string
}

type NoticeTimeoutMsg struct {
	Seq int
}

type AnimationFrameMsg struct {
	Now time.Time
}

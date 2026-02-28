## v0.1.0

### Overview
- Added compact mode for multi-account workflows.
- Added grouped `Exhausted accounts` section in compact view.
- Added multi-target apply flow (`codex` and/or `opencode`).
- Added multi-source delete flow (remove from selected sources, not only app storage).
- Added smooth progress-bar animations for loading and account switching.
- Added sticky exhausted state persisted across restarts.
- Account list now shows active source badges (`Codex`/`OpenCode`) and highlights subscription accounts.
- Improved account sync across app/codex/opencode sources.
- Improved loading UX and visual stability (fewer layout jumps/artifacts).
- Updated on-screen help and keybindings to match current behavior.

### Install
```bash
# this release
go install github.com/deLiseLINO/codex-quota/cmd/cq@v0.1.0

# latest release
go install github.com/deLiseLINO/codex-quota/cmd/cq@latest
```

**Note:** Make sure your Go bin directory is available in `PATH`.

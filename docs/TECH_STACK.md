# Tech stack

| Area | Choice |
|------|--------|
| Language | Go 1.27.3, module `hirectl` |
| TUI | `charm.land/bubbletea/v2` v2.1.1 (`make get PKG=...` adds deps) |
| TUI widgets / styling | `charm.land/bubbles/v2` v2.2.1 (textinput), `charm.land/lipgloss/v2` v2.0.6, `github.com/charmbracelet/x/ansi` v0.11.9 (truncation) |
| Cron validation | `github.com/robfig/cron/v3` v3.0.1 (`ParseStandard`) |
| Storage | JSON files `feeds.json`, `sources.json` in `$HIRECTL_DIR`, else `os.UserConfigDir()/hirectl` |
| Tests | Go `testing` (`make test`) |
| Dev environment | Docker Compose service `go`, image target `dev` (`golang:1.27.3`), run as host user |
| Build | `make build`: static binary `bin/hirectl` (`CGO_ENABLED=0`) |
| Production image | Multi-stage `Dockerfile`, final image `gcr.io/distroless/static-debian12:nonroot` |
| Task runner | `Makefile`; `make help` lists targets |

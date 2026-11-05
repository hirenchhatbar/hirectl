# Tech stack

| Area | Choice |
|------|--------|
| Language | Go 1.27.3, module `hirectl` |
| TUI | `charm.land/bubbletea/v2` v2.1.1 (`make get PKG=...` adds deps) |
| TUI widgets / styling | `charm.land/bubbles/v2` v2.2.1 (textinput), `charm.land/lipgloss/v2` v2.0.6, `github.com/charmbracelet/x/ansi` v0.11.9 (truncation) |
| Job sites | Integrations over `net/http`: Greenhouse Job Board API, Lever Postings API |
| Board discovery | Common Crawl URL index (`cluster.idx` plus ranged `cdx-*.gz` reads on `data.commoncrawl.org`), or given boards |
| Scheduling | System cron calls `hirectl run` and, less often, `hirectl discover` |
| Storage | JSON files `feeds.json`, `jobs.json`, `suggestions.json`, `config.json` (settings) in `$HIRECTL_DIR`, else `os.UserConfigDir()/hirectl` |
| Tests | Go `testing` (`make test`) |
| Dev environment | Docker Compose service `go`, image target `dev` (`golang:1.27.3`), run as host user |
| Build | `make build`: static binary `bin/hirectl` (`CGO_ENABLED=0`) |
| Production image | Multi-stage `Dockerfile`, final image `gcr.io/distroless/static-debian12:nonroot` |
| Task runner | `Makefile`; `make help` lists targets |

# Tech stack

| Area | Choice |
|------|--------|
| Language | Go 1.27.3, module `hirectl` |
| TUI | `charm.land/bubbletea/v2` v2.1.1 (`make get PKG=...` adds deps) |
| TUI widgets / styling | `charm.land/bubbles/v2` v2.2.1 (textinput, spinner), `charm.land/lipgloss/v2` v2.0.6, `github.com/charmbracelet/x/ansi` v0.11.9 (truncation) |
| Job sites | Integrations over `net/http`: Greenhouse Job Board API, Lever Postings API (global and EU), Ashby Job Posting API, Workable job widget, SmartRecruiters Posting API, Rippling job board, Recruitee Careers Site API, Personio XML feed (`encoding/xml`), Gem job board API, Breezy HR job list, Workday career site JSON API (`/wday/cxs/`), hirist.tech site JSON API (`gladiator.hirist.tech`, undocumented) |
| Board discovery | Common Crawl URL index (`cluster.idx` plus ranged `cdx-*.gz` reads on `data.commoncrawl.org`), board pages by path (`<host>/<company>`) or subdomain (`<company>.<domain>`); hirist categories from its sitemap; or given boards |
| Scheduling | System cron calls `hirectl run` and, less often, `hirectl discover` |
| Storage | JSON files `feeds.json`, `jobs/<feed-id>.json` (one per feed), `suggestions.json`, `discover.json` (boards an interrupted discover has left), `config.json` (settings), `jobviews.json` (jobs pane search, filters and sort per feed), `likes.json` (👍/👎 by job URL), `hirectl.lock` (`flock`, held while discover saves) in `$HIRECTL_DIR`, else `os.UserConfigDir()/hirectl` |
| Tests | Go `testing` (`make test`) |
| Dev environment | Docker Compose service `go`, image target `dev` (`golang:1.27.3`), run as host user |
| Build | `make build`: static binary `bin/hirectl` (`CGO_ENABLED=0`) |
| Production image | Multi-stage `Dockerfile`, final image `gcr.io/distroless/static-debian12:nonroot` |
| Task runner | `Makefile`; `make help` lists targets |

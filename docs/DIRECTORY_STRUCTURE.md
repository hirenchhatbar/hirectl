# Directory structure

```
.
├── cmd/hirectl/         # the app: one `main` package, tests beside each file
│   ├── main.go          # entry point: TUI, or `hirectl run` / `hirectl discover` for cron, `hirectl list`, `hirectl cross-check`; holds the `version` const used by `make bump`
│   ├── app.go           # root TUI: home is Feeds (sidebar), function-key bar, switches modules (F2 Feeds, F9 Config, F10 Quit), F5 and timed refresh, shows Feeds' sidebar left of every other module, sets the theme's screen colors
│   ├── store.go         # data dir, generic JSON load/save, data dir lock (hirectl.lock)
│   ├── theme.go         # themes (palettes), styles built from the applied one; dracula default
│   ├── config_ui.go     # Config type (config.json) and the F9 config module: theme, sidebar width, default locations for new feeds, boards per feed, auto-refresh seconds
│   ├── feed.go          # Feed type (sources, no boards), its list kind, validation, job matching and score (skills, locations)
│   ├── list_ui.go       # generic list module: sidebar list, right pane for logo/view/form/delete, form checklist (feed sources, with matching boards), derived fields (feed ID from title, live and ctrl+g)
│   ├── jobs_ui.go       # feed's jobs pane (→ or o in Feeds): fuzzy search, sources checklist, posted, location, score and level filters, sorting (saved per feed in jobviews.json), open in browser
│   ├── run.go           # `hirectl run --feeds --sources --delay`: cross-check, pick each feed's top boards from suggestions.json, fetch them paced per integration, keep matching jobs per feed with score and level in jobs/<feed-id>.json, print those with new URLs, prune files of removed feeds
│   ├── list.go          # `hirectl list feeds|jobs|boards|sources`, `--since`/`--sort` for jobs: print stored data; job and board line format shared with run and discover
│   ├── discover.go      # `hirectl discover --boards --sources --delay`: boards from Common Crawl (by path or subdomain) or given, jobs into suggestions.json, saved every 30s with boards left in discover.json to resume; boards matching a feed, top boards, source suggestions
│   ├── crosscheck.go    # `hirectl cross-check --sources`: per source, boards in suggestions.json vs in the latest Common Crawl
│   ├── progress.go      # output of run and discover: results on stdout, board errors at once and a progress bar (terminal only) on stderr
│   ├── level.go         # job level: years of experience asked (from description, else title words), grouped 0-2, 3-4, 5-7, 8+
│   ├── tech.go          # technology names discover tags jobs with, top techs per board
│   ├── integration.go   # Job type (with posting date), integration registry, request pacer, shared HTTP and location helpers
│   ├── greenhouse.go    # Greenhouse job-board integration (one file per job site, registers itself)
│   ├── lever.go         # Lever postings integrations: `lever` and `lever-eu`
│   ├── ashby.go         # Ashby job-board integration
│   ├── workable.go      # Workable job-widget integration
│   ├── smartrecruiters.go # SmartRecruiters postings integration (paged)
│   ├── rippling.go      # Rippling job-board integration
│   ├── recruitee.go     # Recruitee offers integration
│   ├── personio.go      # Personio XML-feed integration (.de and .com boards)
│   ├── gem.go           # Gem job-board integration
│   ├── breezy.go        # Breezy HR job-list integration
│   └── workday.go       # Workday career-site integration (paged list, then each job)
├── docs/                # project documentation: DIRECTORY_STRUCTURE.md, TECH_STACK.md, CLI.md (commands and options)
├── .claude/             # Claude Code: settings.json (allow `make`, deny host `go`), rules/ imported by CLAUDE.md (code quality, make/docker, docs, TUI)
├── Makefile             # every command runs through a target; `make help` lists them
├── Dockerfile           # dev and production images
├── compose.yaml         # dockerised dev environment
├── go.mod, go.sum       # Go module `hirectl` and dependency checksums
├── feeds.json, jobs/<feed-id>.json, suggestions.json, discover.json, config.json, jobviews.json, hirectl.lock # dev data via HIRECTL_DIR in compose.yaml (git-ignored)
└── bin/                 # build output (git-ignored)
```

# Directory structure

```
.
├── cmd/hirectl/         # the app: one `main` package, tests beside each file
│   ├── main.go          # entry point: TUI, or `hirectl run` / `hirectl discover` for cron, `hirectl list`; holds the `version` const used by `make bump`
│   ├── app.go           # root TUI: home logo, function-key bar, switches modules (F2 Feeds, F9 Config, F10 Quit), sets the theme's screen colors
│   ├── store.go         # data dir, generic JSON load/save
│   ├── theme.go         # themes (palettes), styles built from the applied one; dracula default
│   ├── config_ui.go     # Config type (config.json) and the F9 config module: theme
│   ├── feed.go          # Feed type, its list kind, validation, job matching
│   ├── list_ui.go       # generic list module: sidebar list, right pane for logo/view/form/delete, form checklist (feed board suggestions), derived fields (feed ID from title, live and ctrl+g)
│   ├── run.go           # `hirectl run --feeds --sources --delay`: cross-check, fetch boards paced per integration, store new jobs in jobs.json
│   ├── list.go          # `hirectl list feeds|jobs|boards|sources`: print stored data; job and board line format shared with run and discover
│   ├── discover.go      # `hirectl discover --boards --sources --delay`: boards from Common Crawl or given, jobs into suggestions.json; feed suggestions
│   ├── tech.go          # technology names discover tags jobs with, top techs per board
│   ├── integration.go   # Job type, integration registry, request pacer, shared HTTP helpers
│   ├── greenhouse.go    # Greenhouse job-board integration (one file per job site, registers itself)
│   └── lever.go         # Lever postings integration
├── docs/                # project documentation: DIRECTORY_STRUCTURE.md, TECH_STACK.md, CLI.md (commands and options)
├── .claude/             # Claude Code: settings.json (allow `make`, deny host `go`), rules/ imported by CLAUDE.md
├── Makefile             # every command runs through a target; `make help` lists them
├── Dockerfile           # dev and production images
├── compose.yaml         # dockerised dev environment
├── go.mod, go.sum       # Go module `hirectl` and dependency checksums
├── feeds.json, jobs.json, suggestions.json, config.json # dev data via HIRECTL_DIR in compose.yaml (git-ignored)
└── bin/                 # build output (git-ignored)
```

# Directory structure

```
.
├── cmd/hirectl/         # the app: one `main` package, tests beside each file
│   ├── main.go          # entry point; holds the `version` const used by `make bump`
│   ├── app.go           # root TUI: home logo, function-key bar, switches modules (F2 Sources, F3 Feeds, F10 Quit)
│   ├── store.go         # data dir, generic JSON list load/save
│   ├── feed.go          # Feed type, its list kind, validation
│   ├── source.go        # Source type (job website URL), its list kind, validation
│   └── list_ui.go       # generic list module: sidebar list, right pane for logo/view/form/delete
├── docs/                # project documentation
├── .claude/             # Claude Code: settings.json (allow `make`, deny host `go`), rules/ imported by CLAUDE.md
├── Makefile             # every command runs through a target; `make help` lists them
├── Dockerfile           # dev and production images
├── compose.yaml         # dockerised dev environment
├── go.mod, go.sum       # Go module `hirectl` and dependency checksums
├── feeds.json, sources.json # dev data via HIRECTL_DIR in compose.yaml (git-ignored)
└── bin/                 # build output (git-ignored)
```

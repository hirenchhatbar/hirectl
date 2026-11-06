# hirectl

Terminal job-feed tracker. Define feeds (skills, locations, sources), discover job boards via Common Crawl, fetch matching jobs from Greenhouse, Lever, Ashby, Workable, SmartRecruiters, Rippling, Recruitee, Personio, Gem, Breezy HR and Workday, and browse them in a TUI with search, filters, score and level.

## Usage

```
hirectl                                                    # TUI: feeds, jobs, config
hirectl discover [--boards a,b] [--sources s,t] [--delay 1s]  # find boards, store their jobs in suggestions.json
hirectl run      [--feeds a,b]  [--sources s,t] [--delay 1s]  # fetch each feed's top boards, store matching jobs
hirectl list     <feeds|jobs|boards|sources> [--since 7d] [--sort posted]
hirectl cross-check [--sources s,t]                        # stored boards vs latest Common Crawl
```

Typical flow: add feeds in the TUI, run `hirectl discover` (rarely) and `hirectl run` (often) from cron, read jobs in the TUI or with `hirectl list jobs`.

Data lives in `$HIRECTL_DIR`, else `~/.config/hirectl`. Full reference: [docs/CLI.md](docs/CLI.md).

## Development

Only Docker and `make` needed on the host; every command runs in the dev container.

```
make setup   # build dev image
make build   # bin/hirectl
make run     # ARGS="list jobs" etc.
make test
make fmt vet
make image   # production image hirectl:latest
make bump    # PART=patch|minor|major, or V=x.y.z
make help    # all targets
```

See [docs/DIRECTORY_STRUCTURE.md](docs/DIRECTORY_STRUCTURE.md) and [docs/TECH_STACK.md](docs/TECH_STACK.md).

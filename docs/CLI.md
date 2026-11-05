# CLI

```
hirectl                                                         # interactive TUI
hirectl --help                                                  # also -h, -help, help: list commands
hirectl --version                                               # also -v, -version, version: print `hirectl <version>`
hirectl run      [--feeds a,b]  [--sources s,t] [--delay 1s]
hirectl discover [--boards a,b] [--sources s,t] [--delay 1s]
hirectl list     <feeds|jobs|boards|sources>
```

- Data dir: `$HIRECTL_DIR`, else `os.UserConfigDir()/hirectl` (e.g. `~/.config/hirectl`).
- List options take comma-separated values; blanks around commas are trimmed.
- Integrations (`--sources` values): `greenhouse`, `lever`.
- Identifiers, used in options, files and output:
  - Feed: its ID, a unique slug such as `go-remote` (lowercase letters, digits, dashes). In the TUI feed form, a new feed's ID fills in from the title as you type (`Go remote (EU)` becomes `go-remote-eu`) until you type over it; editing a feed keeps its ID. Ctrl+G sets the ID from the title, on create and edit. Left blank, it is made from the title.
  - Source: the integration name, `greenhouse` or `lever`.
  - Board: `<source>:<name>`, e.g. `greenhouse:acme`.
- Exit status 1 with `hirectl: <error>` on stderr. `run` and `discover` keep going past a failing board, save what they found, then report every board error.
- `--help` (or `-h`, `-help`) after `run`, `discover` or `list` prints its options. Help exits 0.
- Any other first argument is an error: `unknown command "<arg>", see hirectl --help`.
- Ctrl+C cancels `run` and `discover`.

## `hirectl`

No arguments: opens the TUI.

| Key | Action |
|-----|--------|
| F2 | Feeds: list, view, add, edit, delete feeds (`feeds.json`). A feed whose ID is already taken is not saved. In the form, Ctrl+G sets the ID from the title. The feed form offers boards from `suggestions.json` as a checklist: Tab from Location opens it, listing every discovered board, those with most jobs matching Skills (in title or tagged technologies, any spelling such as Golang for Go) and Location first, with matching and total jobs, main technologies, and sample matching roles of the selected board. j/k moves, Space checks or unchecks. On a feed with no boards, the top 10 boards with matches start checked. Tab again goes to the Boards field, to type boards by hand. |
| F9 | Config: ←/→ (or h/l, enter, space) cycles the theme, saved to `config.json` and applied at once. Themes: `catppuccin-latte`, `catppuccin-mocha`, `dracula` (default), `gruvbox-dark`, `gruvbox-light`, `nord`, `solarized-dark`, `solarized-light`, `terminal` (terminal's own colors), `tokyonight-day`, `tokyonight-night`. An unknown theme in `config.json` falls back to dracula, with an error on this page. |
| F10 | Quit |

## `hirectl run`

Fetches each enabled feed's boards, keeps jobs matching the feed, appends jobs not seen before for that feed to `jobs.json`, and prints each one as a tab-separated line: `feed-id  board  title  location  url`. A board shared by several feeds is fetched once. Meant for cron.

| Option | Default | Description |
|--------|---------|-------------|
| `--feeds` | all enabled feeds | Feed IDs, e.g. `--feeds go-remote,backend-nl`. Unknown or disabled feed, or a named feed with no board on the selected sources, is an error. |
| `--sources` | all integrations | Only fetch boards of these integrations, e.g. `--sources greenhouse` or `--sources greenhouse,lever`. Unknown integration is an error. |
| `--delay` | `1s` | Minimum wait between requests to the same integration (Go duration: `500ms`, `2s`, ...). |

No positional arguments.

Examples:

```sh
hirectl run                                               # all enabled feeds, all sources
hirectl run --feeds go-remote --sources greenhouse        # one feed, Greenhouse boards only
hirectl run --feeds go-remote,backend-nl --delay 2s       # two feeds, slower pace
```

## `hirectl discover`

Probes boards and stores their jobs (title, location, and the technologies named in title or description, from a fixed list in `tech.go`) in `suggestions.json`, for the feed form to suggest boards from. Descriptions are read but not stored. Prints each board with jobs as `board  <n> jobs  <top techs>`. A probed board replaces its stored jobs; a board that no longer exists is dropped; a board that fails otherwise keeps its stored jobs. Integrations are probed in parallel. Meant for cron, less often than `run`.

| Option | Default | Description |
|--------|---------|-------------|
| `--boards` | boards in the latest Common Crawl, plus those already in `suggestions.json` | Boards as `<source>:<name>`, or a bare name tried on every selected source. Names are lowercased with spaces removed. `--boards "greenhouse:stripe,Acme Corp"` tries `greenhouse:stripe`, `greenhouse:acmecorp` and `lever:acmecorp`. Unknown source or blank name is an error. The default takes long since requests are paced. |
| `--sources` | all integrations | Integrations to probe, e.g. `--sources lever` or `--sources greenhouse,lever`. Unknown integration is an error. |
| `--delay` | `1s` | Minimum wait between requests to the same integration. |

No positional arguments.

Examples:

```sh
hirectl discover --boards stripe                          # greenhouse:stripe, lever:stripe
hirectl discover --boards greenhouse:stripe,lever:acme    # these two boards only
hirectl discover --sources lever --delay 2s               # Common Crawl boards on Lever, plus stored ones, slower pace
```

## `hirectl list`

Prints stored data, one tab-separated line per item. Reads only; makes no requests.

| Argument | Reads | Prints |
|----------|-------|--------|
| `feeds` | `feeds.json` | `feed-id  enabled\|disabled  title  boards` (boards comma-separated) |
| `jobs` | `jobs.json` | `feed-id  board  title  location  url`, as `run` prints |
| `boards` | `suggestions.json` | `board  <n> jobs  <top techs>` (up to 4, comma-separated), as `discover` prints, sorted by board |
| `sources` | none | each integration name |

Exactly one argument; none, another, or more is an error: `list: want one of boards, feeds, jobs, sources`.

Examples:

```sh
hirectl list jobs | grep go-remote    # stored jobs of one feed
hirectl list boards                   # boards discover found, with job counts
```

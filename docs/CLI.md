# CLI

```
hirectl                                                         # interactive TUI
hirectl --help                                                  # also -h, -help, help: list commands
hirectl --version                                               # also -v, -version, version: print `hirectl <version>`
hirectl run      [--feeds a,b]  [--sources s,t] [--delay 1s]
hirectl discover [--boards a,b] [--sources s,t] [--delay 1s]
hirectl list     <feeds|jobs|boards|sources> [--since 7d] [--sort posted]
hirectl cross-check [--sources s,t]
```

- Data dir: `$HIRECTL_DIR`, else `os.UserConfigDir()/hirectl` (e.g. `~/.config/hirectl`).
- List options take comma-separated values; blanks around commas are trimmed.
- Integrations (`--sources` values): `ashby`, `breezy` (Breezy HR), `gem`, `greenhouse`, `hirist` (hirist.tech, India tech jobs), `lever`, `lever-eu` (Lever EU instance), `personio` (boards on `jobs.personio.de` and `jobs.personio.com`), `recruitee`, `rippling`, `smartrecruiters`, `workable`, `workday`. `workday` reads a list page per 20 jobs, then each job for its locations and description, so a board of n jobs takes about n requests, paced only between boards. `hirist` is a job site, not a company's board: its boards are its job categories, named by the slug of the category page `https://www.hirist.tech/c/<slug>-jobs` (`hirist:backend-development`), listed from its category sitemap. It reads the category page, then a list page per 50 jobs (paced only between boards), through the undocumented API the site's pages use, so it may break when the site changes. Its list has no description: a job's years of experience asked and skill tags stand in for it, for level, score and technologies.
- Identifiers, used in options, files and output:
  - Feed: its ID, a unique slug such as `go-remote` (lowercase letters, digits, dashes). In the TUI feed form, a new feed's ID fills in from the title as you type (`Go remote (EU)` becomes `go-remote-eu`) until you type over it; editing a feed keeps its ID. Ctrl+G sets the ID from the title, on create and edit. Left blank, it is made from the title.
  - Source: the integration name, e.g. `greenhouse` or `lever`.
  - Board: `<source>:<name>`, e.g. `greenhouse:acme`; on `hirist` a category, e.g. `hirist:devops-sre`. A Workday board's name is `<tenant>.<wdN>/<site>`, from its page `https://<tenant>.<wdN>.myworkdayjobs.com/<site>`: `workday:nvidia.wd5/nvidiaexternalcareersite`.
- Feed matching: a job matches a feed when any of its Locations and any of its Skills fit; no Locations or no Skills match every job. A skill fits under any spelling (`Golang` for Go) as a word not followed by `-` (`Go-to-Market` is not Go): in the title in any case, in the description case-sensitive for a technology from `tech.go` (so "go above and beyond" is not Go) and in any case for another skill, or among the technologies `discover` tagged. A location fits when each of its words is a word of the job's location, in any order, ignoring case and punctuation: `Remote India` fits `Bangalore, India - Remote`; `India` fits `Bangalore, Karnataka, India` but not `Indianapolis, IN`; `Remote` fits `Remote US` too. Locations are typed comma-separated, so one location holds no comma; `Remote Worldwide` fits `Remote, Worldwide`. A city-only job location such as `Mumbai` needs the city listed.
- Score: how much a job is about its feed's Skills, summed over them: 10 for a skill in the title, plus 1 per time it is in the description (up to 5), else 1 for it among the tagged technologies. `Go Engineer` writing "We write Go." scores 11; a job naming Go once in its description scores 1. No Skills scores 0.
- Level (YOE): the years of experience a job asks, grouped as `0-2`, `3-4`, `5-7`, `8+`: from the first years its description asks before "experience" in the same sentence (`5+ years of experience`, `3-5 years' professional experience`; the fewest years count), else the highest level a title word names (`0-2` Intern, Internship, Junior, Jr, Graduate, Entry, Trainee, Apprentice; `3-4` Mid, Medior, Intermediate; `5-7` Senior, Sr; `8+` Staff, Principal, Lead, Architect, Head, Director, Distinguished), else unknown. Years spelled in words are missed.
- Exit status 1 with `hirectl: <error>` on stderr. `run` and `discover` keep going past a failing board: they print each board error on stderr as it happens (`hirectl: <board>: <error>`), save what they found, then fail with `<n> of <m> boards failed`.
- `run` and `discover` print results on stdout as they come. When stderr is a terminal, a progress bar on stderr's last line shows boards done, boards total and the board being fetched (`discover` first shows its Common Crawl index read, then a bar over the index blocks, unless every selected source lists its own boards, as `hirist` does). Off a terminal, as under cron or `2>file`, no bar is drawn.
- `--help` (or `-h`, `-help`) after `run`, `discover`, `list` or `cross-check` prints its options. Help exits 0.
- Any other first argument is an error: `unknown command "<arg>", see hirectl --help`.
- Ctrl+C (or SIGTERM, SIGHUP) cancels `run` and `discover`; both save what they found.

## `hirectl`

No arguments: opens the TUI on its home screen, Feeds (F2): the feeds sidebar, logo on the right. Esc goes back one level everywhere: it cancels a form or delete, closes a feed's jobs, and on a module's main screen returns to Feeds. F10 quits from anywhere; q quits from anywhere but a text input, where it types `q`. F5 (or a click on it in the bar) refreshes from anywhere: rereads `feeds.json`, job counts, `suggestions.json` (for the Sources checklist) and the open jobs list, keeping its search, filters, sort and the selected feed and job, so data `run` or `discover` wrote since shows; it skips while a form or delete prompt is open, or while typing or picking in a jobs list, so no input is lost. Refresh in F9 repeats it every so many seconds.

| Key | Action |
|-----|--------|
| F2 | Feeds: list, view, add, edit, delete feeds (`feeds.json`). The sidebar shows each feed in three rows: its title with its stored job count at the right (`jobs/<feed-id>.json`, read at start, on save, on refresh and on opening its jobs), then its Skills and its Locations, each in brackets (`[Go, Rust]`, `[Remote India]`; `[any skill]`, `[any location]` when blank), cut to the sidebar width. A feed whose ID is already taken is not saved. A new feed starts with the default Locations from F9. In the form, Ctrl+G sets the ID from the title. A feed has no boards: `run` picks them (see `hirectl run`). The feed form offers the integrations as a checklist for Sources: Tab from Locations opens it, listing each integration with its boards and jobs in `suggestions.json` matching Skills (in title or tagged technologies, any spelling such as Golang for Go) and Locations, and the top boards of the selected one. j/k moves, Space checks or unchecks; Enter (or Ctrl+S) saves the feed from any field, the checklist included. None checked means all. Tab again goes to the Sources field, to type them by hand. Esc cancels the form. On a feed, → (or l, o) lists its jobs; Enter shows the feed's fields. Jobs (`jobs/<feed-id>.json`, as `run` stored them) list in the right pane, newest first, undated last: date, score, level, thumb (👍 or 👎, blank when unrated), title, company (the board's name, a Workday board's tenant), board, location, and the selected job's URL below. / types a fuzzy search over title, board and location (each word's letters in order, ignoring case; best match first), l types Locations to keep (comma-separated, matched as feed Locations), s opens the sources among its jobs as a checklist, with job counts: j/k moves, Space checks or unchecks, Enter keeps, Esc undoes; only checked sources show, none checked shows all. d cycles the posted age (any, 1d, 7d, 30d; undated jobs stay), r the least score (any, 3+, 10+; 10+ keeps jobs with a skill in the title), y the level (any, 0-2, 3-4, 5-7, 8+; jobs of unknown level stay), t the thumb (any, 👍, 👎, none for unrated), S the sort (newest, board, title, score highest first, level fewest years first and unknown last), c clears search, Locations, sources, posted age, score, level, thumb and sort back to their defaults. While typing, Enter keeps the text and Esc undoes it; Esc in the checklist or while typing does not close the list. j/k (or the wheel) moves, Enter opens the job in the default browser (`xdg-open`, `open` on macOS). + gives the selected job 👍, - gives it 👎; the same key again clears it. Thumbs are saved at once in `likes.json` by job URL, so a job keeps its thumb in every feed and across `run`s; a job whose site changes its URL loses it. Space opens the job's snapshot, a box over the list: title, company, board, location, posted date, score, level, thumb, URL and its description as plain text (HTML tags dropped), fetched from the site on opening, with a spinner meanwhile. The fetch reads the job's whole board (one request; a Workday job is read alone), so the other jobs on that board open at once afterwards, until the TUI quits; `breezy`, `smartrecruiters` and `rippling` give no description, and a job gone from its board says so. In the snapshot j/k scrolls, + and - rate, Enter opens the browser, Space, Esc or ← (or h) closes it. F5 skips while it is open. Esc closes the list to the feed's fields; ← (or h) goes back to the sidebar. Search, Locations, checked sources, posted age, score, level, thumb filter and sort are saved per feed in `jobviews.json` as they change (typed text and checks on Enter), and apply when that feed's list opens again; a checked source no longer among its jobs drops out. |
| F5 | Refresh: reread data files, see above |
| F9 | Config, right of the Feeds sidebar: ↑/↓ (or Tab) moves between Theme, Sidebar, Locations, Boards and Refresh. On Theme, ←/→ (or h/l, enter, space) cycles the theme, saved to `config.json` and applied at once. Themes: `catppuccin-latte`, `catppuccin-mocha`, `dracula` (default), `gruvbox-dark`, `gruvbox-light`, `nord`, `solarized-dark`, `solarized-light`, `terminal` (terminal's own colors), `tokyonight-day`, `tokyonight-night`. An unknown theme in `config.json` falls back to dracula, with an error on this page. On Sidebar, ←/→ (or h/l, enter, space), and + or - on Theme or Sidebar, narrows or widens the Feeds sidebar by 2 cells (16 to 80, default 30, border included), saved to `config.json` and applied at once. On Locations, type comma-separated default locations for new feeds; Enter saves them to `config.json`. On Boards, type how many boards `run` fetches per feed (default 10, at least 1); Enter saves. On Refresh, type the seconds between automatic F5 refreshes (default 0, off); Enter saves and restarts the timer. Esc drops unsaved Locations, Boards and Refresh and returns to Feeds. |
| F10 | Quit |

## `hirectl run`

Picks each enabled feed's boards from `suggestions.json`: the boards on the feed's Sources (all when none) with most jobs matching the feed, up to Boards in F9 (`config.json`, default 10). So the picks follow the last `discover`; with no `suggestions.json` yet, `run` fails with `no discovered boards, run hirectl discover first`. Fetches those boards and keeps the feed's matching jobs in `jobs/<feed-id>.json`: the file is replaced by the jobs now matching on the picked boards, so a job gone from its board, no longer matching an edited feed, or on a board no longer picked drops out; jobs of a board that failed, or (with `--sources`) of a source not selected, stay. Stores each job's posting date (`posted`), as the site gives it: first published on `greenhouse` and `gem`, created on `lever` and `personio`, published on `ashby`, `breezy`, `recruitee`, `smartrecruiters` and `workable`, the posting's start date on `workday`, created on `hirist`; `rippling` gives none, so its jobs are undated. Stores each job's score (`score`) and level (`level`), from its title and description (title only on `breezy`, `smartrecruiters` and `rippling`); jobs stored before keep none until their board is fetched again. Prints each job whose URL was not in the file as a tab-separated line: `feed-id  board  title  location  url  posted` (posted as `2026-11-06`, blank when undated), so a board picked again does not print its jobs twice. Then deletes the files of feeds no longer in `feeds.json` (a renamed feed starts afresh, its jobs printed again on its next run), and anything else in `jobs/`; disabled feeds keep theirs. A board picked by several feeds is fetched once. Meant for cron.

| Option | Default | Description |
|--------|---------|-------------|
| `--feeds` | all enabled feeds | Feed IDs, e.g. `--feeds go-remote,backend-nl`. Unknown or disabled feed, or a named feed with no board with matching jobs on the selected sources, is an error. |
| `--sources` | all integrations | Only pick boards of these integrations, on top of each feed's Sources, e.g. `--sources greenhouse` or `--sources greenhouse,lever`. Unknown integration is an error. |
| `--delay` | `1s` | Minimum wait between requests to the same integration (Go duration: `500ms`, `2s`, ...). |

No positional arguments.

Examples:

```sh
hirectl run                                               # all enabled feeds, all sources
hirectl run --feeds go-remote --sources greenhouse        # one feed, its top Greenhouse boards only
hirectl run --feeds go-remote,backend-nl --delay 2s       # two feeds, slower pace
```

## `hirectl discover`

Probes boards and stores their jobs (title, location, and the technologies named in title or description, from a fixed list in `tech.go`; title only on `breezy`, `smartrecruiters` and `rippling`, whose lists carry no description) in `suggestions.json`, for `run` to pick each feed's boards from. Descriptions are read but not stored. Prints each board with jobs as `board  <n> jobs  <top techs>` as soon as it is probed, so in no set order. A probed board replaces its stored jobs; a board that no longer exists is dropped; a board that fails otherwise keeps its stored jobs. Integrations are probed in parallel. Meant for cron, less often than `run`.

Progress is saved every 30s: probed boards are merged into `suggestions.json` as it is then, so discovers run at once (say with other `--sources`) keep each other's boards; each save holds `hirectl.lock` in the data dir, and a discover finding it held waits for it. The boards left are kept in `discover.json`, per `--sources` and `--boards`. A discover cancelled, killed or failing midway leaves them there; the next one with the same `--sources` and `--boards` resumes, probing only those (failed boards included), skips the Common Crawl read, and notes `hirectl: resuming the interrupted discover, <n> boards left` on stderr. A discover that probes all its boards clears its entry, so the next one starts afresh. A killed one re-probes up to 30s of boards.

| Option | Default | Description |
|--------|---------|-------------|
| `--boards` | boards in the latest Common Crawl (on `hirist`, its categories), plus those already in `suggestions.json` | Boards as `<source>:<name>`, or a bare name tried on every selected source. Names are lowercased with spaces removed. A bare name is never a Workday board, so on `workday` it finds nothing. `--boards "greenhouse:stripe,Acme Corp"` tries `greenhouse:stripe`, and `acmecorp` on every selected source (`ashby:acmecorp`, `greenhouse:acmecorp`, ...). Unknown source or blank name is an error. The default takes long since requests are paced. |
| `--sources` | all integrations | Integrations to probe, e.g. `--sources lever` or `--sources greenhouse,lever`. Unknown integration is an error. |
| `--delay` | `1s` | Minimum wait between requests to the same integration. |

No positional arguments.

Examples:

```sh
hirectl discover --boards stripe                          # stripe on every source: ashby:stripe, greenhouse:stripe, ...
hirectl discover --boards greenhouse:stripe,lever:acme    # these two boards only
hirectl discover --sources lever --delay 2s               # Common Crawl boards on Lever, plus stored ones, slower pace
```

## `hirectl list`

Prints stored data, one tab-separated line per item. Reads only; makes no requests.

| Argument | Reads | Prints |
|----------|-------|--------|
| `feeds` | `feeds.json` | `feed-id  enabled\|disabled  title  sources` (sources comma-separated, blank for all) |
| `jobs` | `jobs/*.json` | `feed-id  board  title  location  url  posted`, as `run` prints, the jobs each feed holds, by feed and board |
| `boards` | `suggestions.json` | `board  <n> jobs  <top techs>` (up to 4, comma-separated), as `discover` prints, sorted by board |
| `sources` | `suggestions.json` | `source  <n> boards`, each integration and how many boards `discover` stored for it |

Exactly one argument; none, another, or more is an error: `list: want one of boards, feeds, jobs, sources`. Options go before or after it, and apply to `jobs` only; with another argument they are an error.

| Option | Default | Description |
|--------|---------|-------------|
| `--since` | all jobs | Drop jobs posted longer ago: whole days (`7d`) or a Go duration (`12h`). Undated jobs stay, as their age is unknown. |
| `--sort` | `board` | `board`: by feed and board. `posted`: newest first across feeds, undated last. |

Examples:

```sh
hirectl list jobs | grep go-remote    # stored jobs of one feed
hirectl list jobs --since 7d --sort posted  # jobs posted this week, newest first
hirectl list boards                   # boards discover found, with job counts
hirectl list sources                  # integrations, with board counts
```

## `hirectl cross-check`

Compares, per source, the boards stored in `suggestions.json` with the boards online: in the latest Common Crawl, read as `discover` reads it (with its progress bar on a terminal), or on `hirist` its categories. Prints one tab-separated line per source, in name order: `source  <n> stored  <m> online  <k> not stored`, where `not stored` counts the online boards missing from `suggestions.json`, which a full `discover` would probe. Then, after a blank line, the command to run for each source with boards not stored, one per line, most boards not stored first (ties by name): `hirectl discover --sources <source>`; none when all are stored. Stored boards are those `discover` found jobs on, so a board online with no jobs stays `not stored`. Writes nothing.

| Option | Default | Description |
|--------|---------|-------------|
| `--sources` | all integrations | Integrations to check, e.g. `--sources lever` or `--sources greenhouse,lever`. Unknown integration is an error. |

No positional arguments.

Examples:

```sh
hirectl cross-check                       # every source
hirectl cross-check --sources greenhouse  # Greenhouse only
```

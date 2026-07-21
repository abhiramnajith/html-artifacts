# Design: Lightweight multi-agent distribution

Date: 2026-07-22
Status: Approved (brainstorm) — pending implementation plan

## Problem

Two limitations surfaced after v1:

1. **Size.** Mermaid (3.4 MB) is inlined into every diagram artifact and copied
   into every agent's skill directory by `install.sh`. Supporting more agents
   multiplies the on-disk weight, and diagram artifacts are huge.
2. **Install friction.** The tool is installed by the user manually running
   `install.sh`. There is no harness-native ("installed by the agent itself")
   path, and nothing is released for download.

## Goals

- Adding another agent adds ~0 heavy bytes.
- Diagram artifacts shrink from ~3.4 MB to ~20 KB.
- The viewer runs without the user babysitting it, on a port that will not
  collide with other local dev servers.
- Claude Code installs it natively via `/plugin`; every other agent installs via
  one agent-agnostic script. The server binary arrives automatically.

## Added post-review

- **`render` subcommand** — `html-artifacts render <path.md>` converts an
  existing Markdown file (plan, spec, README, doc) into an artifact in the store
  and prints its `/view/<id>` URL. Uses a small stdlib-only Markdown renderer and
  the embedded `base.html`. Distinct from authoring new rich artifacts (§2–3 of
  CORE.md): this makes *existing* Markdown viewable/annotatable. Motivated by the
  gap that the tool could only author new HTML, never render an existing `.md`.

## Non-goals (deferred)

- **`export` command** (produce a standalone, Mermaid-inlined HTML file for
  sharing off-machine). Deferred — the auto-started server covers normal viewing;
  add only if hand-off of standalone files becomes a real need.
- Adapters for agents other than Claude Code (structure supports them; none
  built here).
- Auth / multi-user / non-localhost hosting (unchanged from v1).

## Decisions locked in brainstorming

- Mermaid moves **into the server binary**; injected at view-time like the editor
  shell. It leaves `instructions/` and all skill dirs.
- **Default port `47600`** (quiet registered range), overridable with `--port`;
  auto-start walks to the next free port if taken.
- **Global artifacts store** `~/.html-artifacts/artifacts/` is the new default
  `--dir`, so one always-on server serves everything regardless of cwd.
  `--dir` still overrides for anyone who wants project-local artifacts.
- Server **auto-starts on demand** (no service to install); the same routine
  lazily bootstraps the binary.
- Distribution: **Claude Code plugin** + **agent-agnostic `install.sh`**; binary
  delivered via **GitHub Release** downloads (first `v0.1.0`).

---

## Part A — View-time asset injection

### Changes

- **Move** `instructions/templates/vendor/mermaid.min.js` → `server/embed/` and
  add it to the existing `//go:embed` set. Binary grows ~3.4 MB (one-time; single
  self-contained binary). Remove the `vendor/` dir from `instructions/`.
- **`base.html`**: remove the inlined-Mermaid block and the runtime marker. Keep
  the `.mermaid` divs and the small `mermaid.initialize(...)` init script. The
  template stops being responsible for the runtime. On-disk artifacts become
  ~20 KB.
- **Server**:
  - New route `GET /_vendor/mermaid.min.js` serving the embedded file
    (`text/javascript`), outside the artifacts namespace like `/_editor/`.
  - `/view/{id}`: after reading the artifact, if the HTML contains a Mermaid
    block (`class="mermaid"`), inject `<script src="/_vendor/mermaid.min.js">`
    before the existing init script, in addition to the editor shell. No
    injection when there are no diagrams.
- **`CORE.md`**: delete the "inline the vendored mermaid runtime" step (§4). The
  agent now just writes `.mermaid` divs; the server supplies the runtime. Note
  that diagrams render through the local viewer (which the skill auto-starts),
  not from a bare `file://` open.

### Consequence

Mermaid exists in exactly one place (the binary). Skill/adapter dirs carry no
heavy assets, so N agents ≈ N tiny text files.

---

## Part B — On-demand bootstrap + auto-start

### `scripts/ensure-server.sh`

One POSIX shell script, installed beside the skill and bundled in the plugin.
`CORE.md` calls it before opening an artifact; it prints the base URL on stdout
(e.g. `http://127.0.0.1:47600`). Logic:

1. **Already running?** If `~/.html-artifacts/port` exists and the server answers
   at that port (`curl -sf .../artifacts`) → print URL, exit 0.
2. **Ensure the binary**, first hit wins:
   - `html-artifacts` on `$PATH`
   - `~/.html-artifacts/bin/html-artifacts`
   - **download** the prebuilt release binary for this OS/arch from the latest
     GitHub Release into `~/.html-artifacts/bin/`, `chmod +x`
   - `go install github.com/abhiramnajith/html-artifacts/server@latest` fallback
     (binary lands as `server`)
   - else: exit non-zero with a clear message (how to install Go / download).
3. **Pick a free port** starting at `47600`, incrementing until one binds.
4. **Start** `<binary> serve --port <port> --dir "$HA_DIR"` in the background
   (`nohup … &`, detached), where `HA_DIR` defaults to
   `~/.html-artifacts/artifacts` (override via `HTML_ARTIFACTS_DIR`).
5. **Record** the port in `~/.html-artifacts/port`; poll until the server
   answers; print `http://127.0.0.1:<port>`.

### State & storage

- `~/.html-artifacts/` holds `bin/`, `artifacts/`, and `port`.
- Global artifacts dir is the default; the server, `List`, annotations, and the
  apply-loop all operate there. `main.go`'s `--dir` default changes from
  `./artifacts` to `~/.html-artifacts/artifacts` (created if missing).

### `CORE.md` open step (§5)

Replace the manual "curl to check / open file" block with: run
`ensure-server.sh`, capture the URL, open `<url>/view/<id>` cross-platform.
Fallback to a `file://` open only if the script fails (diagrams won't render
there — acceptable, rare).

---

## Part C — Distribution

### 1. Release `v0.1.0`

Tag `v0.1.0`; existing `ci.yml` builds and attaches linux/darwin × amd64/arm64
binaries to the GitHub Release. This is the download source for the bootstrap and
the answer to "is it published".

### 2. Claude Code plugin

- `.claude-plugin/marketplace.json` — marketplace listing this one plugin.
- `.claude-plugin/plugin.json` — plugin metadata (name, version, description).
- `skills/html-artifacts/` — the shipped skill: `SKILL.md`, `CORE.md`,
  `ensure-server.sh`. No Mermaid (now in the binary), so the plugin is light.
- Install UX: `/plugin marketplace add abhiramnajith/html-artifacts` then
  `/plugin install html-artifacts`. Binary is fetched lazily by `ensure-server.sh`
  on first use (plugins can't run arbitrary install steps — consistent with the
  on-demand model).
- Exact manifest field names to be confirmed against current Claude Code plugin
  docs during implementation.

### 3. `install.sh` (other agents)

- Keep it, now lighter: place the chosen adapter + `CORE.md` + `ensure-server.sh`
  into the agent's skills dir. No `templates/vendor` copy.
- Optional `--with-binary` flag to eagerly download the release binary instead of
  waiting for first-use bootstrap.

### 4. Single source of truth

`instructions/CORE.md` stays canonical. A `make sync` target copies it (and
`ensure-server.sh`) into `skills/html-artifacts/` and each `adapters/*/` so there
is no hand-maintained duplication (upholds the "no duplicated instructions"
invariant). `make sync` runs as part of the release step.

---

## Affected files (summary)

| File | Change |
|------|--------|
| `server/embed/mermaid.min.js` | new (moved from instructions), `go:embed`'d |
| `instructions/templates/vendor/` | removed |
| `instructions/templates/base.html` | drop inlined Mermaid; keep `.mermaid` + init |
| `server/internal/server/server.go` | `/_vendor/mermaid.min.js` route; inject Mermaid in `/view` when diagrams present |
| `server/main.go` | default `--dir` → `~/.html-artifacts/artifacts`; default `--port` → `47600` |
| `scripts/ensure-server.sh` | new bootstrap + auto-start script |
| `instructions/CORE.md` | remove inline-Mermaid step; new open step via ensure-server |
| `.claude-plugin/marketplace.json`, `.claude-plugin/plugin.json` | new plugin manifest |
| `skills/html-artifacts/` | new: plugin-shipped skill (synced from canonical) |
| `install.sh` | drop templates copy; add `ensure-server.sh`; optional `--with-binary` |
| `Makefile` | `sync` target; default port/dir updates |
| `README.md`, `docs/PLAN.md` | document new install/usage, port, storage |

## Testing

- **Go (automated):** `/view` injects `/_vendor/mermaid.min.js` only when a
  `.mermaid` block is present (httptest); `/_vendor/mermaid.min.js` served with
  JS content-type; default `--dir`/`--port` values; existing storage/traversal
  tests unchanged. Free-port selection helper unit-tested if extracted into Go.
- **Shell:** `ensure-server.sh` — idempotent (second call reuses the running
  server), picks a free port when the default is busy, writes the state file.
  Tested with a throwaway `HOME`/state dir.
- **End-to-end (browser):** produce a diagram artifact (~20 KB on disk),
  auto-start via the script, confirm the diagram renders at the localhost URL,
  and the annotate→send→apply loop still works against the global store.
- **Plugin:** manual — add the marketplace and install in a fresh Claude Code
  session; confirm the skill triggers and bootstraps the binary.

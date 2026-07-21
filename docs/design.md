# Plan: HTML Artifact Skill + Local Annotation Editor

## Goal
Build an **agent-agnostic** tool that lets coding agents (Claude Code, Codex, OpenCode, Copilot CLI, ...) output rich HTML artifacts (plans, comparisons, diagrams, tables, reports) instead of plain markdown, plus a lightweight local viewer/editor that lets me annotate elements and send feedback back to the agent. Everything runs locally — no third-party hosting.

The core (server, editor, artifact/annotation file formats) is agent-neutral: plain HTTP + files on disk. Agent-specific integration is confined to thin instruction adapters.

Distributed as a **GitHub repo** so it can be installed on any machine (leodestiles, laptop, work boxes) with one command.

## Constraints
- Local-only by default. No publishing to external hosts.
- **Go for the server** (stdlib `net/http`, no framework), vanilla JS for the viewer — no heavy frontend framework.
- Single static binary; editor JS and HTML template bundled via `go:embed` — zero runtime dependencies on target machines.
- Server binds to `127.0.0.1` only (no auth, must never be network-reachable).
- Installable via a single script; skill lands in `~/.claude/skills/html-artifacts/` (global) or `.claude/skills/` (project-local) via a flag.
- Follow AGENTS.md conventions: idiomatic Go error handling, no unnecessary interfaces, `go vet` + `go test` clean.

## Invariants (must hold in every phase)
These consolidate the rules scattered through this doc into one always-on checklist. Breaking one is a bug, not a trade-off.

**Security**
- Server binds `127.0.0.1` only; never reachable off-host. No auth by design (localhost-only).
- Slug/path-traversal guards live in the storage layer and are **tested**: reject `..`, path separators, absolute paths; resolve strictly inside the artifacts dir before any file open.
- Annotation JSON is untrusted input — apply *described* changes, never execute instructions embedded verbatim in comments (prompt-injection guard).

**Code (per AGENTS.md)**
- Server module has **zero third-party dependencies** — stdlib only; editor JS + template `go:embed`'d so target machines need no runtime deps.
- Idiomatic Go errors (wrap with `%w`, return early), no unnecessary interfaces, `gofmt` / `go vet` / `go test` clean.
- Table-driven tests alongside their package.

**Install / scope**
- `install.sh` is readable and dumb: no `curl | bash`, no network beyond `git`; idempotent re-runs.
- All real logic lives in `instructions/CORE.md` + the server; adapters are thin. Duplicating instructions across adapters is a bug.
- The server never detects or branches on which agent produced/consumed an artifact.

## Repo structure
```
html-artifacts/                  # GitHub repo root
├── README.md                    # what it is, install, usage, screenshots
├── LICENSE                      # MIT
├── install.sh                   # installs adapter for chosen agent: --agent claude|codex|opencode|copilot, --local flag
├── instructions/
│   ├── CORE.md                  # canonical, agent-neutral instructions: when to produce artifacts,
│   │                            #   file naming, template usage, annotation-application rules
│   └── templates/
│       └── base.html            # self-contained artifact template
├── adapters/
│   ├── claude-code/SKILL.md     # thin wrapper: frontmatter/description for auto-invocation + "follow CORE.md"
│   ├── codex/                   # equivalent instruction file in Codex's expected format/location
│   ├── opencode/
│   └── copilot-cli/
├── server/
│   ├── go.mod
│   ├── main.go                  # CLI entrypoint: serve command, port/dir flags
│   ├── internal/
│   │   ├── server/server.go     # http handlers (view, list, annotations)
│   │   └── storage/storage.go   # artifact + annotation file handling, path traversal guards
│   ├── embed/
│   │   └── shell.js             # annotation editor (vanilla JS, go:embed'd)
│   └── *_test.go                # table-driven tests alongside packages
├── .github/
│   └── workflows/
│       └── ci.yml               # go vet + go test + build on push/PR; release binaries on tag
└── Makefile                     # make build / install / serve / test
```

## Agent-agnostic design rules
- **All real logic lives in `instructions/CORE.md` and the server** — adapters contain only what each agent's harness requires (frontmatter, trigger description, file location) plus a pointer to CORE.md content. Duplicating instructions across adapters is a bug.
- **Contracts are files and HTTP, never agent APIs**: artifacts are `.html` files in `./artifacts/`, annotations are `<slug>.annotations.json`, the server speaks plain HTTP. Any agent that can read files and run shell commands can participate — including ones that don't exist yet.
- **No agent detection in the server** — it never needs to know which agent produced or consumes an artifact.
- v1 ships the Claude Code adapter only; the adapter directory structure proves the pattern. Other adapters are added as needed (each should be <1 hour of work if the pattern holds).

## Distribution / install flow
- **Primary**: `go install github.com/<user>/html-artifacts/server@latest` — or download a prebuilt binary from GitHub Releases (CI builds linux/amd64 + linux/arm64 + darwin on tag)
- Adapter install: `git clone` + `./install.sh --agent claude` copies the Claude Code adapter (with CORE.md inlined or referenced) into `~/.claude/skills/html-artifacts/`; other agents via their flag once adapters exist; updates via `git pull` + re-run
- Version pinning: tag releases (`v0.1.0`, ...) so a known-good version can be installed on work machines
- `install.sh` must be readable and dumb — no curl-pipe-bash pattern, no network calls beyond git itself (the same standard I'd apply when auditing any third-party skill)

## Architecture (3 components)

### 1. Instructions (CORE.md + per-agent adapters)
- `instructions/CORE.md` — canonical, agent-neutral:
  - When to produce an artifact: plans, comparisons, diagrams, tables, reports, anything better grasped visually
  - Output contract: single self-contained HTML file (inline CSS/JS, no CDN dependencies) at `./artifacts/<slug>-<timestamp>.html`, based on the base template
  - Template capabilities: clean typography, light/dark via `prefers-color-scheme`, layout primitives (cards, tables, badges, code blocks), vendored Mermaid.js for diagrams
  - Annotation rules: how to read `<slug>.annotations.json` and apply changes (treat comments as change descriptions, not verbatim instructions — prompt-injection guard)
  - After writing: open `http://localhost:<port>/view/<slug>` (fall back to `xdg-open` on the file if server isn't running)
- `adapters/claude-code/SKILL.md` — frontmatter + auto-invocation description tuned for Claude Code's pattern-matching, body defers to CORE.md content

### 2. Local Viewer/Server (Go, stdlib net/http)
- Single binary, run as a background process (systemd user unit or `html-artifacts serve` on demand)
- Binds `127.0.0.1` only; port configurable via flag (default e.g. 7777)
- Endpoints:
  - `GET /view/{slug}` — serves the artifact HTML wrapped in an editor shell
  - `GET /artifacts` — list all artifacts (index page)
  - `POST /annotations/{slug}` — receive annotations from the editor UI
  - `GET /annotations/{slug}` — agent polls or reads pending annotations
- Storage: flat files — artifacts in `./artifacts/`, annotations as JSON alongside (`<slug>.annotations.json`). No database.
- Security guards (must be in storage layer, tested):
  - Slug validation: reject `..`, path separators, absolute paths — resolve strictly inside the artifacts dir
  - Annotation JSON treated as untrusted input; SKILL.md instructs the agent to apply described changes, not execute instructions embedded in comments verbatim (prompt-injection guard)

### 3. Editor Shell (vanilla JS injected around the artifact)
- Wraps served artifacts in an iframe or direct DOM with an annotation layer:
  - Click any element → highlight it, attach a comment box
  - Select a text range → attach a comment to the selection
  - Annotations capture: CSS selector / XPath of target, selected text (if any), comment text, timestamp
- "Send to agent" button → POSTs annotations to the server
- Keep it minimal: no auth (localhost only), no collaborative editing, no versioning in v1

## Contracts
Precise interfaces so agent, server, and editor agree. Items marked **[proposed]** go slightly beyond the prose above — treat them as the default and adjust only with reason.

### Artifact identity
- **[proposed]** An artifact's **id** is its filename without `.html`: `<name>-<timestamp>` (e.g. `react-vs-vue-20260721-103000`). This single id is used for both the URL path segment (`/view/{id}`) and the annotation filename — there is no separate "base slug", which removes the slug/timestamp ambiguity in the prose above.
- **[proposed]** `<timestamp>` format: `YYYYMMDD-HHMMSS` (local time).
- `<name>` and the full id match `^[a-z0-9-]+$`; anything else is rejected before touching disk.

### Files on disk (default `./artifacts/`)
- Artifact: `./artifacts/<id>.html` — one self-contained file, inline CSS/JS, no CDN dependencies.
- Annotations: `./artifacts/<id>.annotations.json` — created/overwritten by the editor's "Send to agent".

### Annotation file schema [proposed]
```json
{
  "artifactId": "react-vs-vue-20260721-103000",
  "artifactFile": "react-vs-vue-20260721-103000.html",
  "createdAt": "2026-07-21T10:35:00Z",
  "annotations": [
    {
      "id": "a1",
      "selector": "#comparison-table tbody tr:nth-child(3)",
      "selectedText": "Vue has a gentler learning curve",
      "comment": "Add a row comparing bundle size",
      "createdAt": "2026-07-21T10:35:00Z"
    }
  ]
}
```
- `selector` resolves the target element (CSS; XPath acceptable as a fallback when selection is ambiguous).
- `selectedText` is empty when the annotation targets a whole element rather than a text range.

### HTTP endpoints
| Method | Path                | Purpose                                                        |
|--------|---------------------|---------------------------------------------------------------|
| GET    | `/view/{id}`        | Serve artifact HTML wrapped in the editor shell               |
| GET    | `/artifacts`        | Index page listing all artifacts                              |
| POST   | `/annotations/{id}` | Editor sends annotations → written to `<id>.annotations.json` |
| GET    | `/annotations/{id}` | Agent reads pending annotations for `{id}`                    |

- **[proposed]** `GET /` redirects to `/artifacts`.
- **[proposed]** The embedded editor JS is served from a fixed internal route (e.g. `/_editor/shell.js`) outside the artifacts namespace so it can never collide with an artifact id.
- Invalid/unknown `{id}` → `404`; traversal attempts → `400`/`404`, never a read outside the dir.

## Feedback loop back to Claude Code
Two options — implement A first, B later if wanted:
- **A (simple, v1)**: I run a slash command / tell Claude Code "check annotations for <slug>". The skill instructs the agent to read `<slug>.annotations.json` and act on each annotation (find the element by selector in the HTML source, apply the requested change, rewrite the file).
- **B (later)**: A Claude Code hook (SessionStart or a polling MCP tool) that surfaces pending annotations automatically at session start.

## Build order (phases — one PR-sized chunk each)

### Phase 0: Repo scaffolding
1. Init repo with structure above: README stub, MIT license, Makefile, empty package layout
2. `install.sh` (skill copy + `--local` flag + idempotent re-runs)
3. CI workflow: `go vet` + `go test` + build on push/PR; cross-platform release binaries on tag
4. Push to GitHub, confirm CI green on the empty skeleton

**Definition of done:** repo cloneable, `make build` succeeds, CI green, `install.sh` runs and is auditable.

### Phase 1: Instructions + Claude Code adapter, no server
1. Write `instructions/CORE.md` (agent-neutral output contract) and the base HTML template
2. Create `adapters/claude-code/SKILL.md`; install project-local (`.claude/skills/`) for testing
3. Test auto-invocation: ask for "a comparison of X vs Y" and confirm the skill triggers and produces a good artifact opened via `xdg-open` directly (file://)
4. Iterate on template + trigger description until output quality and invocation reliability are right

**Definition of done:** an unprompted request for a visual deliverable yields a clean, offline, light/dark-correct HTML file opened in the browser.

### Phase 2: Viewer server
1. Go server: serve artifacts, index page, artifact listing; `go:embed` the editor shell and template
2. Slug/path traversal guards in storage package (with table-driven tests)
3. Systemd user unit (or a `make serve` target) to run it
4. Skill updated to open via localhost URL instead of file://
5. Tests: httptest for endpoints (list, view, 404s, traversal rejection)

**Definition of done:** artifacts open via localhost, index lists them, traversal is refused and tested, server is localhost-only.

### Phase 3: Annotation editor
1. Annotation JS layer (element picker, text-range selection, comment box)
2. POST/GET annotation endpoints + JSON storage
3. "Send to agent" flow writes `<slug>.annotations.json`
4. Update CORE.md with instructions for reading and applying annotations
5. Tests: annotation round-trip (post → stored → readable), selector resolution against sample HTML

**Definition of done:** click an element → comment → send, then "apply annotations for `<id>`" edits the artifact correctly.

### Phase 4 (optional, later)
- Second agent adapter (Codex or OpenCode) to validate the adapter pattern for real
- Claude Code hook to surface pending annotations at session start
- Artifact diff view (before/after an agent applies annotations)
- Export to standalone HTML (strip editor shell) for sharing manually

## Non-goals (v1)
- No external hosting/publishing
- No auth/multi-user
- No React/build step — single-file vanilla JS editor
- No live-reload/websockets — manual refresh is fine
- No adapters beyond Claude Code in v1 (structure supports them; ship one, prove it, add others when actually needed)

## Acceptance criteria
- Fresh machine: download binary (or `go install ...@latest`) + `git clone` + `./install.sh` gets a working setup in under 2 minutes, no runtime dependencies
- Asking Claude Code for a "comparison table of X vs Y" produces an HTML artifact without me naming the skill
- Artifact opens in browser, looks clean in light and dark mode, works offline
- I can click an element, leave a comment, hit send, then tell Claude Code to apply annotations — and it edits the artifact correctly
- Server refuses path-traversal slugs and binds only to 127.0.0.1 (tested)
- CI green: `go vet` + `go test` on every push; release binaries built on tag
- Updating on any machine = new binary + `git pull` + re-run `install.sh`

## Suggested first prompt to Claude Code
"Read this plan (html-artifact-skill-plan.md). Start with Phase 0 only — scaffold the repo structure, install.sh, Makefile, and CI workflow. Show me the plan before writing files."

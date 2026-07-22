# vellum

An **agent-agnostic** tool that lets coding agents (Claude Code, Codex, OpenCode,
Copilot CLI, …) emit rich, self-contained **HTML artifacts** — plans, comparisons,
diagrams, tables, reports — instead of plain markdown, plus a **local** viewer/editor
to annotate elements and feed changes back to the agent.

Everything runs on `127.0.0.1`. No external hosting, no auth, no runtime dependencies.

> **Status: v1 complete**, now with a lightweight distribution path (Claude Code
> plugin, agent-agnostic installer, a global artifact store, and a
> lazily-fetched server binary — no `go install` or manual build required to get
> started). Build notes live in [`docs/PLAN.md`](docs/PLAN.md) and
> [`docs/design.md`](docs/design.md).

## Supported agents

Any agent that can read files and run shell commands can drive vellum. Four are
supported out of the box (see [Install](#install) for details):

| Agent | Install | Instructions land in |
|-------|---------|----------------------|
| **Claude Code** | `/plugin install vellum` or `./install.sh --agent claude` | auto-invoked skill → `~/.claude/skills/vellum/` |
| **Codex** | `./install.sh --agent codex` | `~/.codex/AGENTS.md` |
| **OpenCode** | `./install.sh --agent opencode` | `~/.config/opencode/AGENTS.md` |
| **Copilot CLI** | `./install.sh --agent copilot` | `~/.copilot/copilot-instructions.md` |

All four defer to one canonical [`instructions/CORE.md`](instructions/CORE.md) —
no per-agent behavior drift. Adding another agent is a few lines; see
[`adapters/README.md`](adapters/README.md).

> The Codex, OpenCode, and Copilot CLI adapters are new — their install and
> idempotency are tested, but live auto-invocation on those agents isn't yet
> confirmed. Reports welcome.

## How it works

The contract between agent and tool is **files + HTTP**, never agent APIs:

- **Instructions** — `instructions/CORE.md` (canonical, agent-neutral) plus thin
  per-agent adapters under `adapters/`. v1 ships the Claude Code adapter only.
- **Local server** — a single Go binary (stdlib `net/http`, zero third-party deps)
  that serves artifacts and an editor shell, and stores annotations as files.
- **Editor shell** — vanilla JS injected around the served artifact: click/select →
  comment → "Send to agent".

Any agent that can read files and run shell commands can participate.

## Install

There are two ways to install, depending on your agent.

### Claude Code — via the plugin marketplace

```
/plugin marketplace add abhiramnajith/vellum
/plugin install vellum
```

This installs the skill (instructions, template, and the `ensure-server.sh`
bootstrap script) directly from this repo — no separate clone step needed.
Restart your session (or reload the extension) afterwards so Claude Code picks
up the new skill.

### Any other agent (or Claude Code without the plugin) — via `install.sh`

```sh
git clone https://github.com/abhiramnajith/vellum
cd vellum
./install.sh --agent claude     # Claude Code   -> ~/.claude/skills/vellum/
./install.sh --agent codex      # Codex         -> ~/.codex/AGENTS.md
./install.sh --agent opencode   # OpenCode      -> ~/.config/opencode/AGENTS.md
./install.sh --agent copilot    # Copilot CLI   -> ~/.copilot/copilot-instructions.md
```

For **Claude Code**, the adapter is an auto-invoked skill directory (add
`--local` to install into `./.claude/skills/` for the current project instead).

For **Codex, OpenCode, and Copilot CLI**, which read an always-on `AGENTS.md`-style
instruction file, `install.sh` installs the shared runtime once to `~/.vellum/`
and inserts a managed `<!-- vellum:begin -->…<!-- vellum:end -->` block into that
agent's instruction file. Re-running replaces the block in place and **never
touches other instructions** in the file. (`CODEX_HOME` / `COPILOT_HOME` are
honored if set.)

`install.sh` makes no network calls beyond `git`, and re-running it is idempotent.
The `vellum` binary is fetched lazily on first use — pass `--with-binary` to fetch
it eagerly during install.

## Using it

Either install path gives Claude Code the same skill. The flow is the same
everywhere:

**1. Ask for something visual — no need to name the skill.** In an agent
session opened in the project where you want the artifacts, just ask for the
*shape* of a deliverable:

> "give me a comparison of Postgres vs MySQL"
> "lay out a phased rollout plan for the migration"
> "diagram the request lifecycle"

The skill triggers on its own, writes a self-contained file to the **global
artifact store** at `~/.vellum/artifacts/<id>.html`, and opens it. The
first time it runs, `ensure-server.sh` transparently fetches the right release
binary for your OS/arch (verified against the release's `SHA256SUMS`), starts
it in the background on the first free port from **47600** up, and records
that port so subsequent artifacts reuse the same server. The artifact opens at
`http://127.0.0.1:<port>/view/<id>` if the server started successfully,
otherwise straight from the file.

> Auto-invocation is picked up when a session **starts**. If you just installed
> the skill, open a fresh session (or restart the extension) so Claude Code
> loads it.

**2. Annotate in the browser.** On any `/view/<id>` page, click **✎ Annotate**
(bottom-right), then click an element or select text and leave a comment.
Repeat, then hit **Send to agent** — that writes
`~/.vellum/artifacts/<id>.annotations.json`.

**3. Ask the agent to apply your notes.** Back in your editor session:

> "apply the annotations for `<id>`"

Claude Code reads the annotations, edits the artifact to match your described
changes, and re-opens it.

### Using it with other agents

The core is agent-neutral — artifacts are `.html` files and the server speaks
plain HTTP, so any agent that can read files and run shell commands can join.
**Claude Code, Codex, OpenCode, and Copilot CLI** are supported by `install.sh`
(see [Install](#install)); Claude Code gets an auto-invoked skill, the others get
a managed block in their `AGENTS.md`-style instruction file. All of them defer to
the one canonical [`instructions/CORE.md`](instructions/CORE.md) — see
[`adapters/README.md`](adapters/README.md) for the pattern and how to add another
agent.

## Server lifecycle

You never start or stop the server by hand. `ensure-server.sh` starts one shared
viewer on demand (per `~/.vellum`) and every session reuses it via the recorded
port. When it's auto-started this way it **reaps itself after 30 minutes with no
requests** (`VELLUM_IDLE_TIMEOUT`), so a forgotten viewer never lingers — and it
comes right back on the next artifact you open. Because it's shared, activity from
*any* session resets that idle clock, so it won't vanish out from under a session
that's still using it. Concurrent first-time starts are serialized with a lock, so
two sessions can never spawn duplicate servers. `SIGTERM`/`Ctrl-C` shut it down
gracefully. A manually run `vellum serve` (or `make serve`) never idle-exits
(`--idle-timeout 0`) — it runs until you stop it.

## Build & run (from source)

```sh
make build     # builds the server binary into ./bin/
make serve     # runs it on 127.0.0.1:47600, storing artifacts in ~/.vellum/artifacts
               # override: make serve PORT=8080 DIR=./some/other/dir
make test      # go vet + go test
```

Requires Go 1.23+. The server has zero third-party dependencies; the editor
shell and index template are embedded in the binary. If you'd rather install
the binary instead of building from a clone, `go install
github.com/abhiramnajith/vellum/server@latest` puts a binary named
`server` (not `vellum`) on your `$PATH`; run it with `server serve
--port 47600 --dir ~/.vellum/artifacts`.

Prebuilt binaries for the current release (once one is cut) are attached to
the corresponding [GitHub Release](https://github.com/abhiramnajith/vellum/releases),
alongside a `SHA256SUMS` file for verification — this is what `ensure-server.sh`
downloads and checks automatically, so most users never need to fetch these by hand.

## License

[MIT](LICENSE)

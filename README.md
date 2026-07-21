# html-artifacts

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
/plugin marketplace add abhiramnajith/html-artifacts
/plugin install html-artifacts
```

This installs the skill (instructions, template, and the `ensure-server.sh`
bootstrap script) directly from this repo — no separate clone step needed.
Restart your session (or reload the extension) afterwards so Claude Code picks
up the new skill.

### Any other agent (or Claude Code without the plugin) — via `install.sh`

```sh
git clone https://github.com/abhiramnajith/html-artifacts
cd html-artifacts
./install.sh --agent claude          # installs the adapter into ~/.claude/skills/
./install.sh --agent claude --local  # or into ./.claude/skills/ (project-local)
```

`install.sh` is agent-agnostic by design (`--agent codex|opencode|copilot`
directories exist to prove the pattern; only `claude` ships real content in
v1). It makes no network calls beyond `git`, and re-running it is idempotent.
Pass `--with-binary` to eagerly fetch the server binary during install instead
of waiting for first use.

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
artifact store** at `~/.html-artifacts/artifacts/<id>.html`, and opens it. The
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
`~/.html-artifacts/artifacts/<id>.annotations.json`.

**3. Ask the agent to apply your notes.** Back in your editor session:

> "apply the annotations for `<id>`"

Claude Code reads the annotations, edits the artifact to match your described
changes, and re-opens it.

### Using it with other agents

The core is agent-neutral — artifacts are `.html` files and the server speaks
plain HTTP, so any agent that can read files and run shell commands can join.
v1 ships only the Claude Code adapter; the `adapters/codex`, `adapters/opencode`,
and `adapters/copilot-cli` directories mark where thin per-agent adapters go
(each just needs that agent's trigger format plus a pointer to
[`instructions/CORE.md`](instructions/CORE.md)).

## Build & run (from source)

```sh
make build     # builds the server binary into ./bin/
make serve     # runs it on 127.0.0.1:47600, storing artifacts in ~/.html-artifacts/artifacts
               # override: make serve PORT=8080 DIR=./some/other/dir
make test      # go vet + go test
```

Requires Go 1.23+. The server has zero third-party dependencies; the editor
shell and index template are embedded in the binary. If you'd rather install
the binary instead of building from a clone, `go install
github.com/abhiramnajith/html-artifacts/server@latest` puts a binary named
`server` (not `html-artifacts`) on your `$PATH`; run it with `server serve
--port 47600 --dir ~/.html-artifacts/artifacts`.

Prebuilt binaries for the current release (once one is cut) are attached to
the corresponding [GitHub Release](https://github.com/abhiramnajith/html-artifacts/releases),
alongside a `SHA256SUMS` file for verification — this is what `ensure-server.sh`
downloads and checks automatically, so most users never need to fetch these by hand.

## License

[MIT](LICENSE)

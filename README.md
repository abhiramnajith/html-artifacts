# html-artifacts

An **agent-agnostic** tool that lets coding agents (Claude Code, Codex, OpenCode,
Copilot CLI, …) emit rich, self-contained **HTML artifacts** — plans, comparisons,
diagrams, tables, reports — instead of plain markdown, plus a **local** viewer/editor
to annotate elements and feed changes back to the agent.

Everything runs on `127.0.0.1`. No external hosting, no auth, no runtime dependencies.

> **Status: v1 complete.** Instructions + template, the local viewer server, and
> the annotation editor all work end-to-end. v1 ships the Claude Code adapter;
> the `adapters/` layout is ready for others. Build notes live in
> [`docs/PLAN.md`](docs/PLAN.md) and [`docs/design.md`](docs/design.md).

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

```sh
git clone https://github.com/abhiramnajith/html-artifacts
cd html-artifacts
./install.sh --agent claude          # installs the adapter into ~/.claude/skills/
./install.sh --agent claude --local  # or into ./.claude/skills/ (project-local)
```

Re-running `install.sh` is idempotent. It makes no network calls beyond `git`.

## Using it with your code editor (Claude Code)

The skill installs into Claude Code's skills directory, so any Claude Code
session — terminal, VS Code / JetBrains extension, or the desktop app — can use
it. The flow is the same everywhere:

**1. Install the adapter and start the viewer.**

```sh
./install.sh --agent claude   # global: ~/.claude/skills/  (use --local for this repo only)
make serve                    # viewer at http://127.0.0.1:7777  (leave running)
```

`make serve` runs the bundled Go binary. If you'd rather install it, `go install
github.com/abhiramnajith/html-artifacts/server@latest` puts a `server` binary on
your `$PATH`; run `server serve --port 7777 --dir ./artifacts`.

**2. Ask for something visual — no need to name the skill.** In a Claude Code
session opened in the project where you want the artifacts, just ask for the
*shape* of a deliverable:

> "give me a comparison of Postgres vs MySQL"
> "lay out a phased rollout plan for the migration"
> "diagram the request lifecycle"

The skill triggers on its own, writes a self-contained file to
`./artifacts/<id>.html`, and opens it — at `http://127.0.0.1:7777/view/<id>` if
the server is running, otherwise straight from the file.

> Auto-invocation is picked up when a session **starts**. If you just installed
> the skill, open a fresh session (or restart the extension) so Claude Code
> loads it.

**3. Annotate in the browser.** On any `/view/<id>` page, click **✎ Annotate**
(bottom-right), then click an element or select text and leave a comment.
Repeat, then hit **Send to agent** — that writes
`./artifacts/<id>.annotations.json`.

**4. Ask the agent to apply your notes.** Back in your editor session:

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
make serve     # runs it on 127.0.0.1:7777  (override: make serve PORT=8080 DIR=./artifacts)
make test      # go vet + go test
```

Requires Go 1.23+. The server has zero third-party dependencies; the editor
shell and index template are embedded in the binary.

## License

[MIT](LICENSE)

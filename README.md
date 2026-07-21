# html-artifacts

An **agent-agnostic** tool that lets coding agents (Claude Code, Codex, OpenCode,
Copilot CLI, …) emit rich, self-contained **HTML artifacts** — plans, comparisons,
diagrams, tables, reports — instead of plain markdown, plus a **local** viewer/editor
to annotate elements and feed changes back to the agent.

Everything runs on `127.0.0.1`. No external hosting, no auth, no runtime dependencies.

> **Status: Phase 0 (scaffolding).** The repo layout, build, install script, and CI
> are in place. Instructions, server, and editor arrive in later phases — see
> [`docs/PLAN.md`](docs/PLAN.md) (execution) and [`docs/design.md`](docs/design.md) (rationale).

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

## Build & run (from source)

```sh
make build     # builds the server binary into ./bin/
make serve     # runs it on 127.0.0.1:7777
make test      # go vet + go test
```

## License

[MIT](LICENSE)

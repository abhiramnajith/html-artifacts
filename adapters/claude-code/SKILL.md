---
name: vellum
description: Use when a deliverable is easier to grasp visually than as chat markdown — comparisons and "X vs Y" trade-off tables, phased plans and roadmaps, architecture/flow/sequence diagrams, structured reports and audits, or dense reference tables. Produces one self-contained, offline, light/dark HTML artifact, written to the global artifact store, and opens it in the local viewer in the browser. Triggers on the shape of the deliverable, not on the words "artifact" or "HTML".
---

# vellum

Produce a rich, self-contained **HTML artifact** instead of plain markdown when
the user asks for something structured and visual.

**All instructions live in `CORE.md`** (bundled next to this file). Read it and
follow it exactly: when to produce an artifact, the output contract (the
global artifact store, `~/.vellum/artifacts/<id>.html`, built from
`templates/base.html`), the content primitives, optional Mermaid diagrams, and
how to open the result.

Do not duplicate that logic here — this adapter only tells Claude Code when to
reach for the skill; `CORE.md` is the source of truth for how.

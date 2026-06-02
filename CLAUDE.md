# CLAUDE.md

The canonical agent guide for this repository is [AGENTS.md](./AGENTS.md).
Start there; it links to the focused notes under `.agents/project/`:

- `.agents/project/project.md` — product context, architecture, stack
- `.agents/project/workflow.md` — workflow, design gates, validation commands
- `.agents/project/conventions.md` — Go and project coding conventions
- `.agents/project/branch-naming.md` — branch naming rules
- `.agents/project/commits.md` — commit message rules

Project skills live in `.agents/skills/` (symlinked as `.claude/skills/`). For
async/queue work see the `goque-async-patterns` and `transaction-patterns`
skills; before handing work back, self-review with `code-reviewer` and
`code-skeptic`.

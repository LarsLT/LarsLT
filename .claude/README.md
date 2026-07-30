# .claude/

Project-specific Claude Code configuration. Everything in here (except `settings.local.json` and `dev/active/`) is checked in so the whole team gets the same setup.

## What lives where

- **`CLAUDE.md`** — auto-loaded into every Claude conversation. Top-level rules and pointers.
- **`agents/`** — specialized subagents (`esp-idf-expert`, `code-reviewer`).
- **`commands/`** — slash commands for repetitive flows (`/build`, `/flash`, ...).
- **`dev/active/`** — one folder per in-flight feature with a `PLAN.md`. Gitignored.
- **`docs/`** — durable project knowledge (architecture, conventions, cheatsheets).
- **`hooks/`** — project-local hook scripts. The shared formatting and comment-lint hooks ship via the `claude-kit` marketplace.
- **`rules/`** — hard rules Claude must follow when generating code.
- **`skills/`** — skills specific to this project. Shared ones (`stop-slop`, `grill-me`, `ponytail`, `karpathy-guidelines`, `comment-style`) ship via `claude-kit`.
- **`marketplaces/`, `plugins/`** — this project consumes the private `claude-kit` marketplace; see `marketplaces/README.md`.

## Onboarding a new dev

1. Clone repo.
2. Open in editor that supports Claude Code.
3. Copy `settings.local.json.example` to `settings.local.json` if you want extra perms.
4. Read `docs/architecture.md` and `rules/coding-style.md`.

## Updating

If you add a new convention or hit a non-obvious bug, update `docs/known-issues.md` or the matching `rules/*.md`. Don't let knowledge live only in chat.

## Shared configuration (claude-kit)

Some of this config is not stored in this repo. It comes from the private
`claude-kit` marketplace (https://github.com/LarsLT/claude-kit), declared in
`settings.json::extraKnownMarketplaces` and enabled per-plugin in
`settings.json::enabledPlugins`.

| Provided by claude-kit | Stays in this repo |
| --- | --- |
| `core` skills: `stop-slop`, `grill-me`, `ponytail`, `karpathy-guidelines` | `rules/` — this project's hard rules and threat model |
| `esp-idf` skill: `comment-style` | `docs/` — this project's knowledge |
| `esp-idf` hooks: `format-cpp`, `comment-lint` | `dev/` — this project's plans |
| `esp-idf` commands: `/clean`, `/size` | `agents/` — reviewers tuned to this codebase |
| | the hardware-specific skills |

Change a shared file in `claude-kit`, then run `/plugin` here to pick it up. Do not
copy shared files back into this repo — that is the drift this setup exists to stop.

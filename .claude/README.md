# .claude/

Claude Code configuration for the **`LarsLT/LarsLT` profile README** repo. Everything here
except `settings.local.json` is tracked in git, so the setup is the same on every machine.

## What lives where

- **`CLAUDE.md`** — auto-loaded into every Claude conversation. Top-level rules and pointers.
- **`docs/`** — durable knowledge. `profile-readme.md`: how the README renders (camo proxy,
  CSS-only SVG, the generated-animation workflow pattern).
- **`dev/`** — feature notes, one folder per idea, through `proposed/ → active/ → closed/`.
  The space-map animation lives in `dev/proposed/space-map/`.
- **`rules/`** — hard rules. `git.md`: single-branch `main`, never push.
- **`hooks/`** — project-local hooks. Shared formatting/comment-lint hooks ship via `claude-kit`.
- **`marketplaces/`, `plugins/`** — this repo consumes the `LarsLT/claude-kit` marketplace;
  see `marketplaces/README.md`.
- **`settings.json`** — shared perms, marketplaces, enabled plugins. `settings.local.json`
  is personal and gitignored.

## Updating

Hit a rendering gotcha or a new convention? Put it in `docs/profile-readme.md` or the
matching `rules/*.md`. Don't let knowledge live only in chat.

## Shared configuration (claude-kit)

Some config comes from the private `claude-kit` marketplace
(https://github.com/LarsLT/claude-kit), declared in `settings.json::extraKnownMarketplaces`
and enabled per-plugin in `settings.json::enabledPlugins`.

| Provided by claude-kit | Stays in this repo |
| --- | --- |
| `core` skills: `stop-slop`, `grill-me`, `ponytail`, `karpathy-guidelines` | `rules/` — this repo's hard rules |
| `go` skill: `comment-style` | `docs/` — this repo's rendering knowledge |
| `go` hooks: `format-go`, `comment-lint` (for generator tooling) | `dev/` — this repo's plans |

Change a shared file in `claude-kit`, then run `/plugin` here to pick it up. Don't copy
shared files back into this repo — that's the drift this setup exists to stop.

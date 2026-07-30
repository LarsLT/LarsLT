# Claude Code tooling added (2026-06-25)

Skills/plugins evaluated from an Instagram reel + three linked repos. Real star counts
verified with `gh api`, not the caption's inflated numbers.

## Installed (project scope, ESRead/.claude/)

| Name | Source | Type | Notes |
|------|--------|------|-------|
| `/stop-slop` | hardikpandya/stop-slop (12.3k) | skill | Strips AI-writing patterns from prose |
| `/karpathy-guidelines` | multica-ai/andrej-karpathy-skills (182k) | skill | Coding-discipline guidelines |
| `/grill-me` | RobMitt/grill-me-skill (68) | skill | Generic relentless-interview planning |
| `/plan-interrogation` | custom (this repo) | skill | Interrogation loop wired to PLAN.md rule + ESRead constraints |
| `superpowers` | obra/superpowers (238k) | plugin | TDD/planning/review framework, `superpowers@claude-plugins-official` |

## Installed globally (~/.claude/)

- **keshavsuki/recall-stack (32)** — global session-memory framework. Installed 2026-06-25
  with user's explicit permission. Cloned to `~/recall-stack`, ran `setup.sh` (no Docker
  container: no API key in env; no shell-rc edit: no --obsidian). Files: `~/.claude/primer.md`,
  `~/.claude/gates.json`, `~/.claude/failures.json`, `~/.claude/hooks/*.sh`. Hooks merged into
  `~/.claude/settings.json` (SessionStart, PreToolUse, PostCompact, SessionEnd) — **active next
  session restart**. Gates: block force-push / rm -rf ~ / creds-in-files, warn on reset --hard.
  Hindsight (Layer 4) NOT running — set ANTHROPIC_API_KEY + run the printed docker command if wanted.

## Skipped, with reason

- **thedotmack/claude-mem (84k)** — runs a worker service + localhost viewer; duplicates
  existing memory.
- **nextlevelbuilder/ui-ux-pro-max-skill (96k)** — web/app visual design; irrelevant to
  ESP32 C++ firmware.

## Bookmark (not installable — it's a curated index, not a skill/plugin)

- **awesome-claude-code**: https://github.com/hesreallyhim/awesome-claude-code (47k)
  Browse it to discover individual skills/hooks/plugins to install.

## recall-stack manual install (if you still want it)

Clone, read the hooks, then run from the repo root. Global, affects all projects:

    bash setup.sh                 # no --obsidian = no shell-rc edit; no API key in env = no Docker container

Then merge its `settings.json` hooks block into `~/.claude/settings.json` (it has none today).
Gate rules live in `~/.claude/gates.json` (block force-push, rm -rf ~, creds-in-files; warn on reset --hard).

## Revert

- Skills: delete the folder under `ESRead/.claude/skills/`.
- Superpowers: `claude plugin uninstall superpowers@claude-plugins-official --scope project`.
- Settings backup before any recall-stack change: `~/.claude/settings.json.bak-recallstack`.
- Full recall-stack revert: `cp ~/.claude/settings.json.bak-recallstack ~/.claude/settings.json`
  then `rm ~/.claude/hooks/{session-start,session-end,post-compact,pre-action-gate}.sh
  ~/.claude/{primer.md,gates.json,failures.json}` and `rm -rf ~/recall-stack`.

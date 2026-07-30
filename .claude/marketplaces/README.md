# marketplaces/

This project consumes the **`claude-kit`** marketplace:
https://github.com/LarsLT/claude-kit (private).

It is declared in `.claude/settings.json` under `extraKnownMarketplaces`, so there is
no config file to keep here — this folder is just the documented home for the decision.

Enabled plugins for this project are listed in `settings.json::enabledPlugins`. Run
`/plugin` to sync after the marketplace changes.

Enabled here: `core` (writing/planning skills) and `python` (formatting + comment-lint
hooks for the generator tooling).

Anything shared across projects (skills, hooks, slash commands) belongs in `claude-kit`,
not in this repo. Anything carrying repo-specific content — `rules/`, `docs/`, `dev/` —
deliberately stays here.

Reference: https://docs.claude.com/en/docs/claude-code/plugin-marketplaces

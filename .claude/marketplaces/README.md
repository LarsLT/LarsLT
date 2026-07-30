# marketplaces/

This project consumes the **`claude-kit`** marketplace:
https://github.com/LarsLT/claude-kit (private).

It is declared in `.claude/settings.json` under `extraKnownMarketplaces`, so there is
no config file to keep here — this folder is just the documented home for the decision.

Enabled plugins for this project are listed in `settings.json::enabledPlugins`. Run
`/plugin` to sync after the marketplace changes.

Anything shared across projects (skills, hooks, slash commands) belongs in `claude-kit`,
not in this repo. Anything carrying project-specific content — `rules/`, `docs/`,
`agents/code-reviewer.md`, the hardware-specific skills — deliberately stays here.

Reference: https://docs.claude.com/en/docs/claude-code/plugin-marketplaces

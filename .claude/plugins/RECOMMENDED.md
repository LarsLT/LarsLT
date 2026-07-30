# Recommended plugins for this repo

> Plugins are enabled in `.claude/settings.json::enabledPlugins`. Run `/plugin` inside
> Claude Code to sync/manage them. Keep it lean — every active plugin eats context.

This is a profile-README repo: Markdown, inline HTML/SVG, and small Python generators run
by GitHub Actions. There is no compiled code and no build to run, so the heavy code-analysis
and firmware plugins do **not** belong here.

## Enabled here

- **`core@claude-kit`** — language-agnostic writing/planning skills (`stop-slop`, `grill-me`,
  `ponytail`, `karpathy-guidelines`). Useful for README prose and plan quality.
- **`python@claude-kit`** — ruff format/lint + comment-lint hooks and the `comment-style`
  skill. For the generator tooling (e.g. the planned space-map builder). Harmless until a
  `.py` file exists; the hooks only fire on Python edits.
- **`superpowers@claude-plugins-official`** — general-purpose helper skills.

## Deliberately NOT enabled

- **`esp-idf@claude-kit`** — firmware/clang-format tooling. No C++ here, nothing to flash.
- **`pr-review-toolkit`** — this repo is single-branch and never pushes, so there are no PRs.
- **`security-guidance`** — heavy session hooks (SessionStart/UserPromptSubmit/PostToolUse/Stop);
  overkill for a static profile page plus small generators.
- **`semgrep`** — code SAST; not worth the weight for this repo's tiny surface. Re-enable if
  the Python tooling grows enough to warrant it.

## Managing plugins

```
/plugin                                  # interactive manager
/plugin disable <name>@<marketplace>     # turn off without uninstalling
/plugin enable  <name>@<marketplace>
/reload-plugins                          # apply changes without restarting Claude
```

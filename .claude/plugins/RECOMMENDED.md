# Recommended plugins for ESRead

> Plugins are installed interactively via `/plugin install`. They cannot be enabled by writing files alone. Run the commands below inside Claude Code.

> **Keep it lean.** Every active plugin baseline-eats your context window. Pick **2–3 active** at a time. Disable what you're not using.

## Marketplaces already registered

This project's `.claude/settings.json` declares two:

- `claude-plugins-official` — Anthropic's official marketplace, auto-available
- `anthropics-claude-code` — Anthropic's demo plugin set (declared via `extraKnownMarketplaces`)

First time you open the project, run `/plugin marketplace update anthropics-claude-code` to sync.

---

## Tier 1 — install these

### 1. `clangd-lsp@claude-plugins-official` — C/C++ language server

Gives Claude real-time diagnostics, jump-to-def, and find-references for this C++23 codebase. Project already has a `.clangd` file — this is the missing half.

**Requires:** `clangd` binary on `$PATH`. On Arch: `pacman -S clang`.

```
/plugin install clangd-lsp@claude-plugins-official
```

Test after install: edit a `.cpp`, intentionally typo a type name — Claude should call it out before you ask.

### 2. `commit-commands@anthropics-claude-code` — git commit / PR workflows

Adds `/commit-commands:commit`, push, PR-create skills with conventional commit message generation. Replaces ad-hoc commit prompts.

```
/plugin install commit-commands@anthropics-claude-code
```

### 3. `github@claude-plugins-official` — GitHub MCP

Lets Claude read issues, PRs, comments without you copy-pasting. Useful once the repo has a GitHub remote (currently local-only — install when you push).

```
/plugin install github@claude-plugins-official
```

---

## Tier 2 — install when relevant

### `pr-review-toolkit@anthropics-claude-code`

Multi-agent PR review (different agents check different angles in parallel). Install when you start opening PRs. Heavy on context — disable between reviews.

```


```

### `explanatory-output-style@claude-plugins-official`

Output style that explains *why* Claude made each choice. Good for learning ESP-IDF idioms; turn off when you want terse output.

```
/plugin install explanatory-output-style@claude-plugins-official
```

### About "brainstorming" / `feature-dev`

You mentioned a brainstorm plugin. The plugin most people talk about is **`feature-dev`** — a 7-phase workflow skill (requirements → exploration → architecture → implementation → testing → review → docs). It's reported as the most popular Claude Code skill (~89k installs).

It is NOT clearly in the two marketplaces above. To find it:

```
/plugin marketplace add Chat2AnyLLM/awesome-claude-plugins
/plugin
# then Tab to Discover and search "feature-dev" or "brainstorm"
```

Verify the publisher before installing — third-party plugins execute arbitrary code with your privileges (per Anthropic's own warning).

---

## Plugins to SKIP for this project

- `frontend-design` — irrelevant, no UI work
- `figma`, `vercel`, `firebase`, `supabase`, `notion`, `slack`, `linear`, `asana`, `atlassian` — not part of this project's workflow
- Any other LSP plugin (`pyright-lsp`, `rust-analyzer-lsp`, etc.) — only C/C++ in this repo
- `connect-apps` — broad integration plugin; large context cost for low value here

---

## Managing plugins

```
/plugin                                  # interactive manager (Discover / Installed / Marketplaces / Errors)
/plugin disable <name>@<marketplace>     # temporarily turn off without uninstall
/plugin enable  <name>@<marketplace>
/plugin uninstall <name>@<marketplace>
/reload-plugins                          # apply changes without restarting Claude
```

Install scopes when prompted:
- **Project** — shared with the team via `.claude/settings.json` (use for `clangd-lsp` etc.)
- **User** — across all your projects
- **Local** — only this repo, only you

For team-shared plugins, pick **Project** scope so it ends up in tracked settings.

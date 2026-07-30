# Hooks

Project-local Claude Code hooks. Currently empty.

The formatting and comment-lint hooks ship with the **`python`** plugin from the
`claude-kit` marketplace (for the generator tooling), wired via that plugin's
`hooks/hooks.json`. Edit them in https://github.com/LarsLT/claude-kit, not here.

Put a script in this folder only if it is genuinely specific to this repo. Wire it in
`.claude/settings.json` under the right event matcher, and document it here.

Reference: https://docs.claude.com/en/docs/claude-code/hooks

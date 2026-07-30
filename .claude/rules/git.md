# Git — HARD RULES

## G1. Commit your work

Do commit. Don't wait to be asked.

- Keep commits small. One commit = one unit of work.
- When a unit of work is done, commit it.

## G2. Commit message format

- One line. No body.
- Use a Conventional Commits type prefix: `feat: ...`, `fix: ...`, etc.
- No parentheses `()` — type prefix only, no scope. So `feat: ...`, never `feat(parser): ...`.
- No emojis.
- No `Co-Authored-By` trailer. Never attribute the commit to Claude, anywhere.

## G3. Always work on `main`

This repo is single-branch. Every commit lands directly on `main`.

- Never create, switch to, or commit on a side branch (`feat/*`, `fix/*`, `development`, ...).
- No feature-branch or PR workflow, and no merges — there is no other branch to merge into.
- If you find yourself on any branch other than `main`, stop and switch back to `main` before committing.

## G4. Never push

Never run `git push`. Commits stay local on `main`. The user pushes.

## G5. Diffs

`git diff` is allowed whenever it's useful or the user asks.

# dev/active/

One subfolder per in-flight feature or investigation. Persists across Claude sessions so context survives.

## Layout per feature

```
dev/active/<feature-slug>/
├── PLAN.md          # what + why + steps + decisions
├── NOTES.md         # scratch findings, dead ends, links
└── (any sketches, sample JSON, etc.)
```

## When to create one

- Multi-session feature (more than one Claude conversation).
- Investigation with findings worth keeping (a bug hunt with a non-obvious root cause).
- A refactor that touches multiple components and needs sequencing.

## When NOT to create one

- One-shot fixes — commit and move on.
- Anything fully derivable from git history or code.

## Lifecycle

This folder is the **active** stage of `proposed/ → active/ → closed/`. See
`../README.md` for the whole flow.

1. Create the folder with `PLAN.md` when work starts (or `git mv` it in from
   `dev/proposed/` if it began as a parked idea).
2. Update as decisions get made — `PLAN.md` is the source of truth, not chat.
3. When the feature **lands** (implemented and committed on `main`):
   - **Promote insights** with lasting value to `.claude/docs/`.
   - **Move the folder** to `dev/closed/<feature>/` with `git mv` (both stages are
     tracked in git).
   - **Delete** instead only if nothing's worth keeping.

## PLAN.md template

```markdown
# <feature name>

## Why
<problem this solves, who asked>

## Approach
<the chosen design, in 3-10 bullets>

## Steps
- [ ] step 1
- [ ] step 2

## Decisions
- chose X over Y because Z

## Open questions
- ?
```

This folder is tracked in git, so in-flight work syncs across machines (PC and laptop).

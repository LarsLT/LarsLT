# dev/ — feature notes lifecycle

One subfolder per idea / feature / investigation. Each lives in one of three
stages. A folder moves through them; it doesn't get copied.

```
proposed/  →  active/  →  closed/
 (idea)       (building)   (done, landed)
```

## Stages

- **`proposed/<slug>/`** — an idea we've discussed but aren't building yet.
  Park it here to sideline it without losing the thinking. Tracked in git so the
  backlog is shared and reviewable. Promote to `active/` when work starts; delete
  if we decide against it.

- **`active/<slug>/`** — in-flight work. `PLAN.md` is the source of truth, not
  chat. **Tracked** in git so in-flight work syncs across machines (e.g. PC and
  laptop). See `active/README.md` for the per-folder layout and `PLAN.md` template.

- **`closed/<slug>/`** — work that has **landed**: implemented, PR reviewed, and
  merged. Move the folder here from `active/` once that's true. Tracked in git as
  institutional memory (why we did it, what we rejected). Before moving, promote
  anything with lasting value (conventions, architecture) into `.claude/docs/`.

## Moving between stages

```bash
# idea graduates to real work
git mv .claude/dev/proposed/<slug> .claude/dev/active/<slug>

# work lands (PR reviewed + merged)
git mv .claude/dev/active/<slug> .claude/dev/closed/<slug>
```

All three stages are tracked in git, so use `git mv` between them and the history
follows. A brand-new `active/` folder still needs a `git add` to start tracking it.

## When to skip all this

One-shot fixes and anything fully derivable from git history or code. Commit and
move on — don't make a folder.

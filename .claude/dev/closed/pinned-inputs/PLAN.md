# Pin the inputs that can change under us

Two unrelated-looking items with the same shape: something this repo depends on is
referenced by a moving name, so the thing we get tomorrow need not be the thing we
got today. Both were raised in the July 2026 review and left undecided.

## 1. Third-party actions run on mutable tags

| file | line | action | owner |
|---|---|---|---|
| `.github/workflows/snake.yml` | 25 | `Platane/snk@v3` | third party |
| `.github/workflows/snake.yml` | 33 | `crazy-max/ghaction-github-pages@v4` | third party |
| `.github/workflows/space-map.yml` | 90 | `crazy-max/ghaction-github-pages@v4` | third party |

`actions/checkout`, `actions/setup-go` and `actions/cache` are GitHub-owned and are
conventionally left on tags — leave them.

**Why it matters here specifically.** Both workflows declare `permissions:
contents: write`. A tag is a pointer its owner can move at any time. If either
third-party action were retargeted, it would execute in this repo holding a token
that can write **any** branch, including `main` — not just the `output` / `space`
branches they are meant to touch.

This is a personal profile repo, not a production deploy, so the realistic risk is
low. It is listed because the fix is two lines and has no downside: a SHA still
does exactly what the tag did on the day you pinned it.

**Fix.** Pin to the commit each tag currently points at, keeping the tag in a
trailing comment so it stays readable and Dependabot can still offer bumps:

```bash
gh api repos/Platane/snk/git/ref/tags/v3 --jq .object.sha
gh api repos/crazy-max/ghaction-github-pages/git/ref/tags/v4 --jq .object.sha
```

```yaml
uses: Platane/snk@<sha>  # v3
```

Note both may be annotated tags — if `.object.type` comes back `tag` rather than
`commit`, dereference with `gh api repos/<owner>/<repo>/git/tags/<sha> --jq .object.sha`.

**Cost.** Three lines, one-time. Needs network, which is why it was left for you.

## 2. The basemap is generated from a moving branch

`tools/space-map/cmd/buildbasemap/main.go:23`

```go
const sourceURL = "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/master/..."
```

`buildbasemap` is a one-shot generator — it produced `data/basemap.json`, which is
committed, so no build ever downloads geometry. That part is right.

The problem is only reproducibility: rerunning it in a year silently picks up
whatever `master` holds then. `data/basemap.json` records `generated`, `source`,
`tolerance_deg`, `viewbox` and `rings`, so the *output* is well documented — but
the input it came from is not pinned, so the record cannot actually be reproduced.

**Fix.** Point the URL at a release tag or commit SHA instead of `master`, and let
`basemap.json`'s `source` field carry that exact ref. Natural Earth vector
publishes tags (`v5.1.2` and similar).

**Cost.** One line. Does not require regenerating the current basemap — the
existing file stays exactly as it is; this only fixes the *next* run.

## Recommendation

Do both. Neither changes any behaviour today, and both are the kind of thing that
is trivial now and annoying to reconstruct later. Item 1 is the one with an actual
(small) security argument behind it.

## Landed — 2026-07-31

Both done, one commit each.

Item 1, `chore: pin third-party actions to commit SHAs`. Both tags resolved to
lightweight tags (`.object.type` came back `commit`), so the annotated-tag
dereference in the note above was never needed:

| action | tag | commit |
|---|---|---|
| `Platane/snk` | v3 | `d8f6715049803e982ee5ff501b6b9b7d5deeb09b` |
| `crazy-max/ghaction-github-pages` | v4 | `df5cc2bfa78282ded844b354faee141f06b41865` |

Item 2, `chore: pin the basemap source to a natural-earth tag`. Pinned to `v5.1.2`,
the newest tag. Verified reproducible rather than assumed: the blob at `v5.1.2` and
at `master` are the same object (`1e6ab74c…`, 838 726 bytes), so a rerun regenerates
the committed `basemap.json` byte for byte instead of silently redrawing the map.

`data/basemap.json` was deliberately left alone. Its `source` field still records the
`master` URL, which is where it genuinely came from; the pin governs the next run.

**Convention going forward:** a third-party action added to a workflow gets a SHA and
a trailing `# vN` comment. GitHub-owned actions (`actions/checkout`, `setup-go`,
`cache`) stay on tags.

# Profile README — how it renders and how not to break it

This repo's `README.md` renders at https://github.com/LarsLT. Everything below is about
getting pixels onto that page reliably.

## The camo image proxy

GitHub rewrites every image URL in rendered Markdown to a `camo.githubusercontent.com`
URL and fetches/caches the image server-side. Consequences:

- The browser never talks to shields.io / skillicons.dev / capsule-render / github-readme-stats
  directly — camo does, then caches. A service being up in your browser doesn't mean camo
  fetched it in time.
- Caching can serve a **stale** image after you change a badge. A hard refresh or a changed
  URL busts it. Don't assume "it didn't update" means "it's broken."
- Anything camo can't fetch (bot-walled, rate-limited, timing out) shows as a broken image
  on the profile. Prefer services that are fast and camo-friendly.

## SVG animation must survive sanitization

GitHub sanitizes embedded/served SVG before display:

- **No `<script>`** — stripped. No JS-driven animation.
- **No external references** inside the SVG (no `<image href="http…">`, no remote fonts) —
  camo blocks them.
- **CSS `@keyframes` inside `<style>` survives.** The Platane/snk snake is the proof.
  Motion along a path uses CSS `offset-path`, not SMIL `<animateMotion>`.

So any generated animation is a **self-contained SVG**: geometry as inline paths, motion as
CSS keyframes. To feel "live" without a server, bake the clock into the CSS — give a periodic
animation an `animation-duration` equal to its real-world period and a negative
`animation-delay` equal to the elapsed time at generation, so it's phase-correct on load and
a periodic rebuild only re-syncs it. (See the space-map plan for a worked example.)

## Light / dark theming

Use a `<picture>` with `<source media="(prefers-color-scheme: dark)" srcset="…dark…">` and a
default `<img>` for light — the README already does this for the github-readme-stats cards.
For a single-mood piece (e.g. a starfield), a dark-only SVG is fine; skip the pair.

## Generated animations: the workflow pattern

`snake.yml` is the template for self-updating content:

1. An Actions workflow runs on a cron (`schedule:`) plus `workflow_dispatch`.
2. It generates SVG(s) into a build dir.
3. It publishes the build dir to a **dedicated output branch** — snake uses `output`.
4. The README embeds the SVG via a `raw.githubusercontent.com/LarsLT/LarsLT/<branch>/<file>` URL.

Rules of thumb for a new one:

- **Give it its own branch.** Two workflows pushing to the same branch clobber each other.
  The planned space-map uses `space`, not `output`, for exactly this reason.
- **`main` stays source-only.** Never commit generated SVGs / `dist/` onto `main`.
- **Degrade, don't break.** Wrap every upstream fetch (timeout + one retry); on failure drop
  that layer and still emit a valid SVG. A broken upstream must never leave a broken image on
  the profile. Cache the last good response as a fallback for rate limits.
- GitHub **disables scheduled workflows after ~60 days of repo inactivity**. `keepalive.yml`
  covers this: an empty commit on `main`, monthly. A push from a generator workflow doesn't
  substitute for it — those land on `space` / `output`, not the default branch. If the map
  ever does freeze, `workflow_dispatch` restarts it and the cron resumes.

## Verifying a change

- Local, for a generator: build the SVG and open it (`xdg-open dist/<name>.svg`); grep it for
  `<script` (expect 0) and for `http` external refs (expect none).
- End to end: the only test that counts is loading **github.com/LarsLT** and watching it render
  and animate through camo. For a baked-clock animation, check again later to confirm it kept
  moving between rebuilds.

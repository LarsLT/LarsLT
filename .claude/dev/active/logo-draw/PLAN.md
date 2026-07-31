# Animated logo at the top of the profile

## Why

The profile has no personal mark on it — it opens with a capsule-render banner that any
profile could use. The Zelf mark exists (`../Logo`, a separate repo) and the portfolio site
(`../website`) already animates it as a stroke draw-in. Lars asked for the same thing here,
above the banner, so the mark is the first thing a visitor sees.

## Approach

- **Copy the mark, don't regenerate it.** `Logo/dist/Logo.svg` is one unbroken path — the
  same `d` the website's `Logo.tsx` inlines. Copy it verbatim into `assets/logo-draw.svg`.
- **Reuse the website's technique** (`website/frontend/src/Components/Logo/Logo.css`):
  `pathLength="1"` + `stroke-dasharray: 1` / `stroke-dashoffset: 1 → 0`, `2s linear forwards`.
- **CSS `@keyframes` in an inline `<style>`.** No script, no SMIL, no external refs, so it
  survives GitHub's SVG sanitizer. Same class of animation as the snake.
- **Keep the `prefers-reduced-motion: no-preference` guard.** Outside the query the path
  carries no dash properties at all, so reduced-motion users get the finished mark rather
  than an empty box.
- **Stroke `#58a6ff`**, the accent already used by the banner gradient, the typing SVG and
  the badges. Readable on both GitHub themes.
- **Static file on `main`**, embedded with a relative `<img src="assets/logo-draw.svg">`
  inside the existing top `<div align="center">`, above the banner.
- **No generator, no workflow, no output branch.** The mark doesn't change and there's no
  live data to fetch.

## Steps

- [ ] Write `assets/logo-draw.svg` — path from `Logo/dist/Logo.svg`, `pathLength="1"`,
      stroke `#58a6ff`, inline `<style>` with the reduced-motion-guarded keyframes
- [ ] Verify locally: no `<script`, no external `http` refs, renders and animates in a browser
- [ ] Add the `<img>` to `README.md` above the banner
- [ ] Note the convention in `.claude/docs/profile-readme.md`
- [ ] Confirm on github.com/LarsLT in both themes (needs a push — Lars does that)

## Decisions

- **Accent blue over a light/dark `<picture>` pair.** The mark only exists as `#FFFFFF`, so
  it is invisible on GitHub's light theme as-is. A `<picture>` pair would have worked but
  needs a dark-stroke light-background variant, and `Logo/.claude/docs/brand.md` lists
  "light-background variant" as an open brand question that shouldn't be resolved from here.
  Tinting to the README's own accent keeps this a page-local choice, not a brand decision.
- **One-shot, not looping.** A `forwards` draw-in can finish before a visitor scrolls to it,
  which argued for a looping draw/hold/reset cycle. Placing the logo at the very top removes
  the problem, so the animation stays byte-for-byte the website's behaviour.
- **`main`, not a dedicated branch.** Hard rule 3 keeps *generated* output off `main`. A
  hand-authored SVG is source, so `assets/` on `main` is the right home; a branch and a
  workflow would exist only to copy a file that never changes.
- **`Logo.svg`, not `logo-oneline-arc.svg` or `logo-weave.svg`.** All three lineages are
  maintained upstream with no winner declared, but `Logo.svg` is what the website animates,
  so the two surfaces match. `logo-weave.svg` is 12 separate edges and cannot animate as a
  single dash sweep at all.
- **Don't touch the arc sweep flags.** Per `brand.md`, the draw order is deliberate: each
  petal leaves on the side away from where the previous stroke ended, so the sweep reads as
  one motion. Flipping a sweep flag looks identical statically and visibly doubles back
  when animated.

## Open questions

- The mark now appears in the README accent blue while `Logo/dist/*` is still white-only.
  If the Logo repo ever settles the light-background question, revisit whether this should
  point at a real variant instead of a local tint.

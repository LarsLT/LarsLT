# Live space map for the profile README

## Why

This repo already generates one animation: `.github/workflows/snake.yml` builds the
Platane/snk contribution snake and pushes SVGs to the `output` branch, embedded in
`README.md`. Lars wants a second one in the same spirit, but about space instead of commits.

A world map showing what the sky is doing right now: rocket launches, the path of the next
solar eclipse, where the aurora is visible, plus a day/night terminator, the ISS ground
track, and active meteor showers. Underneath it, a short static paragraph about why space
interests him.

It is a showpiece, not a monitoring tool. Nobody opens GitHub to decide whether to go
outside. So the map has to be legible at a glance: you see the eclipse band crossing a
continent, you see the aurora oval sitting over northern Europe, and you understand. No
personal observer marker, the geography does the explaining.

## Feasibility

Every source below was called for real before this plan was written.

| Source | Endpoint | Result |
|---|---|---|
| Launch Library 2.3.0 | `ll.thespacedevs.com/2.3.0/launches/upcoming/` | 200, 367 upcoming, `pad.latitude/longitude` present |
| Kp index (GFZ Potsdam) | `kp.gfz.de/app/json/?start=…&index=Kp` | 200, 3-hourly Kp array |
| ISS TLE | `celestrak.org/NORAD/elements/gp.php?CATNR=25544` | 200, current TLE |
| ISS position | `api.wheretheiss.at/v1/satellites/25544` | 200 (fallback source) |
| Eclipse paths | `eclipse.gsfc.nasa.gov/SEpath/SEpath2001/SE2026Aug12Tpath.html` | 200, lat/lon table at 120 s steps |
| Natural Earth 110m | `nvkelso/natural-earth-vector` GeoJSON | 200, 838 KB, 177 countries |
| NASA APOD / DONKI | `api.nasa.gov` with `DEMO_KEY` | 200 (not used in v1) |

NOAA SWPC is blocked from the dev machine. `services.swpc.noaa.gov` returns HTTP 202 with
an empty body on every path tried, with and without browser headers. That is an Akamai bot
wall. So the OVATION aurora probability grid is not the primary aurora source. Kp from GFZ
is, and the auroral oval gets derived from it. M5 re-probes SWPC from an Actions runner; if
it answers there, the real OVATION contour becomes an optional layer on top.

## Approach

### The idea that makes it feel live

Actions cron is best-effort and drifts 5-15 minutes. A 30-minute rebuild on its own gives a
map that is visibly wrong between runs. So the SVG carries its own clock.

Every periodic animation gets an `animation-duration` equal to its real-world period, and a
negative `animation-delay` equal to the time already elapsed in that cycle at generation
time.

```css
/* night side sweeps a real 24 h, the negative delay puts it at the right longitude on load */
.terminator { animation: sweep 86400s linear infinite; animation-delay: -49380s; }
```

The terminator genuinely tracks the sun and the ISS genuinely moves, inside a static file.
The rebuild only re-syncs the phase and refreshes data, so worst case error is about 30
minutes, roughly 7.5 degrees of longitude. Invisible at this scale.

### Layers, back to front

1. Background: deep navy gradient, ~140 seeded stars in 3 twinkle classes, staggered delays.
2. Graticule every 30 degrees, very faint.
3. Countries: Natural Earth 110m, pre-simplified into committed SVG path strings.
4. Night side: terminator polygon from the subsolar point, drawn twice side by side in a
   group translated by -W over 86400 s for a seamless loop. Translate-only is exact for a
   fixed solar declination, and declination is re-baked every run.
5. Aurora ovals: north and south rings around the geomagnetic poles. Equatorward boundary
   from the Kp table (Kp 0 at 66.5 degrees corrected geomagnetic latitude, about 2 degrees
   further south per Kp step, Kp 9 at 48). Green to teal gradient, 4 s opacity pulse. A
   label says the boundary in plain words, "visible down to ~55N, northern Netherlands".
6. Eclipse path: next solar eclipse as a filled band between northern limit, central line
   and southern limit. Umbra disc loops along the centre line in 12 s. Date, type and
   crossed regions labelled, so a fast demo loop cannot read as a live event.
7. Meteor showers: active showers get short diagonal streaks over the latitude band they
   are visible from, staggered so they do not fire in unison.
8. Launch pads: a dot per pad with a launch in the next 30 days. The next launch gets a
   pulsing ring, a label, and a small trajectory arc.
9. ISS: ground track polyline from the propagated TLE, plus a glowing dot riding it.
10. Chrome: title, legend, bottom ticker with next 3 launches, next eclipse, Kp and aurora
    reach, active shower, and "updated every 30 min".

Canvas 1000x560. The map is a 1000x500 equirectangular projection,
`x = (lon+180)/360*W`, `y = (90-lat)/180*H`, with header and legend bands around it.

### Files

```
.github/workflows/space-map.yml
tools/space-map/
├── build.py                 # fetch, model, render, write dist/space-map.svg
├── sources/{launches,aurora,iss,eclipses,showers}.py
├── render/{projection,theme,layers,svg}.py
├── data/                    # committed, generated once, never fetched at runtime
│   ├── basemap.json         # country path strings
│   ├── eclipses.json        # solar eclipse paths 2026-2030
│   └── meteor_showers.json  # IMO peak dates, radiants, ZHR
├── scripts/{build_basemap,build_eclipses}.py   # one-off generators
└── requirements.txt         # requests, sgp4
```

Runtime dependencies stay at two, `requests` and `sgp4`. The projection is four lines of
arithmetic and the SVG is plain string templating. No d3, no headless browser, no geo
stack. Those live only in the one-off `scripts/` that produce the committed JSON.

### Degradation

Every fetch is wrapped: timeout, one retry, and on failure the layer drops out, a warning
is logged, and the build still succeeds with a shorter legend. A broken upstream must never
leave a broken image on the profile. `--offline` forces every source to fail, for testing.
The last good Launch Library response is cached on the output branch as a 429 fallback.

## Steps

- [ ] M0. This file, committed.
- [ ] M1. `scripts/build_basemap.py`: Natural Earth 110m, Douglas-Peucker at ~0.35 degrees,
      coordinates rounded to 1 decimal, antimeridian split, out to `data/basemap.json`.
      Target 80 KB or under.
- [ ] M2. `build.py` skeleton plus `render/` producing a static SVG: starfield, graticule,
      countries, header, legend. Open it in a browser. This is the "does it look good" gate,
      fix the palette here rather than later.
- [ ] M3. Terminator layer and the baked-clock helper `phase_delay(period, epoch)`, reused
      by every later animation.
- [ ] M4. Launches: Launch Library 2.3.0, next 30 days, pad dots, next-launch ring and
      label, ticker lines. Cache the last good response.
- [ ] M5. Aurora: GFZ Kp, oval geometry, visibility sentence. Probe SWPC OVATION from the
      runner in the same job and log the status code, then decide on the enrichment layer.
- [ ] M6. `scripts/build_eclipses.py`: parse the NASA `SEpath` tables for solar eclipses
      2026-2030 into `data/eclipses.json`, then the band and umbra layer.
- [ ] M7. ISS: Celestrak TLE, `sgp4`, ground track for plus or minus one orbit, dot on a CSS
      `offset-path`. `wheretheiss.at` as fallback for the instantaneous point.
- [ ] M8. Meteor showers from the static table, streak layer.
- [ ] M9. `space-map.yml`: `*/30 * * * *`, `workflow_dispatch`, push on main filtered to
      `tools/space-map/**`. Push `dist/` to a `space` branch, not `output`, see Decisions.
      Verify degradation with `--offline` and with a forced 429.
- [ ] M10. README: new section above the snake, embedding
      `raw.githubusercontent.com/LarsLT/LarsLT/space/space-map.svg`, plus the static space
      paragraph. Claude drafts 3-5 lines, Lars rewrites them in his own voice.

## Decisions

- Separate `space` branch, not `output`. `crazy-max/ghaction-github-pages@v4` replaces the
  target branch with `build_dir`. Snake already pushes `dist/` to `output`, so a second
  workflow doing the same would clobber the snake SVGs on every run and vice versa. A
  separate branch is one line and zero coupling.
- Dark-only SVG, no light variant. Space is dark. A light-themed starfield looks like a
  mistake. One file, no `<picture>` pair.
- CSS keyframes only. No SMIL, no `<script>`. GitHub strips scripts and proxies images
  through camo, and snk proves CSS animation inside `<style>` survives that path. Motion
  along a path uses CSS `offset-path`, not `<animateMotion>`.
- Eclipse paths precomputed and committed. A handful of solar eclipses per year, geometry
  fixed centuries ahead. Scraping NASA every 30 minutes would be pointless traffic against
  a page that never changes.
- Kp-derived oval as the primary aurora layer, OVATION as optional enrichment. Verified
  reachable beats prettier but blocked. The oval is also cleaner to draw than a 65k-point
  probability grid.
- Absolute launch times inside the SVG, relative ones only in README text. SVG cannot count
  down without JS, and a baked "T-3h" string is a lie forty minutes later.
- Actions over a hosted renderer. A Cloudflare Worker rendering on request would be truly
  live, but it adds a deploy, a domain and a service to keep alive for a profile
  decoration. This matches the snake pattern and is free on public repos.

## Verification

```bash
# local build, then look at it
python tools/space-map/build.py --out dist && xdg-open dist/space-map.svg

# every source down, must still emit a valid SVG
python tools/space-map/build.py --out dist --offline

# no scripts, no external references, camo blocks them and GitHub strips them
grep -c '<script' dist/space-map.svg          # expect 0
grep -oE 'https?://[^"]+' dist/space-map.svg  # expect no hits
du -h dist/space-map.svg                      # target under 250 KB
```

End to end: run the workflow with `workflow_dispatch`, confirm the `space` branch updates,
then load `github.com/LarsLT` and check the animation actually plays through camo. That is
the only test that counts, camo is the one hop that can break it. Check again an hour later
to confirm the baked clock kept the terminator moving without a rebuild.

## Open questions

- Does an Actions runner get past the SWPC bot wall? Decided at M5 from a logged status code.
- Cron frequency versus commit noise. `*/30` is about 48 commits a day on the `space`
  branch. If that gets annoying, force-push the branch with a single rolling commit instead.
- GitHub disables scheduled workflows after 60 days of repo inactivity, and bot commits do
  not count as activity. If the profile goes quiet the map silently freezes. Worth a note in
  the README section, or a monthly manual `workflow_dispatch`.

## Parked for later

Discussed and declined for v1, kept so the thinking is not lost: moon phase disc, Kp storm
gauge with a needle, deep-space ticker (Voyager 1 and 2 distance, next near-Earth asteroid
pass, latest solar flare class), APOD image of the day, comet visibility, Starlink train
passes, and lunar eclipses. That last one is nearly free once the terminator layer exists,
since visibility is just the night hemisphere.

## Note on repo rules

`.claude/CLAUDE.md` here is ESRead carryover and describes an ESP-IDF C++ project. Those
C++ and ESP-IDF rules do not apply to this Python tooling. The git rules do: work on `main`,
one-line Conventional Commit messages, never push.

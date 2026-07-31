# Ascent arc for the next launch

## Why

The launch layer is the least alive thing on the map. A pad is a dot, the soonest
one gets a ring, and at T-0 nothing happens — the flight just drops out of the feed
at the next rebuild and the ring hops elsewhere. Nothing shows where the rocket
*goes*.

Wanted: a dot climbing away from the pad along the track it would fly if everything
went perfectly. Explicitly a simulation, not a live feed — the same honesty contract
the eclipse umbra already runs under ("a prop, not a clock").

## Feasibility

Measured against the captured feed in `tools/space-map/cache/launches.json`
(60 records, 2026-07-30).

**What the feed gives us**

| field | coverage | use |
|---|---|---|
| `pad.latitude` / `longitude` | 60/60 | arc origin |
| `mission.orbit.abbrev` | 60/60 | nominal inclination |

Orbit classes present: `LEO` 23, `PO` 10, `SSO` 8, `GTO` 4, `LO` 3, `MEO` 2,
`N/A` 7, plus one each `L2`, `Sub`, `Mars`.

**What it does not give us — checked, not assumed**

- **No target inclination.** The string "inclination" appears in the payload only
  inside free-text mission descriptions. There is no structured field, in any
  response mode we use.
- No azimuth, trajectory, apogee or perigee fields.

So the aim has to be inferred from orbit class. That is the whole accuracy story
of this feature, and it should be stated in the plan rather than discovered later.

## Approach

### Aiming the arc

Launch azimuth from the standard relation:

```
sin(azimuth) = cos(inclination) / cos(pad latitude)
```

with inclination taken from a small table by orbit class, defaulting to the
minimum-energy case (inclination = pad latitude, i.e. due east):

| class | nominal inclination |
|---|---|
| `SSO` | 97.8° |
| `PO` | 90° |
| `MEO` | 55° |
| everything else | \|pad latitude\| (due east) |

Spot-checked against real pads, and it lands on the right answers where the
answer is unambiguous:

```
Cape Canaveral   GTO   az  90.0  E     correct
Guiana Centre    GTO   az  90.0  E     correct
Cape Canaveral   MEO   az  40.7  NE    correct (GPS launches go northeast)
Vandenberg       SSO   az 189.5  S     correct
Rocket Lab NZ    SSO   az 190.1  S     correct
```

### The two things it will get wrong

**1. The azimuth branch is a range-safety decision, not a physical one.**
`asin` gives two valid roots — a northbound and a southbound launch reach the same
inclination. Which one a site actually flies depends on what it is allowed to fly
over. Picking the southward branch blindly is wrong at least twice in the current
feed:

- Vandenberg `PO` → computes 0.0° (north). Real polar launches from Vandenberg fly
  **south**, down the Pacific.
- Guiana `SSO` → computes 187.8° (south). Kourou flies SSO **north** over the
  Atlantic.

No formula fixes this. It needs a small per-site table of preferred headings for
the dozen spaceports that actually appear, defaulting to the prograde branch.

**2. Inclination guessed from orbit class is wrong whenever a mission targets a
specific higher inclination.** `LEO` is the worst case — it covers ISS flights at
51.6° and Starlink shells at 43° and 53°, but the default treats it as
minimum-energy. Baikonur is the visible example: computed 90° (due east) where a
Progress flight to the ISS really flies northeast at ~35°.

Both are acceptable for a piece labelled as nominal. Neither should be quietly
presented as the real trajectory.

### Drawing it

- Sample a great circle from the pad along the azimuth, ~25° of arc (~2 800 km),
  which is roughly ground-track distance to orbital insertion.
- Split at the antimeridian with the existing `render.TrackD`, exactly as the ISS
  ground track and the eclipse centre line already do.
- Animate with CSS `offset-path`, same construction as `.umbra` and `.station` —
  no `<script>`, no SMIL, nothing camo blocks.
- The arc belongs to the ringed next-launch pad only. One arc, never a bundle.

## Decisions — 2026-07-31

**D1: loop it, styled quiet.** One visual state, no date gating. The feature was
asked for as a simulation, and the umbra's own comment already sanctions a prop
that loops in seconds where the real thing takes hours. Thin stroke, low opacity,
no glow, so it reads as a projection; the label keeps carrying the real T-0.

**D2: skip `Sub`, `Mars` and `L2`.** A 25-degree insertion arc means nothing for
a sounding rocket or an escape trajectory. They keep the plain pad dot. Three of
sixty records in the captured feed.

**D3: no per-site heading table.** Trust the formula and take whichever root it
gives. This is a deliberate accuracy trade, made with the cost known: the two
cases this plan already identified stay wrong.

- Vandenberg `PO` draws due north, up the California coast, where the real
  flight goes south down the Pacific.
- Guiana `SSO` draws south over Brazil, where Kourou really flies north over
  the Atlantic.

Both draw over land, which is exactly the tell the Verification section names.
If that looks bad on the live profile, the fix is the table this decision
dropped, not a change to the geometry.

## Open decisions (resolved above)

**D1 — does the dot loop, or only run near T-0?** The umbra was just gated to run
only while the eclipse is genuinely happening, and the meteor layer was deleted for
inventing motion. An ascent that loops forever for a flight three weeks out is the
same pattern.

But this feature was asked for explicitly as a simulation ("doesn't need to be real
time, if everything went perfect that flight path"), which the umbra's own comment
already sanctions. Suggested: loop it, and keep the arc visually quiet — thin,
low-opacity, no glow — so it reads as a projection rather than telemetry.
Alternative if it grates: draw the static arc always, animate the dot only inside
some window before T-0.

**D2 — suborbital and escape missions.** `Sub` never reaches orbit and `Mars`/`L2`
leave it. Either skip them or shorten the arc. Skipping is simpler and honest.

**D3 — the per-site heading table.** Which spaceports to include, and what to do
for one that is not listed.

## Steps

1. `internal/astro`: `LaunchAzimuth(padLat, inclination)` and `GreatCircle(from,
   azimuth, arcDeg, stepDeg)`. Pure geometry, unit-testable with no network.
2. `internal/sources`: decode `mission.orbit.abbrev` onto `Launch`. One field.
3. `main.go`: nominal-inclination table, per-site heading table, build the arc for
   `pads[0]` only.
4. `internal/render`: `Sky.Ascent`, a path plus a looping dot, one `@keyframes`
   reused from the umbra's shape.
5. Tests: azimuth against the five verified pads above; great circle closes and
   stays on the sphere; `Document` still emits well-formed XML with the layer both
   present and absent.

## Verification

```bash
# geometry, no network
go test ./internal/astro/... ./internal/render/...

# a real build, and the degraded one
go run . -out dist -cache cache
go run . -out dist -offline

# camo/sanitiser gate, same as CI
grep -c '<script\|<animate' dist/space-map.svg   # must be 0
```

Then eyeball: the arc must leave the pad over water for every site in the feed. An
arc crossing a continent immediately is the tell that D1's heading table is wrong.

## Parked for later

- Real trajectories dogleg (Baikonur avoids China, Canaveral bends for the
  Bahamas). A great circle never will.
- Actual ascent timing is not linear in ground distance — a real dot accelerates.
  Fixable with the arc-length keyframes trick already used for the ISS, if it ever
  matters.
- Booster return tracks. Feed carries no landing data.

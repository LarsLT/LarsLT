# Aurora visibility

## Why

The map draws the auroral oval on every run, so the green band is always there and means
nothing. Lars: "I only want to see it when the aurora is actually visible."

Visible is not the same as storming. Tromsø sees aurora at Kp 1; the oval is a permanent
feature that simply sits over the Arctic when the field is quiet. So the band must answer
"can anyone see this, and from where" rather than "is the index high":

- Norway can see it → the band covers Norway.
- The Netherlands can see it → the band covers the Netherlands.
- Nobody can see it → nothing is drawn.

Three things decide that, and the map currently uses only the first:

1. **Emission** — where the glow is.
2. **Horizon reach** — aurora sits ~100 km up, so it is seen from a few hundred kilometres
   equatorward of where it stands. The band people can see from is wider than the oval.
3. **Darkness** — the sun drowns it. A daylit oval is invisible no matter how strong.

The rebuild runs every 30 minutes, so live measurements plus a short forecast both fit.

## Approach

- **Drive the shape from OVATION, not from Kp.** `services.swpc.noaa.gov/json/
  ovation_aurora_latest.json` is a 360×181 grid of aurora probability with a `Forecast Time`
  about 40-80 minutes ahead of its observation. That is both halves of what Lars asked for:
  measured now, extrapolated to what is expected. It is also the real oval — asymmetric, offset
  towards midnight — instead of a small circle around the geomagnetic pole.
- **Trace the band per meridian.** For each of the 360 longitude columns take the equatorward
  and poleward latitudes where probability crosses the threshold, in each hemisphere. That
  reuses the existing "edge out, edge back" polygon shape and its antimeridian handling.
  Longitudes with nothing above threshold break the ring into separate runs.
- **Add the horizon skirt.** The drawn equatorward edge sits `visibilityReach` below the
  emission edge, so the band is the footprint you could see it from, which is what the words
  on the ticker already promise.
- **Mask by night, and let the mask move.** The night polygon already slides one map width
  west over 86400 s. The aurora goes behind a `<mask>` holding that same sliding shape, so the
  band shows only on the dark side and stays in sync as the terminator sweeps, instead of
  being clipped once at generation time and drifting out of register within the hour.
- **Say nothing when nothing is visible.** If no part of the footprint is dark at generation
  time, the layer, its legend chip and its ticker line are all left out.
- **Degrade in two steps.** OVATION unreachable → fall back to the Kp-derived oval that exists
  today. Kp unreachable too → no aurora layer, map still renders.

## Steps

- [x] Drop the `Storming` Kp≥5 gate (3ce12be) — wrong model, hides quiet aurora Norway can see
- [x] `internal/sources/ovation.go` — fetch, cache, validate the probability grid
- [x] `internal/astro/ovation.go` — threshold the grid into bands per hemisphere, with the
      horizon skirt and gaps split into runs
- [x] `internal/render` — night mask, feathered edge, aurora drawn through it
- [x] `main.go` — OVATION first, Kp oval as fallback, ticker sentence from the drawn footprint
- [x] Darkness test: an oval entirely in daylight produces no layer, no legend, no ticker
- [ ] Verify the mask survives camo (precedent: `clipPath` and gradients already render)
- [ ] Watch a run of the workflow: does SWPC answer an Actions runner, or is the Kp oval
      carrying the layer in production

## Decisions

- **Two thresholds, not one: seed at 8%, trace out to 4%.** A single low contour was tried at
  3% and drew the polar cap as a slab across the bottom of the map — on a quiet day the cap
  carries 3-5% of diffuse glow with no oval under it at all, and the projection stretches the
  pole to the full map width. The seed demands proof of an oval on that meridian; the lower
  contour then keeps the diffuse edge, which is real aurora and where the faint horizon arcs
  live. Measured against the live grid: 8/4 leaves the cap alone in both hemispheres and moves
  the equatorward reach by one degree.
- **Arcs get tapered ends and the whole band is blurred.** Above the seed only the active
  sector of the oval survives, so a band stops dead at a meridian — a wall of light with a
  vertical edge. The run closes to a point 4 degrees past its last meridian, and a gaussian
  blur takes the outline off the whole shape. A run reaching the seam is left square, since it
  carries on over the map's other edge.
- **Mask, not clip-at-build.** A build-time clip is 7.5° of longitude stale after 30 minutes
  and visibly wrong to anyone who watches the terminator move. The mask costs one `<mask>`
  element and is correct for as long as the file is open.
- **Reuse the geometric night polygon** for the mask rather than a −6° civil-twilight version.
  One shape, guaranteed in sync with the terminator that is already drawn. A feathered mask
  edge covers the overstatement at dusk. Revisit if it reads too generous.
- **Kp oval kept as the fallback**, not deleted. SWPC was bot-walled from this machine when
  the space-map plan was written and answers now; it may well block the Actions runner.
- **"Nobody" means nowhere on Earth**, not nowhere populated. The southern oval over an empty
  Antarctic ocean still counts as visible.
- **The band's brightness fades across the skirt**, so the strong green sits where the glow
  hangs overhead and the faint edge where it is only a light on someone's horizon. That needs
  a gradient per band rather than the two fixed ones, since the skirt is a different share of
  the band's height each run. Without it the map was brightest exactly where it was weakest.
- **The ticker quotes the reach at midnight, and says so.** The southernmost point of the band
  is in whatever sector local midnight is over: 48N on a Kp 0.7 night, which is real over
  Alberta and nonsense read as "near the Netherlands". The Dutch line only appears when the
  footprint actually covers the country and it is dark there.

## Open questions

- Does SWPC answer from a GitHub Actions runner? Unknown until the workflow runs — the
  fallback exists for exactly this.
- OVATION is ~900 KB per fetch. Fine at 30-minute cadence, but worth watching in the run log.

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

- [ ] Drop the `Storming` Kp≥5 gate (3ce12be) — wrong model, hides quiet aurora Norway can see
- [ ] `internal/sources/ovation.go` — fetch, cache, validate the probability grid
- [ ] `internal/astro/ovation.go` — threshold the grid into bands per hemisphere, with the
      horizon skirt and gaps split into runs
- [ ] `internal/render` — night mask, feathered edge, aurora drawn through it
- [ ] `main.go` — OVATION first, Kp oval as fallback, ticker sentence from the drawn footprint
- [ ] Darkness test: an oval entirely in daylight produces no layer, no legend, no ticker
- [ ] Verify the mask survives camo (precedent: `clipPath` and gradients already render)

## Decisions

- **Threshold at 3% probability, not 10.** NOAA's own map draws its lowest contour low because
  the diffuse edge is real; 10% keeps only the bright core and would hide the nights when the
  glow is a faint arc on the northern horizon — exactly the nights this is meant to show.
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

## Open questions

- Does SWPC answer from a GitHub Actions runner? Unknown until the workflow runs — the
  fallback exists for exactly this.
- OVATION is ~900 KB per fetch. Fine at 30-minute cadence, but worth watching in the run log.

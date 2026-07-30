# ORP colour wheel (v2 M1)

> Phase A cheap win in `docs/v2.md`. Self-contained settings screen, no network, no flash gate
> beyond the widget cost below.

## Why

The pivot letter (ORP) is the one accent colour in the reading view. Picking it by
cycling a fixed palette and reading a hex code was clumsy, so it was pulled out of the v1
settings. The right way to choose a colour is to see it: a hue wheel with a live preview.

## Scope

In: a dedicated colour screen reached from settings, a hue wheel (or a saturation/value
field), a preview word rendered with the candidate colour, apply on confirm. Out:
everything already in v1 settings.

## Sketch

- Its own screen, not a settings row. Enter from a "Reading colour" entry.
- A colour wheel drawn with LVGL (`lv_colorwheel` if the build includes it, else a
  canvas-drawn hue ring). Live preview: a sample word with the pivot letter in the
  candidate colour, updated as the knob moves.
- Confirm writes `orp_color` to settings (already persisted); back cancels.

## Open questions

- ~~Does the LVGL build include `lv_colorwheel`?~~ Resolved 2026-07-04: the project pins
  LVGL `^9.5.0` and v9 dropped the v8 `lv_colorwheel` widget, so this is a canvas-drawn
  hue ring with angle hit-testing. Flash is not a gate: the factory slot is 4 MB with
  ~68% free after v1.
- Full RGB is overkill on a small strip; a hue ring at fixed saturation/value is likely
  enough and easier to operate with a fingertip.

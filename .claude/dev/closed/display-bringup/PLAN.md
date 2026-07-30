# Display bring-up (M4)

## Why

The screen is the second core piece the author asked to plan in detail. After M1 proves the
pipeline over serial, M4 puts a real frame on the 3.49" LCD. The reading experience is "focal
word + ORP," so the display is deliberately minimal — render-only, no timing logic (that's
`rsvp`). Tight word-to-word timing (≤ a few ms jitter at 600 WPM) is the worst failure mode to
avoid (`product.md`).

## Scope

In: `display` component (LVGL on AXS15231B), bundled font, `display_render` task, swap M1's
serial frame sink for the screen, idle screen-sleep hook (paused-only). Out: touch input + settings UI (M5),
progress bar / context words (post-v1).

## Approach

### `display` — `components/display/` (class `Display`)
- Render-only. Receive `Frame { word, orp_index, duration_ms }` (locked in M1) and draw the
  word with the ORP letter (`orp_index`, already chosen by `rsvp`'s pivot fraction) colored and
  positioned so that letter's center sits at the screen's **horizontal center** — the fixed
  eye-rest anchor. The word shifts around that anchor; it is not centered as a whole. No timing
  here — `rsvp` already decided when.
- **Expose the LVGL root screen** so `main` can compose UI on the same panel in M5 (home overlay,
  picker, settings). `display` owns panel init + the word-rendering label; it does **not** own the
  overlay/picker/settings widgets — those are `main` glue, keeping `display` render-only and the M5
  components leaves. Public surface adds an accessor for the active `lv_disp`/screen.
- Stack: LVGL on AXS15231B over QSPI, **landscape 640×172**.
- Render: one LVGL label; recolor span on the ORP glyph; offset the label so the ORP glyph's
  center lands on the screen's horizontal center (measure glyph advances up to `orp_index`). ORP
  color + font size default here but are read from the M5 `settings` store once it exists (the
  settings UI changes them live). The pivot fraction itself lives in `rsvp`, not here — `display`
  only honors the `orp_index` it is handed.
- Font: one bundled LVGL font, glyph subset = ASCII + Latin-1 + Latin-Extended-A + smart quotes
  / em-en dash / ellipsis / NBSP, sized for ~3.5 mm glyph height on this panel.
- Runs as `display_render` task (12 KB, `APP_CPU_NUM`), draining the same frame queue that fed
  the serial sink in M1. Swapping sinks touches only `main` wiring, not `rsvp`.
- **Degrade, don't panic** (rule 8 / I11): if the panel/touch probe fails, log and continue;
  the device must still boot.
- **Idle screen-sleep hook (paused only):** there is **no** inactivity timer while playing — an
  active reader is never auto-paused (reading needs no touches). The hook arms only when `rsvp` is
  paused: after an idle interval it dims, then turns the panel off; a touch wakes it. The bookmark
  is already flushed on pause (M2), so nothing is lost. M5 owns the timer and the HOME overlay it
  wakes to; M4 just exposes dim / off / wake. Deep-sleep power management is post-v1.

## Steps
- [x] Add the AXS15231B driver to `main/idf_component.yml`; regenerate + review
      `dependencies.lock`. Author approved. Pulled `esp_lcd_axs15231b` 2.1.0 (brings
      `esp_lcd_touch` 1.2.1, so touch is bundled) and `lvgl` 9.5.0.
- [x] Correct the board config: 16MB flash, octal PSRAM, custom `partitions.csv` (4MB app).
      Author approved. App now 933KB in a 4MB slot, 78% free.
- [x] Verify and record real LCD + touch pins in `board_pins.hpp`.
- [x] Configure LVGL via Kconfig (no `lv_conf.h`): RGB565, Montserrat 48, examples off.
- [~] Font: using built-in Montserrat 48 for now. The bundled Latin-Extended subset is
      deferred (ASCII renders correctly today); revisit when non-ASCII books need it.
- [x] Implement `Display`: panel init (degrades on probe failure), one label, ORP recolor + anchor.
- [x] `main` renders to the display. The serial frame sink was removed on review: the screen is
      always the target, and `show_frame` no-ops if the probe failed, so boot still never hard-fails.
      No separate frame queue: the pipeline tick task owns timing and calls `show_frame` under the
      display's LVGL mutex.
- [x] Expose `sleep()` / `wake()` (panel on/off) for the idle hook. Brightness dim deferred to M5.
- [x] `idf.py build` green; flashed; read a `.txt` on hardware: word renders upright, white with
      a red ORP letter, anchored left of center with edge padding. Stable, no panic.

## Decisions
- LVGL (not direct esp_lcd draw) — anti-aliased glyphs, easy ORP recolor + centering; the
  architecture already assumes it.
- Focal word + ORP only — no progress bar or context words in v1 (lowest jitter, product intent).
- `Frame` interface stays as locked in M1; M4 is a sink swap.
- **Landscape by software rotation, not hardware.** The panel runs native portrait (172×640);
  LVGL renders the 640×172 view and the flush rotates it (90°) and pushes it in DMA stripes
  through a small internal buffer. `swap_xy` did not map content into the visible area for this
  panel. This mirrors the vendor's working LVGL port.
- **Panel needs explicit init commands** `{0x11 sleep-out, 0x29 display-on}`. The driver's
  default init targets a different AXS15231B variant and leaves this 172×640 module blank.
- **ORP anchor at 38% from the left**, not screen center, so long words have room to the right;
  an edge-padding clamp shifts over-long words in so nothing touches the border.
- **No frame queue / leaf boundary kept.** `Display` takes a `(word, orp_index)` pair, not
  `rsvp::Frame`, so it stays a leaf. `main` translates and calls `show_frame`.
- **Render preferences are injected, not hardcoded** (review). `Display::init` takes a
  `DisplayConfig` (ORP/text/bg color, anchor %, edge padding) with defaults; the settings store
  fills it later. The layout geometry was inlined back into the component for cleaner code, which
  dropped its host tests, an accepted trade for keeping the display in one place.

## Open risks (resolved)
- **New managed deps:** done. `lvgl/lvgl` 9.5.0 + `espressif/esp_lcd_axs15231b` 2.1.0 added and
  approved; lock reviewed. The AXS component exists (no vendoring) and bundles touch via
  `esp_lcd_touch` 1.2.1.
- **Flash budget:** resolved, not real. The chip is 16MB flash + 8MB octal PSRAM; stock config
  under-reported it as 2MB / 1MB app. Bumped to 16MB with a 4MB custom app partition. App 933KB,
  78% free. Framebuffer (172×640×2 ≈ 215KB) lives in PSRAM.
- **Backlight = GPIO8 trap:** the backlight enable is the same GPIO8 rail powering SD/IMU/RTC,
  already high at boot. Idle dim/off must use the panel brightness command, never toggle GPIO8,
  or it cuts power to those peripherals. Carry this into the screen-sleep step.

## Verification
- On hardware: open a `.txt`, sweep WPM low→high. Word is centered, ORP letter colored at
  the screen's horizontal center, cadence matches `rsvp`, no visible jitter. While playing, leaving
  it untouched does **not** pause it. Once paused, idle → screen dims then off; touch wakes at the
  same word.

# Controls + book picker + settings (M5)

## Build design (2026-07-03) — the UI half, all in one branch

Reusing branch `feat/controls-picker-settings` (rebased on `development`); merges again later.
Building the whole UI half in one pass. Concrete design and resolved open questions below.

### Interaction model revision (2026-07-03, author feedback)

The author reworked the overlay after the first build. New model:

- **WPM is global**, not per-book: it lives in `settings` and is edited only in SETTINGS. It is off
  the reading overlay entirely. `open_path` always uses `settings.default_wpm()`; the bookmark's
  per-book WPM is no longer read. Bluetooth/Connect is also off the reading overlay (v2, menu only).
- **PAUSED overlay** = five **white icons, no background, at the top** of the screen, over the word:
  back-paragraph, back-sentence, **menu (home)**, forward-sentence, forward-paragraph. Nothing else.
- **Tap anywhere that is not an icon → resume** (play); the icons disappear (READING). A tap while
  READING pauses back to the icons.
- **MENU (start menu)** is a separate screen reached by the home icon: Picker + Settings + Resume.
  PICKER and SETTINGS are reached from MENU and their Back returns to MENU.
- **Forward navigation** (forward sentence / paragraph) is new; only rewind existed. It is done in
  the pipeline by draining words to the next boundary (text_stream stays unchanged, no new tests).
  Every nav action (back/forward) re-renders the shown word so the reader sees where play resumes.
- Progress robustness: flush interval shortened and a flush on entering MENU, so an abrupt reflash
  loses less. A hard reset still can't be caught; pausing or the power button checkpoints.

States are now MENU, READING, PAUSED, PICKER, SETTINGS. The sections below describe the first build;
where they say HOME bar / WPM ± / Connect pictograms, read the revision above.

**Substrate first (two things the plan assumed but that were never built):**
- **Display root exposure.** `display` gains `root()` returning its active LVGL screen so `main`
  composes chrome (a bottom pictogram bar, plus full-screen picker/settings overlays) as children
  on the same screen. `display` stays render-only: it keeps owning the word label; `main` only adds
  siblings and toggles their visibility under the display lock. Display also gains live re-apply
  (`set_orp_color`, `set_palette`, `set_reading_font`) that re-renders the last word, and `wake()`.
- **Runtime book-open.** The pipeline only force-opened one hardcoded book. Its open logic is
  refactored into `open_path(path, resume)`; a new `open_book(path)` posts a command so the tick
  task (sole owner of source/stream/engine) swaps books, seeks its bookmark, sets its WPM, renders
  the first word paused. The picker's list comes from `book_list()` — a mutex-guarded snapshot the
  tick task rebuilds on open (dir scan of the books dir merged with bookmark last_read/wpm), so
  `main` never touches pipeline-owned state across tasks.

**Backlight / dimming.** Backlight is bit-banged on GPIO42 (active low), on/off only. `power` gains
an LEDC channel: `set_brightness(0..255)` (duty inverted for active-low), with `set_backlight(bool)`
kept as full-on/off. This serves both the idle dim and the settings brightness field.

**Screen state machine (main/ui).** States HOME / READING / PICKER / SETTINGS. All controls are
LVGL-native (buttons + `LV_EVENT_CLICKED`) driven by the touch indev — not the raw tap handler —
so widget hit-testing is clean and there is no double-fire. The word label stays visible under HOME
and READING; PICKER/SETTINGS are opaque overlays that cover it.
- **HOME** — pictogram bar along the bottom (Settings, Picker, WPM −, WPM +, Rewind, Connect) over
  the resume word (already on the label). A central clickable above the bar = play → READING.
- **READING** — full-area transparent clickable; a tap → pause → HOME. No chrome.
- **PICKER** — opaque list from `book_list()` (in-progress first, then alphabetical; current marked),
  row tap → `open_book` → HOME; Back → HOME.
- **SETTINGS** — editable rows (default WPM ±, ORP color cycle, ORP pivot ±, brightness ±, night
  mode toggle, font size cycle) + an inert "Re-pair / Wi-Fi (v2)" row + Back. Back saves and
  live-applies (display palette + power brightness), → HOME.

**Idle timer (paused only).** An LVGL timer keyed off `lv_display_get_inactive_time`, active only in
HOME. Stage 1 dims (LEDC low), stage 2 turns the panel off + backlight off; a touch wakes to HOME.
Never runs while READING, so an active reader is never interrupted. Transitions are edge-tracked so
the timer only calls power/panel ops on a state change.

**Resolved open questions.**
- Rewind: **one pictogram = rewind sentence** (the common case). Paragraph rewind stays on serial `]`.
- Night mode vs brightness: **two separate controls.** Brightness = LEDC backlight level; night mode
  = warmer/dimmer text palette (swap text color), applied live via `set_palette`.
- Pictogram glyphs: **LVGL built-in `LV_SYMBOL_*`** (Montserrat 14 is compiled in), no icon font.
- Font size: wired through settings + UI + a `display` font hook, but only one reading font is
  compiled today, so the control persists and takes visible effect once the extra reading-font
  assets land (that font generation belongs to `dev/active/glyph-coverage`). ORP color, brightness
  and night mode apply live; **ORP pivot, like font size, takes effect on the next book open** (the
  rsvp engine reads the pivot in its constructor and has no live setter — a small follow-up if we
  want it live). Flash is not a constraint (4 MB app slot, ~0.8 MB used).

## Status (2026-07-03) — reopened, half landed

> **Update (2026-07-03, later):** the UI half is now built on `feat/controls-picker-settings`
> (see the Build design section above and the checked Steps). Build is green; flash/on-glass walk
> is the only thing left before this can close. The snapshot below is the pre-build state, kept for
> context.

M5 was merged (PR #10) but only the two **leaf components** shipped. The **UI half — the screen
state machine and every on-glass control — was never built.** Reopened so the remaining work is
tracked before M6 (home menu) leans on a state machine that does not exist.

**Landed:**
- `components/settings` — NVS store, load/save, typed fields, `SettingsError`.
- `components/controls` — AXS15231B touch as an LVGL indev, tap handler with debounce + cooldown,
  degrades to serial-only on probe failure.
- `main/ui/ui.cpp` — one bare reading view; a tap toggles `rsvp` play/pause. Auto-pause at
  end-of-book resets to paused.
- Boot wiring in `main/app.cpp`: NVS init, `settings.load()`, touch bus up before the die reset,
  `orp_color` fed into `DisplayConfig`, backlight lit on first frame.

**Not built (the whole UI half):**
- Screen **state machine** (HOME / READING / PICKER / SETTINGS) — none. `ui.cpp` has no screen
  concept, just play/pause.
- **HOME overlay** — resume word + pictogram row (Settings, Picker, WPM ±, Rewind, inert Connect).
- **PICKER** — no in-device book list; boot just resumes the newest bookmark.
- **SETTINGS screen** — no editor; nothing applies settings live.
- **Paused-only idle timer** (dim → off, wake to HOME) — none; backlight is on/off only, no dim level.
- Boot-target `current` pointer folded in from M7 — not added to `settings`.

**Wired vs stored-only settings:** `default_wpm` (pipeline fallback WPM), `orp_pivot` (rsvp engine)
and `orp_color` (display init) are consumed. `font_size`, `brightness`, `night_mode` are persisted
but **no code reads them** — they wait on the settings UI and a live-apply path.

**So the remaining M5 work is:** the `main/` screen state machine, the HOME / PICKER / SETTINGS
screens on the display root, WPM ± / rewind controls, live settings re-read, and the paused idle
dim→off timer. M6 (home menu) builds directly on the state machine, so that lands first.

## Why

M4 puts a word on the screen; M5 makes the device usable without a serial cable. The reader
boots into a **paused home overlay** showing the word they'll resume on plus pictograms
(settings, picker, WPM, rewind). Tapping the middle hides the pictograms and the words flash by;
tapping again pauses and brings the overlay back, from where the device is powered off with the
physical button. This is the interaction model the author asked for and it satisfies the v1 done
line: "a non-technical user can pick up the device, press play, and read for 20 minutes unaided."

## Scope

Depends on **M4** (display/panel + touch driver wired), **M2** (`bookmark.list()`, per-book WPM)
and **M1** (`rsvp` play/pause/set_wpm, `text_stream` rewind). In: a `settings` leaf (NVS store),
a `controls` leaf (touch → semantic events / LVGL input device), and the LVGL **screens**
(home overlay, reading view, picker, settings) composed in `main/`. Out: book intake / Wi-Fi
reprovisioning UI flow itself (M6 — M5 only places the "re-pair / Wi-Fi" entry point), IMU
gestures (post-v1), progress bar / context words (post-v1).

## Approach

Two leaf components plus `main/` glue. Components never include each other or `display`; `main`
wires touch events to screen transitions and reads/writes `settings`, `bookmark`, `rsvp`.

### State model (owned by `main/`)

A small screen state machine:

- **HOME** — paused overlay: one word rendered large, plus a pictogram row. The word is the
  resume word if the book has a bookmark; on a **freshly opened book with no bookmark, the first
  word is preloaded** and shown (not playing), so the reader knows where to focus before tapping
  play. Pictograms: Settings (gear), Book picker (library), WPM − / +, Rewind, and a **Connect
  (Bluetooth) pictogram that is a v2 placeholder** — it is laid out now but only logs "v2" in v1,
  since companion intake / pairing is v2. This is also the boot screen (boot resumes paused, never
  auto-plays). Power-off is the physical button, not a pictogram.
- **READING** — bare focal word frames drained from the `rsvp` queue (the M4 view). No chrome.
  A tap anywhere on the reading area → `rsvp.pause()` → HOME.
- **PICKER** — list of books from `/sdcard/books/` merged with `bookmark.list()` (in-progress
  state, newest first). v1 rows show the **title plus the in-progress ordering only**. Tap a row →
  open that book (swap parser, seek bookmark, set its WPM), return to HOME paused. Back returns to
  HOME. **Word-based progress % and estimated time-to-finish are v2**, not v1: both need a total
  word count per book, which v2's companion app computes once at upload and ships as `total_words`
  metadata alongside the bookmark. v1 has no upload step (SD sideload), so it shows no % / ETA.
- **SETTINGS** — editable fields, persisted to `settings` (NVS). Back returns to HOME.

From HOME: tap center / play → READING; tap Settings/Picker → that screen; WPM ± and Rewind act
in place and stay on HOME.

**Boot target (book vs HOME).** Whether boot lands on HOME or resumes a book is an explicit
persisted pointer, not "newest bookmark" — so finishing a book and powering off returns to HOME,
not the finished book. Design and build this with the state machine here. See
`dev/proposed/boot-target-state/PLAN.md` (the `settings` store gains a `current` book pointer:
`set_current` on open, `clear_current` on end-of-book / close-to-HOME, `current()` read on boot).

### `settings` — `components/settings/` (class `Settings`)

- **NVS-backed** (`nvs.h` / `nvs_handle.hpp`, no new dep). Holds the reading preferences that the
  v1 settings UI changes: **default WPM**, **ORP pivot %** (fraction of the word, default ~30%),
  **ORP color**, **font size**,
  **brightness**, **night mode**.
- **WPM split:** `settings` holds the *default* WPM for newly opened books. The HOME WPM ± changes
  the *current book's* WPM, which lives in the `bookmark` record (M2) — `main` writes it there, not
  to `settings`. Keeps "per-book WPM" and "default WPM" from fighting.
- **Settings are read, not hardcoded** (v1.md architecture note): `rsvp` reads the ORP pivot
  fraction and `display` reads ORP color / font size / brightness as injected config. M5 wires the store in;
  M1/M4 already take these as config, so no rework there.
- **API (tentative):** `load() -> expected<void, SettingsError>` (first boot = defaults, not an
  error); typed getters/setters per field; `save() -> expected<void, SettingsError>` (NVS commit).
  Setters mutate in RAM; `save()` is called on leaving the settings screen.
- **Error:** `struct SettingsError { enum class Type { NVS_OPEN, NVS_READ, NVS_WRITE }; std::string
  context; std::string to_string() const; }` (E2).
- **Defaults are placeholders where they touch secrets** (none here — pure UI prefs), so plain
  `constexpr` defaults: WPM 300, ORP pivot fraction 0.30 (M1), etc.

### `controls` — `components/controls/` (class `Controls`)

- Owns the **touch input** path for v1: register the AXS15231B touch (the same managed component
  M4 adds for the panel) as an **LVGL input device** so LVGL hit-tests pictograms natively.
- **Degrade, don't panic** (rule 8 / I11): if the touch probe fails, log and continue — the device
  still boots and renders; serial commands (M1) remain the dev fallback.
- Exposes a thin semantic hook for the one non-widget gesture: **tap on the reading area** (used to
  pause). Pictograms are ordinary LVGL buttons whose callbacks live in `main`.
- IMU gestures are explicitly **out** (post-v1); `controls` is structured so an IMU source can be
  added later without reshaping the touch path.
- **API (tentative):** `init() -> expected<void, ControlsError>` (registers the indev),
  `lv_indev_t* indev()` for `main` to attach. `ControlsError { enum class Type { PROBE_FAILED,
  INDEV_REGISTER }; ... }` (E2).

### `main/` glue (the screens + wiring)

- Build the four LVGL screens on the root surface `display` exposes (see M4 ripple below). Screens
  are `main`'s responsibility (glue), keeping `display` render-only and both new components leaves.
- Wire pictogram callbacks: Settings → SETTINGS screen; Picker → PICKER; WPM − / + →
  `rsvp.set_wpm(current ± WPM_STEP)` (continuous, clamped to `WPM_MIN`/`WPM_MAX` from M1) + write
  the current book's WPM to `bookmark`; Rewind → `text_stream.rewind_sentence()`
  / `rewind_paragraph()` (one pictogram cycles, or two pictograms — decide at layout).
- **Idle screen-sleep — paused only** (M4 exposes the dim/off hook): there is **no** inactivity
  pause while playing. Reading untouched is expected, so an active reader is never interrupted —
  pausing is a tap or end-of-book. The idle timer runs only in the **paused** state (HOME): after
  an interval it dims, then turns the panel off; a touch wakes back to HOME at the same word. The
  bookmark is already flushed on pause (M2), so nothing is lost. Full deep-sleep power management
  is post-v1.
- **Reprovision entry point:** the settings screen carries a "re-pair / Wi-Fi setup" row that, in
  M6, launches BLE pairing + Wi-Fi provisioning. In M5 it is a placeholder that logs "M6"; this
  replaces the architecture's superseded "BOOT long-press to reprovision".

## Steps
- [x] Scaffold `settings` (`skills/component-scaffold`): NVS load/save, typed fields, `SettingsError`.
      Landed. `components/settings` holds `default_wpm`, `orp_pivot`, `orp_color`, `font_size`,
      `brightness`, `night_mode` with load/save/commit. See "Wired vs stored-only" below: only the
      first three are read by a consumer; `font_size`/`brightness`/`night_mode` persist but nothing
      applies them yet (no settings UI, no live-apply path).
- [x] Scaffold `controls`: register AXS15231B touch as LVGL indev, degrade on probe failure,
      reading-area tap hook, `ControlsError`. Landed. `init_bus` + `start` + `indev()` + tap
      handler with debounce and cooldown; degrades to serial-only when the probe fails.
- [x] `main`: screen state machine (HOME / READING / PICKER / SETTINGS) on the display root.
      Built. `Ui` owns the four states, composes chrome as children of `display->root()`, and
      routes taps through LVGL widgets. A full-screen catcher plays from HOME and pauses from READING.
- [x] HOME overlay: resume word + pictogram row; wire Settings / Picker / WPM ± / Rewind callbacks.
      Built. Bottom bar with a WPM readout plus Settings / Picker / WPM − / WPM + / Rewind / Connect
      (Connect logs v2). WPM ± clamp locally and post to the pipeline; Rewind rewinds a sentence.
- [x] PICKER: scan books dir ∪ bookmark progress; open-on-tap (swap parser, seek, set WPM).
      Built. Rows come from `pipeline.book_list()` (in-progress first); a tap calls `open_book`,
      which the tick task swaps in paused. Back returns to HOME.
- [x] SETTINGS screen: edit fields, `settings.save()` on exit; `display`/`power` read live config.
      Built. WPM default, ORP pivot, brightness, ORP color, font size, night mode, plus an inert
      v2 re-pair row. Changes apply live (palette, brightness, font) and save on Back.
- [x] Paused-only idle timer → dim then screen off; touch wakes to HOME (no pause while playing).
      Built. An LVGL timer keyed off `lv_display_get_inactive_time` dims at 20 s and turns the panel
      off at 40 s, only in HOME; a touch wakes and is swallowed. `power.set_brightness` drives the dim.
- [~] `idf.py build` green; `flash monitor`: boot → HOME paused, tap to read, tap to pause, change
      WPM, rewind, open a second book from the picker, change a setting and see it apply live.
      Build is green (80% free). Flash pending: `/dev/ttyACM0` was busy at build time; the on-glass
      walk still needs a run once the port is free.

## Decisions
- Tap toggles READING ↔ HOME; all other actions are overlay pictograms (no swipe gestures in v1).
- HOME is the boot screen and the wake-from-sleep screen. No inactivity pause while playing — the
  idle dim/off timer runs only when already paused.
- `settings` = default WPM + reading prefs in NVS; per-book WPM stays in `bookmark` (M2).
- LVGL screens live in `main/` (glue); `display` stays render-only; `controls`/`settings` are leaves.
- `controls` owns only touch in v1; IMU is a later additive source.
- Fresh book (no bookmark) preloads and shows its first word on HOME, paused, so the reader has a
  focus point before tapping play.
- Connect (Bluetooth) pictogram is laid out in v1 but inert (logs "v2"); pairing/intake is v2.
- Picker rows are title + in-progress order only in v1; word-based progress % and ETA are v2,
  driven by `total_words` metadata the companion app produces at upload.

## Requires change to earlier plan (M4)
- `display` must **expose its LVGL root screen** so `main` can compose the overlay / picker /
  settings on the same panel, instead of privately owning the single word label. **M4 PLAN updated
  accordingly.** Also M4's idle hook is **paused-only** (dim→off, never pauses an active reader)
  and wakes to **HOME** (defined here).

## Open questions
- Rewind on the overlay: one pictogram that cycles sentence→paragraph, or two separate pictograms?
  Decide at layout against the 640×172 strip width.
- Night mode vs brightness: one combined control or two? Confirm what the panel backlight supports
  at M4 bring-up.
- Pictogram glyphs: reuse the bundled LVGL font's symbol set, or a small dedicated icon font?
  Lean on LVGL built-in symbols first to save flash; revisit if they read poorly at this size.
  Note: text glyph coverage (curly quotes, dashes, accented Latin) is a separate v1 task,
  see `dev/active/glyph-coverage/PLAN.md` — it covers the reading font, not the pictograms.

## Verification
- Boot with a bookmarked book: device shows HOME paused with the resume word + pictograms, no word
  motion until a tap. Tap → words flash; tap → pause back to HOME. WPM ± changes cadence and
  survives reboot (per-book). Rewind jumps to the right boundary. Picker lists books by title
  (in-progress first, no % / ETA in v1) and opens the tapped one at its own position/WPM. A settings change (e.g. font size, ORP color)
  applies live and persists across reboot. Left playing and untouched, it keeps reading (no
  auto-pause). Once paused and left idle → screen dims then off; a touch wakes to HOME at the same word.

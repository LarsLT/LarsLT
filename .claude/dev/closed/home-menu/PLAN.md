# Home menu (M6)

## Why

v1 dropped companion intake; books are sideloaded onto the SD card. The device needs a sensible
default state when no book is open, and a place to sit when the library is empty. The home menu is
that default state, and it is the hub the boot-target-state pointer (M7) falls through to.

## Scope

Depends on M4 (display + LVGL) and M5 (touch controls, book picker, NVS settings, screen state
machine). In: a home screen themed like the reading page, the paused context strip on the reading
screen, and the bluetooth entry point. Out: real BLE provisioning (v2), tap-to-seek on context
words (parked in `dev/proposed/context-tap-seek/`), boot home-vs-resume logic (M7).

## Design (settled with the user)

### A. Home menu redesign, themed like the reading page
The menu drops the three text buttons (Books / Settings / Read) and becomes a minimal focal screen:
- Black background, one focal element, matching the reading look.
- Center: the current book title, rendered like a focal word. Tap center -> start reading it (play,
  go to READING). No current book -> a "pick a book" hint, and center tap opens the picker instead.
- Top-left: book-picker icon (opens PICKER).
- Top-right: settings icon, then bluetooth icon.
- Icons only, no text.
- WPM / color / brightness stay in the Settings tab, unchanged. The old "Re-pair" row leaves
  Settings; bluetooth is now the top-right home icon.

### B. Bluetooth button
No BLE provisioning exists in this branch (the Wifi component only connects to a Kconfig AP). For
v1 the icon just logs to serial; real BLE pairing / companion intake is v2. Keep the icon and a
serial log so the layout and the touch target are final now.

### C. Paused context strip (previous / next words)
When paused, show the words around the current one so the reader can read back and skip:
`[...prev words] CURRENT [next words...]`, as many as fit, crossing paragraph boundaries. The
current word keeps the ORP emphasis; surrounding words are dimmed. Display-only for now: center tap
plays from the current word, and the existing sentence / paragraph buttons still move the position
and refresh the strip.

- `TextStream::context_window(before, after)`: returns tokens around the current position without
  moving the cursor. Snapshots stream state, walks back using the paragraph-starts ring + the seek
  callback to cross earlier paragraphs, reads forward, then restores the cursor exactly. Peeking
  forward must not leave the position stale (see the seek-stale note).
- `TextPipeline::paused_context(before, after)`: runs it on the tick task, returns
  `{prev, current, next, current_orp}`.
- Render: `Display` stays single-word for READING. The strip is an LVGL overlay built in `main/ui`,
  visible only in PAUSED, reusing the reading font and palette. The top nav bar (44px) stays; the
  strip fills the band below it. On entering PAUSED and after each nav action, refresh the strip.

## Steps
- [ ] Home menu rebuild: center current-book label + tap-to-read, top-left picker icon, top-right
      settings + bluetooth icons, icons only, empty-library / no-book hint.
- [ ] Bluetooth action logs to serial (v2 stub); remove the Re-pair row from Settings.
- [ ] `TextStream::context_window(before, after)` + host unit tests (book start, book end,
      single word, paragraph crossings, cursor restored unchanged after the call).
- [ ] `TextPipeline::paused_context(before, after)` on the tick task, thread-safe snapshot to the UI.
- [ ] Paused context strip overlay in `main/ui`, shown only in PAUSED, refreshed on enter + on nav.
- [ ] Park tap-to-seek design in `dev/proposed/context-tap-seek/`.
- [ ] Build green; host tests pass; on device: menu center-tap reads the current book, empty SD
      lands on a hint, paused strip shows and updates on nav, bluetooth icon logs.

## Decisions
- Menu is a `main/` state-machine node rendered via `display`; `display` holds no nav state.
- Bluetooth is a serial-log stub in v1; real provisioning is v2.
- Context strip crosses paragraph boundaries and is display-only in v1; tap-to-seek is parked.
- Center tap on the home menu starts reading the current book (one step to read).

## Refinements (round 2, after on-device review)

Design approved from a to-scale mockup. Changes:
- **Settings persistence to SD.** Move the settings store from NVS to `/sdcard/settings.json`
  so preferences survive a reflash / erase-flash, next to the bookmarks. Missing card falls
  back to defaults for that boot.
- **Real font sizes.** Generate Montserrat reading fonts at 48 / 56 / 64 / 72 px with
  lv_font_conv; the font-size setting is a px number that steps through them, and reading can
  be bigger. Default bumped up from 48.
- **Aligned settings grid.** Fixed columns so every minus, value, and plus line up. The value
  sits centered between the minus and plus. Night mode is a switch.
- **ORP colour out of settings.** Removed the palette-cycle row; a colour wheel is parked in
  `dev/proposed/orp-color-wheel/` for v2.
- **Menu + touch.** Corner icons inset from the wall with bigger padded touch areas; settings
  and bluetooth spaced closer, not stretched to the corner. Reading stays white word, red ORP.

## Open questions
- Exact word count that fits the strip is a device-tuning value; start generous and clamp to width.

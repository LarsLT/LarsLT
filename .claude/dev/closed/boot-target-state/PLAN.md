# Boot target state: open to the current book or the home menu (M7)

> Now a committed v1 milestone (M7), landing right after the home menu (M6) in
> `dev/active/home-menu/`. References below to "the controls/picker/settings milestone" mean M5;
> the menu itself is M6. The optional early slice (finished-book → don't reopen) still stands.

---

## Settled spec and implementation (2026-07-04, built on `development`)

Built on `development`, not the `feat/home-menu` branch (that menu redesign is still in
review). `development` already has the MENU screen, `BookEntry.current`, `book_list()`, and
an NVS-backed `Settings` store, which is all this needs. Branch: `feat/boot-target-state`.

### Authoritative behaviour (from the user, overrides the older sketch below)

The device boots into exactly one of two states: **a book open**, or the **home menu**.

- Power off while in a book (READING or PAUSED) → next boot reopens that book, resumed.
- Power off while on the home menu → next boot opens the home menu.
- The picker and settings screens are **never** a boot target. They are reached from the
  menu, so leaving the device on them boots to the menu.
- No finished-book special case. End of book stays in the book (paused at the end); a reboot
  reopens it there. Simpler than the older "finished → menu" idea, and it is what the user asked.

### The pointer

A single persisted pointer: the **full path** of the open book, empty = "no open book → menu".

- **Storage: NVS**, in the existing `Settings` store (`development` persists settings to NVS).
  NVS, not an SD file, so the pointer is readable even when the SD card or the book is gone,
  which is exactly the "SD missing → home menu" case. Stored as the full path (not the
  sanitized id) so reopening is a plain `open_path(path)`; the id is derived where needed.
- `Settings` gains `current_book()`, `set_current_book(path)`, `clear_current_book()`. Set and
  clear write NVS and commit immediately (not via the settings-screen `save()`), so an abrupt
  power cut still remembers the target, and a book-open write never clobbers unsaved settings edits.

### Who writes the pointer (the UI is the state machine; it owns the transitions)

- Pick a book (`open_selected_book`) → `set_current_book(selected path)`.
- Resume from the menu (Read / `RESUME`) → `set_current_book(current book path)`. If nothing is
  open, go to the picker instead of a blank reader.
- Go to the home menu (Home / `OPEN_MENU`) → `clear_current_book()`.
- Boot open failure with the card present (see edge cases) → the pipeline clears the stale
  pointer. This is the only pointer write outside the UI. Two writer tasks, serialised by NVS.

### Boot flow (`TextPipeline::start`)

Replaces the temporary threebody force-open and the newest-bookmark / sample fallback.

1. `seed_sample_if_missing()` stays (the sample is a pickable book, no longer auto-opened).
2. Read `current = settings.current_book()`.
3. Empty → open no book. `rebuild_book_list()`, start the tasks, land on the menu.
4. Non-empty:
   - `opendir(BOOKS_DIR)` fails → SD/mount gone. **Keep** the pointer, land on the menu this
     boot; it resumes once the card is back.
   - Card present → load the bookmark as resume, `open_path(current, resume)`.
     - Success → book open, UI shows PAUSED.
     - Failure (file missing, or present but unreadable/corrupt) → **clear** the pointer, land
       on the menu.
5. Always create the queue and tasks, even with no book open, so the picker still works.
6. At the end, if no book opened, invoke `_no_book_handler` so the UI switches to the menu and
   lights the panel (no word frame will trigger the reading backlight path).

The pipeline must tolerate "no book open": `run_tick` guards `!_engine`, and `apply_command`
ignores book-driving commands until an OPEN arrives.

### UI

- `build_ui` picks the initial screen from the pointer: empty → MENU, set → PAUSED.
- New `_no_book_handler` (`on_no_book`) shows MENU and lights the backlight, correcting the
  rare "pointer set but boot open failed" mismatch and lighting the first-boot menu.

### Edge cases handled

1. Pointer empty (first boot / fresh NVS) → menu.
2. Pointer → book, file deleted/renamed (card present) → clear pointer, menu.
3. SD card missing / unmounted → keep pointer, menu this boot, resume when the card returns.
4. File present but corrupt / parser fails (card present) → clear pointer, menu.
5. NVS unreadable → `current_book()` is empty → menu (safe default).
6. Book finished → stays in the book, pointer unchanged → reboot reopens at the end.
7. UI initial state vs pipeline mismatch (pointer set, open failed) → `_no_book_handler` → menu.
8. Menu backlight (no word frame) → `_no_book_handler` lights the panel.
9. Resume from the menu re-sets the pointer, so power-off-while-reading reopens the book.
10. Picker / settings are never boot targets: the pointer is already empty when they are entered.
11. `BOOKS_DIR` changed between builds → stale path → file missing → clear, menu.

### Verification

- `idf.py build` green.
- Read a book, power off → resumes it.
- Read → Home → power off → boots to the menu.
- On the menu, power off → boots to the menu.
- Pick book A, read, power off → boots to A. Switch to B via the picker, power off → boots to B.
- Point at a book, delete it, reboot → menu (pointer cleared). Pull the SD, reboot → menu
  (pointer kept), reinsert, reboot → resumes.

---

## Original sketch (kept for context; superseded where it conflicts with the above)

## Why

Today the pipeline resumes the most recently read book (`bookmark.list()`, newest first). That
can't express "I finished this book and walked away." A finished book is still the newest
bookmark, so the next boot reopens it. The author wants the device to boot to where they left the
*app*: a book if one is open, otherwise the home menu. Finishing a book and powering off should
return to the menu next boot, not the finished book.

This needs explicit state ("current target = a book, or none/menu"), not a derived "newest" guess.
No amount of sorting bookmarks gives it, because the signal we need (the reader closed the book)
is not in the per-book records.

## Scope

Depends on the bookmark store (per-book records + resume) and the controls/picker/settings work
(the home menu, the NVS settings store, and the screen state machine). The visible payoff, booting
to a menu, needs the home screen, so this lands with that work, not before.

In: a persisted "current book" pointer (a `book_id`, or empty meaning "no current book → menu"),
boot logic that resumes it or falls through to the menu, set on open, cleared on finish or on an
explicit close-to-menu. Out: the menu UI itself (that is the home screen), multi-device sync.

## Approach

- A single persisted pointer: the current `book_id`, or empty = "no current book, show the menu".
- Storage: a field in the settings (NVS) store, not a standalone file, so session state lives in
  one place. The per-book bookmarks stay separate: they hold *where* in each book; this holds
  *which* book is live.
- Boot:
  - pointer names a book that exists → resume it (restore position + WPM, paused). Mid-book stays
    mid-book.
  - pointer empty, or names a missing or finished book → home menu. Until the menu exists, the
    headless harness falls back to the seeded sample.
- Transitions (owned by the `main/` state machine):
  - open a book (from the picker or a resume) → `set_current(book_id)`.
  - book reaches end of book → `clear_current()`, so it will not reopen.
  - user closes to home and powers off → the pointer reflects home (empty), not the last book.

## Sequencing: where this lands

**Build it with the controls/picker/settings milestone, after the display milestone.** Reasons:

- The "open to menu" behavior has no consumer until the home screen exists. In the headless
  serial harness there is no menu to show.
- The pointer is the entry and exit of the home ↔ reading ↔ picker state machine. Designing it
  alongside that machine keeps the model coherent.
- It naturally lives in the settings (NVS) store. Building a separate file for it now risks a
  throwaway that the settings work would relocate.

It is **not** a standalone next step right after the bookmark store, and **not** part of the
display milestone (the display is render only, it owns no state).

**Optional early slice.** The "finished → don't reopen" half can be pulled into the resume path
cheaply on its own: clear the resume target when a book reaches the end, and fall to the sample.
If reopening a finished book gets annoying on the bench before the menu exists, do just that.
Otherwise wait and do the whole thing together with the menu.

## Steps (rough sketch, not a commitment)

- [ ] Add `current` to the settings store: `set_current(book_id)`, `clear_current()`,
      `current() -> std::optional<std::string>`.
- [ ] Boot: read `current()`; resume if it names an existing book, else open the home menu.
- [ ] State machine: `set_current` on open, `clear_current` on end of book and on close-to-home.
- [ ] Replace the headless sample-stopgap path with the home menu once it renders.
- [ ] Host tests for the pointer. On device: finish a book, reboot, land on home, not the
      finished book.

## Decisions

- Explicit pointer, not `list()`-newest. Only explicit state can express "finished → menu".
- The pointer holds *which* book is live; the per-book bookmarks hold *where* in each. Two
  concerns, two records.

## Open questions

- Pointer in NVS (settings) vs a small SD file. Lean NVS with the rest of the settings, but a
  finished book that intake later deletes must clear the pointer cleanly either way.
- Definition of "finished": end of book reached, or only an explicit close-to-menu? Probably both
  clear the pointer. Confirm against the home-screen interaction model when that is designed.

## Verification

- Read a book partway, power cycle → resumes that book (pointer held).
- Read a book to the end, return to home, power off → next boot opens the home menu, not the
  finished book.
- Open book A, switch to book B, power off → next boot opens B. Switch back via the picker → A
  resumes at its own position.

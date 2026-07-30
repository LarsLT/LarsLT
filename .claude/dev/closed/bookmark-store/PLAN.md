# Bookmark store on SD (M2)

## Why

A power cycle must resume the right book at the right place and speed (`v1.md` done line).
Multiple books in progress and per-book WPM both fall out of a per-book record. Writing the
position every word at 600 WPM is 10 writes/s and wears the SD (known footgun) — persistence
must be batched.

## Scope

Depends on **M0** (SD mounted, `/sdcard/bookmarks/` exists) and **M1** (`StreamPosition`).
In: `bookmark` component, the per-book record, batched persistence, `main` wiring to save/resume.
Out: the picker UI that lists them (M5), settings (M5), sync (post-v1).

### Remove M1 scaffolding when this lands
M1 left a deliberate shim in `main/pipeline/text_pipeline.cpp`: a hardcoded `M1_BOOK_PATH`
opened on boot. **Replace it here** with the last-read book from the bookmark store (open the
book `list()` returns first, resume at its `StreamPosition`, paused). The `seed_sample_if_missing()`
shim stays until M6 intake; this milestone only removes the hardcoded *path*, not the seed.

## Approach

### `bookmark` — `components/bookmark/` (class `Bookmark`)
- **Leaf rule:** `bookmark` does **not** include `text_stream`. It stores the position as an
  **opaque string blob** that `main` gets from `text_stream::StreamPosition::serialize()` and
  feeds back via `deserialize()` on resume. `bookmark` never interprets it.
- **Record:** `{ book_id, std::string position_blob, uint16_t wpm, int64_t last_read }`.
- **Storage:** one small file per book at `/sdcard/bookmarks/<book_id>.json` via POSIX `fopen`
  (not the `sd` component). Per-book files mean multiple-in-progress is free and no big index
  gets rewritten. JSON via IDF's `json` (cJSON) component — already available, no new dep.
- **`book_id`:** sanitized basename of the book file (FAT-illegal chars stripped), e.g.
  `the-iliad.epub`. Human-debuggable; collisions are a non-issue on a single personal SD.
- **`last_read`:** `time()` (NTP-synced in M0 boot; if NTP failed it's degraded but still
  monotonic-ish — note in the record, don't block on it).
- **API (tentative):**
  - `load(book_id) -> expected<Record, BookmarkError>` (`NOT_FOUND` is a normal first-open case).
  - `update(book_id, position_blob, wpm)` — in-memory, marks the record dirty. Cheap; called often.
  - `flush() -> expected<void, BookmarkError>` — writes dirty records to SD.
  - `list() -> std::vector<Record>` — in-progress books, newest `last_read` first (M5 picker).
  - `remove(book_id)` — for when the phone deletes a book (M6).
- **Error:** `struct BookmarkError { enum class Type { LOAD_FAILED, WRITE_FAILED, NOT_FOUND,
  PARSE_FAILED }; std::string context; std::string to_string() const; }` (E2, carries book_id).
- **Untrusted input (S5):** a bookmark JSON on a removable SD is untrusted — validate keys exist
  and cap the blob length before use; a corrupt bookmark degrades to "start of book", never crashes.

### Persistence policy (the SD-wear fix)
- `main` calls `update()` freely (in-memory only). `flush()` runs **batched**: on a 10–30 s
  timer, and immediately on pause / stop / book-close. Never per word. Decide the timer home —
  a small `main` timer is simplest; `bookmark` stays I/O-on-demand.

### `main` wiring
- On open: `bookmark.load(book_id)` → if found, `text_stream.seek(StreamPosition::deserialize(blob))`
  and `rsvp.set_wpm(record.wpm)`; else start at the top with the default WPM.
- While reading: periodically `update(book_id, text_stream.position().serialize(), current_wpm)`.
- On pause/stop: `flush()`.

## Steps
- [x] Scaffold `bookmark`; record struct + `BookmarkError`; cJSON read/write of `<book_id>.json`.
- [x] `book_id` sanitizer; `write_file` creates `/sdcard/bookmarks/` if M0 has not (idempotent).
- [x] `load` / `update` (dirty) / `flush` / `list` / `remove`. (12 host tests)
- [x] `main`: per-word in-memory `update`, batched 15 s flush + flush on pause/stop/end-of-book,
      resume on open (pick newest from `list()`, `deserialize` → `seek`, restore per-book WPM).
      Removed the hardcoded `M1_BOOK_PATH`; the sample seed stays until M6.
- [x] `idf.py build` green (host + esp32s3). On-device verified: read to mid-book, bump to 350 WPM,
      pause, hard reset → resumed on the same word ("dozen") at the saved WPM.

## Status (2026-06-28)
Milestone complete. The `bookmark` leaf + host suite are green (12 tests) and the `main` wiring is
done and verified on hardware. The branch was rebased onto `development` after M1 merged (PR #3),
so it now sits on the merged pipeline; ready to push + open a PR. Multi-book independence is proven
by host tests (`two_books_keep_independent_state`); on-device book-switching is M5 (picker), so
only single-book resume is exercised on hardware here.

## Decisions
- Per-book JSON files keyed by sanitized basename (not one big index) — multiple-in-progress + no
  rewrite churn.
- Position stored as an opaque serialized blob bridged by `main` — keeps `bookmark` a leaf.
- Batched flush (15 s interval + on pause), never per word — SD wear.
- Flush runs inside the `rsvp_tick` task (interval checked in the tick loop), not a separate timer
  task, so `bookmark` stays single-owner and there's no race with the per-word `update`. This
  mirrors M1's single-owner pipeline rule.
- Boot resumes the newest bookmark via `list()`. This is interim: it can't express "finished a
  book, show the menu" (the finished book is still the newest). The explicit boot-target pointer
  that fixes it is deferred to M5, where the menu and `settings` store live. See
  `dev/proposed/boot-target-state/PLAN.md`.

## Requires change to earlier plan (M1)
- `StreamPosition` must gain `serialize() -> std::string` / `deserialize(std::string) ->
  expected<StreamPosition, …>` so `bookmark` can store it without including `text_stream`.
  **M1 PLAN updated accordingly.**

## Open questions
- Progress percentage for the picker (M5): store a fraction now, or compute from byte offset ÷
  file size at list time? Lean compute-at-list to avoid stale data; confirm in M5.

## Verification
- Read a `.txt` to mid-book at 400 WPM, pause, power-cycle → resumes the exact word at 400 WPM.
- Switch between two books; each keeps its own position and WPM. Corrupt a bookmark file by hand
  → that book degrades to start, no crash, others unaffected.

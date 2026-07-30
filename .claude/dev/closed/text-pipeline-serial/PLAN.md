# Text pipeline over serial (M1)

## Why

The text pipeline is the spine of ESRead (`architecture.md`): every book becomes a stream of
words the RSVP engine consumes on a tick. Nothing downstream — display, bookmark, intake —
works until this is solid. Building it serial-only proves timing, tokenization, and rewind
with zero display complexity. Author asked for detailed plans on the core readers first.

## Scope

Depends on **M0** (SD mounted, boots offline). In: `parser_txt`, `text_stream`
(rewind-capable + serializable position), `rsvp`, wired in `main/` to a serial frame sink.
Out: display (M4), bookmark persistence (M2), `.epub` (M3), touch triggers (M5). The rewind
*capability* lives here; the *control* that calls it is M5. The `StreamPosition` type and
parser `seek` are defined here so M2 can persist/restore without reshaping M1.

## Approach

Three leaf components (rule 3 — they never include each other). `main/` owns the wiring:
injects a paragraph source into `text_stream`, a word source into `rsvp`, and the frame queue.
Every fallible op returns `std::expected<T, E>` (rule E1).

### `parser_txt` — `components/parser_txt/` (class `ParserTxt`)
- Read a UTF-8 `.txt` from SD via **POSIX `fopen` on `/sdcard/...`** (does not include the `sd`
  component — leaf rule 3; `main` ensures SD is mounted first). Yield paragraphs. Pure: path
  in, paragraphs out; no RSVP knowledge.
- API (tentative): `open(path) -> expected<void, ParserError>`;
  `next_paragraph() -> expected<std::optional<std::string>, ParserError>` (`nullopt` = EOF);
  `seek(uint64_t byte_offset) -> expected<void, ParserError>` and `byte_offset()` — so a
  `StreamPosition` can restore where reading resumes (M2). Seeks land on a paragraph boundary.
- Normalize: strip UTF-8 BOM; CRLF→LF; blank line = paragraph break; collapse single soft
  newlines inside a paragraph to a space.
- Error: `struct ParserError { enum class Type { OPEN_FAILED, READ_FAILED, BAD_ENCODING };
  std::string context; std::string to_string() const; }` (rule E2 — carries the path).
- Untrusted input (S5): cap per-paragraph allocation; malformed file fails cleanly, no crash.

### `text_stream` — `components/text_stream/` (class `TextStream`)
- Tokenize paragraphs into words on **Unicode codepoint** boundaries. Lazy: pull the next
  paragraph only when the current one is exhausted (books never fully in RAM).
- Leaf wiring: constructed with an injected paragraph source
  `std::function<expected<std::optional<std::string>, …>()>` (fed by `ParserTxt` from `main`).
- `Token` carries what variable timing needs: word string, trailing-punctuation class
  (`NONE`, `COMMA`, `SENTENCE_END`), `paragraph_end` flag.
- **Rewind support (the M1-shaping addition).** Beyond a forward cursor + `seek(pos)`, the
  stream retains structure for *backward* seeks:
  - `rewind_sentence()` — jump to the start of the current (or previous) sentence.
  - `rewind_paragraph()` — jump to the start of the current paragraph.
  It tracks sentence-start and paragraph-start positions as it advances so these are O(1)
  back-jumps, not a re-parse. Decide the retention window (recent boundaries vs full index) by
  memory budget — a bounded ring of recent boundaries is enough for "back a sentence / this
  paragraph."
- **Serializable position (the resume currency, consumed by M2).** `position()` returns a
  `StreamPosition` — for `.txt`, the parser byte offset of the current paragraph + the word
  index within it. `seek(StreamPosition)` restores it (drives `ParserTxt::seek` then re-tokenizes
  to the word). Kept opaque to `bookmark`; only the pipeline interprets it. `.epub` reuses the
  same type with `(spine_index, byte_offset, word_index)` in M3.
- `StreamPosition` exposes `serialize() -> std::string` and
  `deserialize(std::string) -> expected<StreamPosition, …>` (a small versioned encoding, e.g.
  `"txt:<byte>:<word>"`). This lets `bookmark` (M2) persist position as an opaque blob without
  including `text_stream` — `main` bridges the two. (Added per M2.)
- API (tentative): `next_word()`, `peek()`, `position() -> StreamPosition`,
  `seek(StreamPosition)`, `rewind_sentence()`, `rewind_paragraph()`.
- Whitespace set: space, tab, newline, NBSP.

### `rsvp` — `components/rsvp/` (class `RSVPEngine`)
- Owns timing. Pull tokens, compute duration + ORP, emit frames.
- API (tentative): `set_wpm(uint16_t)`, `play()`, `pause()`, `reset()`. WPM is **continuous**:
  `set_wpm` accepts any value and clamps to a configured range — `WPM_MIN` / `WPM_MAX` / `WPM_STEP`
  as `constexpr` (R6; e.g. 100–1000 by 25, default 300). No fixed 200/400/600 presets; the M5
  overlay's WPM ± just nudges by `WPM_STEP`. The duration model already derives per-word timing
  from the base WPM, so any value in range works with no extra logic.
- **Starts paused.** Default state on construction/open is paused — no word motion until
  `play()`. This is what makes boot non-auto-playing (M0). In the M1 serial harness, a typed
  `play` / `pause` command drives it; on hardware it's a screen tap (M5).
- Duration model (constexpr, tunable — R6): base = `60000 / wpm` ms; × length factor for long
  words; + pause for `COMMA` (~1.5×), `SENTENCE_END` (~2×), `paragraph_end` (~2.5×).
- ORP index is a **fraction of the word**, not a length-bucket table: `orp_index =
  clamp(lround(pivot_fraction * (len - 1)), 0, len - 1)`. `pivot_fraction` is config — `constexpr
  ORP_PIVOT_DEFAULT = 0.30f` (Reedy-style, slightly left of center), range 0.0–1.0. The M5 settings
  UI exposes this as the "which letter is highlighted" percentage, so `rsvp` reads the fraction
  rather than baking it in. The display anchors this letter at the screen's horizontal center.
- Output: `Frame { word, orp_index, duration_ms }` to an injected sink — a FreeRTOS queue,
  drained by a serial printer in M1, the display task in M4. **Lock `Frame` now** so M4 only
  changes `main` wiring, not `rsvp`.
- Runs as the `rsvp_tick` task (4 KB, `APP_CPU_NUM` — architecture task topology).
- Leaf wiring: word source injected from `main` (the `TextStream`); no include of `text_stream`.

## Steps
- [x] Scaffold `parser_txt`; POSIX read, open + next_paragraph + normalization; add
      `seek(byte_offset)` / `byte_offset()`. (8 host tests)
- [x] Define `StreamPosition` (in `text_stream`'s public header); codepoint tokenizer, `Token`
      metadata, forward cursor.
- [x] Add `position()` / `seek(StreamPosition)` round-trip; verified resume lands on the same word.
- [x] Add rewind: sentence/paragraph boundary tracking + `rewind_sentence` / `rewind_paragraph`
      (bounded ring of recent sentence starts; chained rewinds go back further). (11 host tests)
- [x] Scaffold `rsvp`; config-driven ORP pivot fraction, duration model, `Frame`. (8 host tests)
- [x] `main/` wiring: `ParserTxt → TextStream → RSVPEngine → serial sink`, as a `TextPipeline`
      class under `main/pipeline/`.
- [x] `idf.py build` green (host + esp32s3); flashed and captured cadence across the WPM range
      on hardware, including punctuation/paragraph pauses and rewind-sentence.

## Deviations from the original sketch (intentional)
- **Tests live on the IDF `linux` target** under `test/host/`, one test component per production
  component (`parser_txt_tests`, `text_stream_tests`, `rsvp_tests`) + shared `test_support`. CI
  (`.github/workflows/host-tests.yml`) runs them and prints each result into the PR summary.
- **`rsvp` is decoupled from `text_stream`** (leaf rule): it takes its own `WordInput` (with a
  `Pause` class), and `main` maps `Token` → `WordInput`. `rsvp` does not include `text_stream`.
- **`rsvp::tick()` returns the `Frame`** (with `duration_ms`) rather than pushing to an injected
  queue; `main`'s `rsvp_tick` task owns the wait + sink. M4 still only changes `main` wiring.
- **`TextPipeline` is app glue, not a component.** It includes three components, which the leaf
  rule forbids inside a component, so it lives in `main/pipeline/` (a class, not loose functions).
- **Serial input** comes from the USB-Serial-JTAG RX FIFO directly (`usb_serial_jtag_read_bytes`),
  because `stdin` is bound to the primary UART console, not the native USB port.

## TEMPORARY scaffolding (remove in later milestones)
These are deliberate M1-only shims so the serial harness has something to play before bookmark
resume and intake exist. **Remove them when those land:**
- **Hardcoded book path** `M1_BOOK_PATH = "/sdcard/books/sample.txt"` in
  `main/pipeline/text_pipeline.cpp`. Replace with the last-book lookup in **M2** (bookmark-store).
- **`seed_sample_if_missing()`** writes an embedded sample book to SD on boot if absent. Remove
  once **M6** intake delivers real books. (See bookmark-store and companion-intake PLANs.)

## Decisions
- Reedy-style variable timing (not fixed WPM) — `rsvp` owns per-word duration; `text_stream`
  surfaces punctuation + paragraph boundaries to support it.
- Rewind set = back-sentence + paragraph-start only (no forward skip, no scrub).
- Leaf wiring via injected `std::function` sources, so components don't include each other.

## Open questions
- `text_stream` boundary retention: bounded ring of recent boundaries vs a full per-book index?
  Pick by memory budget once paragraph sizes are measured on real books. (M1 ships the bounded
  ring; revisit if real books need deeper rewind.)
- Long-word length factor curve and exact punctuation multipliers — tune against Reedy feel on
  hardware; start from the values above.

## Follow-ups (from M1 code review, deferred)
- `classify()` only recognizes ASCII `.?!,;:`. v1 char coverage (`v1.md`) includes the ellipsis
  `…` (U+2026) and smart quote closers (`"` `"` `»`), so a word ending in those currently skips
  its sentence pause. Add codepoint-aware sentence-end detection (with tests) when tuning timing
  on real books / in M3.

## Verification
- Put a sample `.txt` on SD (mix of long words, commas, sentence/paragraph breaks).
- Sweep WPM low→high (and the clamp ends); monitor shows correct cadence, visible pauses on long
  words / sentence ends,
  longer pause on paragraph breaks, and rewind jumps land on the right boundary.

## Acceptance: host unit tests (required)
First milestone with pure logic, so it carries the testing standard. `parser_txt`,
`text_stream` and `rsvp` ship host unit tests (ESP-IDF Unity on the `linux` target),
run in CI. Not done without them. Cover: tokenization (long words, punctuation,
paragraph breaks), WPM→delay math and clamps, ORP pivot, rewind boundary math.
Hardware/serial output stays a manual on-device check.

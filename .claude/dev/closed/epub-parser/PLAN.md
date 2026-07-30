# .epub parser (M3)

## Why

`.epub` is the second v1 format and the success-criteria proves it (`v1.md` done line). It
exercises the ZIP + XHTML path and the `StreamPosition` contract for a multi-file source, both
of which the later formats (post-v1 fb2/mobi) and the pipeline lean on. It plugs into the same
`text_stream` + `rsvp` from M1 with zero engine changes.

## Scope

Depends on **M1** (`text_stream`, `StreamPosition`, `rsvp`) and **M0** (SD). In: `parser_epub`
producing paragraphs from an `.epub`, byte-for-byte interchangeable with `parser_txt` behind
`text_stream`. Out: cover art, CSS/styling, footnotes/TOC navigation, DRM (non-goal).

## Approach

### Dependencies (verified — no new managed components)
- **`miniz` via `esp_rom`** (`components/esp_rom/include/miniz.h`) — ROM-provided inflate
  (`tinfl`). We hand-roll the ZIP **central directory** reader on top of it (locate entries,
  get compressed/uncompressed sizes + offsets, inflate one entry). `REQUIRES esp_rom`.
- IDF has **no XML parser** — `META-INF/container.xml` and the OPF are small and regular, so we
  do **minimal hand-parsing** (find `rootfile full-path`, then the `<spine>` `itemref` order
  mapped through `<manifest>` ids to hrefs). No expat, no new dep.

### `parser_epub` — `components/parser_epub/` (class `ParserEpub`)
- Same public shape as `parser_txt` so `main` swaps by file extension:
  `open(path)`, `next_paragraph() -> expected<optional<string>, ParserError>`,
  `seek(uint64_t)`, `byte_offset()`.

### Decisions made during build (deviations from the sketch above)
- **Interface mirrors `parser_txt` exactly** (`seek(uint64_t)` / `byte_offset()`), not a new
  `StreamPosition` overload. `TextStream` already drives its source through a single
  `uint64_t` byte offset, so `main` wires `.epub` with the identical lambdas it uses for `.txt`.
  No change to the shared M1 `StreamPosition` / `TextStream` / `rsvp`.
- **The `uint64_t` offset packs the multi-file position:** `(spine_index << 48) | paragraph_index`.
  The low 48 bits index the paragraph within the spine entry, not a byte offset. `TextStream`
  treats the value as opaque, so seek decodes spine + paragraph, re-strips that one entry, and
  skips to the paragraph: an exact resume with no per-paragraph byte bookkeeping.
- **Per-entry bound:** a spine entry inflates whole into a capped buffer; entries over the cap are
  logged and skipped rather than failing the whole book.
- **`parser_epub` is `esp_rom`-free.** Inflate is injected as a `std::function` so the component
  is a pure leaf that builds on the linux host target. `main` supplies the `esp_rom` `tinfl`
  adapter on device; host tests inject system `zlib` (raw deflate) for a real round-trip.
- Pure units, each host-tested: entity decode, incremental XHTML strip, ZIP central-directory
  read + entry extract, `container.xml`/OPF spine parse. `ParserEpub` orchestrates them.
- **Open:** read the ZIP central directory from `/sdcard/books/<id>.epub` (POSIX `fopen`, leaf
  rule). Read `container.xml` → OPF path. Parse OPF → ordered **spine** of XHTML hrefs.
- **Iterate:** inflate **one spine XHTML entry at a time** into a bounded buffer; never
  decompress the whole spine into RAM (known EPUB memory footgun). For a large XHTML entry,
  inflate in chunks and strip incrementally.
- **XHTML → paragraphs (streaming strip, no DOM):**
  - Skip `<head>`, `<style>`, `<script>` content.
  - Block elements (`</p>`, `<br>`, `<div>`, `<h1..6>`, `<li>`) → paragraph break.
  - Strip all other tags; decode entities (`&amp; &lt; &#8217; &#x2019;` …) to the Unicode
    codepoints `text_stream` already handles (Latin-1 + Latin-Ext-A + punctuation).
  - Collapse runs of whitespace; drop empty paragraphs.
- **`StreamPosition` for epub:** `(spine_index, byte_offset_within_xhtml, word_index)`, reusing
  the M1 type. `seek` restores the spine entry, re-inflates to the offset, re-tokenizes to the
  word. `serialize()` encodes the variant (e.g. `"epub:<spine>:<byte>:<word>"`).
- **Error:** reuse the `ParserError` shape (E2) with epub-specific `Type`s
  (`BAD_ZIP`, `NO_OPF`, `BAD_SPINE`, plus the shared open/read ones); `context` carries the
  failing entry path.
- **Untrusted input (S5):** an `.epub` is attacker-shaped data. Validate the central directory,
  cap entry sizes and the inflate output buffer, bound the spine count, check OPF/container keys
  exist before deref. A malformed `.epub` fails the parser cleanly — never crashes the device.

## Steps
- [x] ZIP central-directory reader over injected inflate; extract one named entry to a bounded buffer.
- [x] `container.xml` + OPF minimal parse → ordered spine of XHTML hrefs.
- [x] Streaming XHTML stripper → paragraphs (block-break rules + entity decode).
- [x] `ParserEpub` implementing the `parser_txt` interface + packed resume + `seek`.
- [x] `main`: pick parser by extension (`.txt`→`ParserTxt`, `.epub`→`ParserEpub`) via a
      `ParagraphSource` adapter; ROM `tinfl` injected on target. Rest of the pipeline unchanged.
- [x] Host suite green (entity decode, XHTML strip, ZIP round-trip via zlib, OPF spine, ParserEpub).
- [x] On-host smoke test against a real 2.4 MB `.epub` (The Three-Body Problem): 13,304 paragraphs
      in spine order, no leaked tags, seek round-trips. Opt-in via `ESREAD_TEST_EPUB`.
- [x] On-target compile clean (parser_epub + paragraph_source + text_pipeline) for ESP32-S3.
- [x] **On-device serial test** — verified on hardware. The Three-Body `.epub` streams over serial
      one word at a time with ORP index and per-word timing, reading in spine order. Surfaced three
      on-device-only bugs (all fixed): SD DMA can't read mapped flash (stage writes through RAM),
      FATFS short reads (read-until-full loop), FATFS bad backward-seek-after-EOF (POSIX lseek/read),
      and the ROM tinfl decompressor (~11 KB) overflowing the task stack (inflate on the heap).

## Decisions
- Hand-rolled ZIP + minimal OPF/XHTML parsing on ROM `miniz` — no new managed dependency.
- One spine entry inflated at a time — bounded RAM.
- Same `text_stream` / `rsvp` / `StreamPosition` as `.txt`; only the parser differs.

## Open questions (resolved)
- Entity coverage: settled on the common named subset (`amp lt gt quot apos nbsp mdash ndash
  hellip lsquo rsquo ldquo rdquo`) plus all numeric `&#...;`. Unknown references stay literal.
  Expand if real books surface gaps.
- Inflate ceiling per XHTML entry: 1 MB, oversized entries logged and skipped. Revisit against
  the real heap budget during the on-device test.

## Verification
- Three real `.epub`s (a novel, a non-fiction with headings, one with accented text) parse in
  reading order, no stray tags/entities, play cleanly across the WPM range (low / mid / high), and resume to the
  exact word after power-cycle.

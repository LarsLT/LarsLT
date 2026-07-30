# Glyph coverage: smart punctuation + Latin-1 on screen (v1)

## Why

Today the reading font renders plain ASCII only, so a curly apostrophe (`’`), curly quotes
(`“ ”`), en/em dashes (`– —`), an ellipsis (`…`), or an accented Latin letter (`café`, `naïve`)
either shows as a box or drops out. Real `.txt` and `.epub` books are full of these, especially
the curly quotes and apostrophes that publishers use by default. The reader sees broken words. v1
must render them.

This is a follow-up to the display milestone (M4, merged): the panel and font pipeline exist; this
extends the font's glyph set. It is a v1 item, sequenced after M4 and independent of M5/M6.

## Decision (resolved with author 2026-06-29)

Ship **ASCII + Latin-1 Supplement + a smart-punctuation subset of General Punctuation.** Not the
full Unicode BMP (too large for the flash budget, no English-book need), and not ASCII-only with
transliteration (loses the real glyphs). This covers effectively all English and Western-European
books at a small flash cost.

Codepoint ranges to include in the reading font:

- `U+0020`..`U+007E` — ASCII printable.
- `U+00A0`..`U+00FF` — Latin-1 Supplement (accented Latin: à á â ä ç é è ê ë ñ ö ü …, plus `«»`,
  `·`, `¿`, `¡`).
- Smart punctuation from General Punctuation:
  - `U+2018` `U+2019` — curly single quotes / apostrophe.
  - `U+201C` `U+201D` — curly double quotes.
  - `U+2013` `U+2014` — en dash, em dash.
  - `U+2026` — horizontal ellipsis.
  - (`U+2022` bullet optional, only if list rendering ever needs it.)

## Approach

- **LVGL renders UTF-8 natively when the font contains the codepoint.** So this is a font-asset
  change, not a text rewrite: regenerate the reading font with the ranges above. No transliteration,
  no mapping curly quotes down to ASCII.
- **Font generation.** Use the LVGL font converter (`lv_font_conv`, or the online tool) to emit the
  C array for the chosen face at the reading size(s), with the explicit range/symbol list above.
  Keep it as a generated source under the `display` component's font area, matching how the current
  font is bundled. Document the exact range string in a comment so it can be regenerated.
- **Parser / text_stream must pass the bytes through untouched.** `parser_txt` is UTF-8 passthrough
  already (it only strips BOM and CRLF); confirm it does not mangle multi-byte sequences when it
  splits paragraphs or enforces `MAX_PARAGRAPH_BYTES` (the cap must not cut a UTF-8 sequence in
  half). `.epub` text is already UTF-8 from the XHTML; confirm the same. The RSVP word splitter
  splits on whitespace, so multi-byte letters inside a word are fine, but verify the ORP pivot index
  counts **characters, not bytes** (the highlighted letter must land on a glyph boundary, not the
  middle of a two-byte char).
- **Flash budget.** Latin-1 + the punctuation subset is a modest addition over ASCII. Measure
  `idf.py size` before and after; record the delta. If the accented Latin half is ever too costly,
  the curly quotes / dashes / ellipsis are the non-negotiable core (most common in English books)
  and Latin-1 accents are the trimmable part.

## Steps
- [x] Pick the final range string; regenerate the reading font with `lv_font_conv` (ASCII +
      Latin-1 + the smart-punctuation codepoints), bundle the generated source in `display`.
      Generated `components/display/fonts/font_reading_48.c` (exact command in its header);
      built-in `lv_font_montserrat_48` dropped from sdkconfig.defaults.
- [x] Confirm `parser_txt` / `parser_epub` pass UTF-8 through and never split a multi-byte sequence
      (paragraph cap + word split). Found and fixed: `parser_txt`'s force-split cap could cut a
      UTF-8 sequence in half; it now finishes the sequence before splitting (host test added).
      `parser_epub` rejects oversized entries outright and decodes entities to UTF-8, clean.
      Bonus fix: `text_stream` trailing classification now skips curly closing quotes and
      guillemets and treats a trailing ellipsis as a sentence end, so smart-quoted dialogue keeps
      its pause timing (host test added).
- [x] Confirm the RSVP ORP pivot indexes by character, not byte, so the highlight lands on a glyph.
      Already correct: `rsvp` counts codepoints (existing test covers it) and `display` converts
      the character index to a byte offset with `utf8_offset`. No change.
- [x] `idf.py size` before/after: 809,237 → 800,985 bytes, net **-8,252**. The extended font is
      larger than the built-in ASCII set, but dropping `lv_font_montserrat_48` also drops its
      bundled FontAwesome symbol glyphs at 48 px, which more than pays for the new codepoints.
- [x] On device: a book using curly quotes, an em dash, an ellipsis, and an accented word renders
      every glyph correctly, with the ORP letter highlighted on the right character. Verified
      2026-07-03 with threebody.epub on the 3.49" board. Follow-up found on device and fixed in
      this branch: epub footnote reference markers glued onto the preceding word ("lives.3");
      the stripper now drops sup and EPUB3 noteref content.

## Decisions
- ASCII + Latin-1 Supplement + smart-punctuation subset; not full BMP, not transliteration.
- Font-asset change only; LVGL renders UTF-8 directly. Parser stays passthrough.
- Curly quotes / dashes / ellipsis are the must-have core; Latin-1 accents are the trim-if-needed
  part if the flash budget tightens.

## Open questions
- Which face/size(s) need regenerating — just the reading font, or also any settings/menu text
  font M5 introduces? Settle once M5's font usage is known; menu chrome may need fewer glyphs.
- Does the chosen base face have well-drawn Latin-1 accents at the reading size, or do some look
  poor on the 172 px-tall strip? Check at regeneration.

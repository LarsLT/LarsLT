# Bookmark resume fix + review polish

## Problem

Known v1 bug: `TextStream::seek()` does not update `_last_pos`, so right after a
resume seek `position()` still reads `{0,0}`. Any flush before the first played
word (pause, open menu, power off) overwrites the bookmark with the top of the
book. `rewind_sentence`/`rewind_paragraph` already patch `_last_pos` by hand
after their seeks, which is the same bug worked around at two call sites.

## Fix (root cause)

- `TextStream::seek()` sets `_last_pos = pos; _have_last = true` on success.
  `position()` now means "the word the next `next_word()` yields" after a seek,
  which is exactly what the bookmark wants to store.
- Delete the manual `_last_pos = target` patches in both rewinds (now redundant).
- `context_window`/`tokens_at` seek internally but restore via snapshot copy
  assignment, so they are unaffected.
- Host test: fresh stream, `seek(saved)`, `position() == saved`; rewind works
  immediately after a resume seek.

## Polish sweep (same pass, review request)

- `wifi.cpp`: replace `ESP_ERROR_CHECK` aborts in `init_once` with
  `std::expected` returns (new `INIT_FAILED` value). A failed Wi-Fi bring-up
  must not reboot the reader.
- Shared day/night text colors: `0xFFFFFF`/`0xC89050` duplicated in `app.cpp`
  and `ui.cpp` → named constants on `Ui`, used by both.
- `display.cpp` `render_word`: fixed 160-byte markup buffer → `std::string`
  (no magic size, no truncation of very long words).
- `parser_epub.cpp`: name the position packing (`spine << 48 | para`) constants.
- `bookmark.cpp` `list()`: magic `5` for `.json` → `ends_with` on a named
  extension.
- `controls.cpp`: name the two `100` ms I2C timeouts in `read()`.
- `ui.cpp`: name the leftover layout magic numbers (menu title margin, icon and
  picker-row hit pads, reuse `SET_HIT` for the night checkbox).
- `settings.cpp`: drop the `std::string(text).size()` copy, use `strlen`.
- `ntp.cpp`: name the 100 ms poll step.
- Kconfig symbol names (`CONFIG_wifi_ssid` etc.) stay lowercase: renaming them
  drops the values saved in everyone's gitignored `sdkconfig`. Not worth it.

## Verify

- Host tests: `test/host` on linux target, all green including the new one.
- `idf.py build` green.

## Status

- [x] Plan written
- [x] text_stream fix + test (`seek_updates_position_to_target`)
- [x] wifi expected conversion
- [x] constant sweep
- [x] builds + tests green (87 host tests pass, firmware builds), committed

# Known issues & footguns

Read this before adding similar code — these are failure shapes that have bitten this codebase or are obvious traps for ESRead's domain.

## Active / unresolved

(none currently tracked — add here when you hit one)

## ESRead-specific footguns

### UTF-8 word boundaries
Books contain non-ASCII whitespace and punctuation: `\xc2\xa0` (NBSP), `\xe2\x80\x94` (em-dash), `\xe2\x80\xa6` (ellipsis). Splitting on a raw byte `isspace()` corrupts these. Token boundaries must operate on Unicode codepoints (or at minimum, treat any byte with the high bit set as part of a word). Hyphenation across line breaks (`\n-\n`) is also book-specific.

### SD card write amplification (bookmark)
Writing a bookmark on every word at 600 WPM = 10 writes/second. SD cards die fast under that. Persist on:
- pause / stop
- a "dirty" interval (e.g. every 10–30 s)
- on shutdown / brownout if at all possible

Hold the running position in RAM and flush.

### EPUB memory pressure
A `.epub` is a ZIP. Decompressing the whole spine into RAM blows past internal RAM and pressures PSRAM. Stream chunks from the ZIP: open one XHTML file at a time, parse, emit, discard.

### PDF on-device is out of scope
PDF parsing requires substantial code and memory (font tables, content streams). Do it on the phone, ship plain text.

### Peripheral probe can fail at runtime
The 3.49" board has the display, touch, and IMU wired. A probe (I²C/QSPI init) can still fail on bad wiring or a flaky panel. Degrade and log, don't panic at boot — a failed display init must not stop the parser/intake path from running over serial.

### Wi-Fi-down is recoverable
Books arrive over BLE in the common case. Current `main.cpp` aborts if Wi-Fi fails — that's an inherited assumption from a previous project. Treat Wi-Fi-down as a soft failure when refactoring boot.

## Framework footguns (carry-over, still apply)

- **`esp_http_client_set_post_field` does not copy.** If you pass `std::string::c_str()` from a stack variable that goes out of scope, you get garbage in the request body.
- **Stack overflow in TLS tasks.** Default task stack is too small for mbedTLS. Bump to ≥ 8 KB for any task that does HTTPS.
- **`sdkconfig` accidentally committed.** It's gitignored, but if it ever gets staged you'll wipe the user's settings on pull. Always `git status` before commit.
- **Re-flashing without `idf.py build` first.** `idf.py flash` does build, but if you only ran `clean` and not `build`, you may flash a stale binary.
- **Backtrace doesn't decode.** Almost always a stale ELF — rebuild before re-running `monitor`.
- **New I2C driver vs legacy.** Use `driver/i2c_master.h`, not `driver/i2c.h`. SPI same story: `driver/spi_master.h`.

## Fixed (kept so the pattern doesn't recur)

### Build blocked: `wifi_provisioning` missing on IDF 6.0 (resolved 2026-05-18 by pinning IDF 5.5.4)
`idf.py build` failed at CMake configure:
```
Failed to resolve component 'wifi_provisioning' required by component 'espressif__arduino-esp32'
```
- **Cause:** IDF 6.0 renamed `wifi_provisioning` → `network_provisioning`. The pinned `arduino-esp32` still required the old name.
- **Resolution:** dropped from IDF 6.0 to **IDF 5.5.4** (lives at `/home/lqrslt/esp-idf-v5.5.4`). IDF 5.5 still ships `wifi_provisioning` AND already has C++23 / `std::expected` via GCC 13.2 / the new I²C / SPI master drivers — no functional regression.
- **Revisit:** when `arduino-esp32` ships a release built against IDF 6, switch back. No ETA — Espressif-side change.

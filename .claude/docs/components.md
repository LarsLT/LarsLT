# Components inventory

Existing components are real; planned components are listed so new work has a name + intended public surface to align with. Naming follows `.claude/rules/coding-style.md`.

## Existing (scaffolded)

| Component | Kind      | Purpose                                       | Public surface                          | Status        |
|-----------|-----------|-----------------------------------------------|-----------------------------------------|---------------|
| `wifi`    | namespace | STA connect via stored creds                  | `WIFI::connect()`                       | working       |
| `ntp`     | namespace | SNTP sync                                     | `NTP::sync()`                           | working       |
| `sd`      | class     | SD-card-backed read/write                     | `SD` class + `SDError`                  | scaffolded; expect refactor for book/bookmark use |

## Planned — text pipeline

| Component       | Kind  | Purpose                                                              | Tentative public surface                       |
|-----------------|-------|----------------------------------------------------------------------|------------------------------------------------|
| `parser_txt`    | class | Read UTF-8 `.txt` from SD; yield paragraphs                          | `ParserTxt::open(path)`, `next_paragraph()`    |
| `parser_epub`   | class | Unzip `.epub`; walk spine; emit text from XHTML                      | `ParserEpub::open(path)`, `next_paragraph()`   |
| `parser_fb2`    | class | Parse FictionBook XML; emit text                                     | `ParserFb2::open(path)`, `next_paragraph()`    |
| `parser_mobi`   | class | Decode `.mobi` containers; emit text                                 | TBD                                            |
| `text_stream`   | class | Tokenize paragraphs into UTF-8 words; cursor with peek/advance/seek  | `TextStream::next_word()`, `seek(pos)`         |
| `rsvp`          | class | Timing/WPM/ORP engine; emits render frames                           | `RSVPEngine::set_wpm()`, `play()`, `pause()`   |

## Planned — I/O

| Component         | Kind  | Purpose                                                       | Notes                                          |
|-------------------|-------|---------------------------------------------------------------|------------------------------------------------|
| `display`         | class | Render a single word (with ORP) on 172×640 IPS LCD via LVGL   | AXS15231B over QSPI                             |
| `controls`        | class | Input: capacitive touch; serial as a dev shortcut             | Emits play/pause/wpm-up/wpm-down events        |
| `imu`             | class | QMI8658 6-axis read; orientation + tap/flick gestures         | Over I²C                                        |
| `book_intake_ble` | class | GATT server: receives book file from companion app            | Writes to `/sdcard/books/`                     |
| `book_intake_wifi`| class | TCP/HTTP server: receives book file from companion app        | Larger payloads / faster than BLE              |
| `bookmark`        | class | Per-book position; persists periodically to SD                | Avoids writing on every word — see footguns    |

## Dependencies (managed)

From `main/idf_component.yml`:
- `espressif/arduino-esp32` `>=3.0.0,<4.0.0` (Arduino-as-component) — resolves cleanly on the pinned IDF 5.5.4. Would block on IDF 6.0 (still requires `wifi_provisioning`), see `known-issues.md`.
- `waveshare/esp32_s3_touch_amoled_1_75` — stale pin from the retired bench board; swap to 3.49" board support (or vendored AXS15231B driver) at display bring-up
- `espressif/esp_wifi_remote`, `espressif/esp_hosted` (only for p4/h2 — unused on S3 but in lockfile)

Will need to add when their respective components are built:
- An LVGL pin (`lvgl/lvgl` via component registry) for `display` on the 3.49"
- A board-support pin (or vendored Waveshare `esp_lcd` driver) for the 3.49" target — AXS15231B may not yet have a managed component; check at purchase time
- An epub/zip helper for `parser_epub` (or implement directly with miniz, which is in IDF)

Adding a new managed component requires updating `main/idf_component.yml` AND committing the regenerated `dependencies.lock`. See `.claude/rules/security.md` S6 for supply-chain hygiene.

## Cross-component data flow

```
SD ──books──> parser_<fmt> ──paragraphs──> text_stream ──words──> rsvp ──frames──> display
                                                ↑                    │
                                                └── seek(pos) ←──── bookmark ←── SD

book_intake_ble  ──┐
book_intake_wifi ──┴──> /sdcard/books/<id>.<ext>

WIFI ──STA up──> book_intake_wifi
NTP  ──wall clock──> bookmark.last_read (timestamp on persisted position)
```

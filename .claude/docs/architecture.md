# Architecture

## Layering

```
┌─────────────────────────────────────────────────────────────┐
│  main/   (app glue — orchestration, RTOS tasks, event wiring)
└─────────────────────────────────────────────────────────────┘
              │   uses ↓
┌─────────────────────────────────────────────────────────────┐
│  components/   (reusable leaf libraries)
│
│   Infrastructure:  wifi   ntp   sd
│   Text pipeline:   parser_txt   parser_epub   parser_fb2
│                    parser_mobi   text_stream   rsvp
│   Output:          display    (LVGL on the 3.49" LCD)
│   Input:           controls   (touch + IMU; serial as a dev shortcut)
│   Intake:          book_intake_ble   book_intake_wifi
│   State:           bookmark
└─────────────────────────────────────────────────────────────┘
              │   uses ↓
┌─────────────────────────────────────────────────────────────┐
│  ESP-IDF 5.5.4 + arduino-esp32 + managed_components
└─────────────────────────────────────────────────────────────┘
```

**Rule:** components never include each other. If two components need to coordinate, that coordination lives in `main/`.

Most components above are **planned**, not yet scaffolded. Only `wifi`, `ntp`, `sd` exist today. See `components.md` for status.

## Component anatomy

```
components/<name>/
├── CMakeLists.txt          # idf_component_register(...)
├── <name>.cpp              # implementation, TAG declared here
└── include/
    └── <name>.hpp          # public API only
```

Two flavors:
- **Class-based** — stateful (holds a handle, a parser cursor, a token stream): `Parser_Epub`, `RSVPEngine`, `Bookmark`, `BookIntakeBle`.
- **Namespace-based** — stateless utilities: `WIFI`, `NTP`.

Pick class when there is state to hold. Pick namespace when there isn't. See `.claude/rules/coding-style.md` for naming.

## Text pipeline (the spine)

This is the most important data flow in the project. Each book becomes a stream of words; the RSVP engine consumes that stream on a tick.

```
┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│ parser_<fmt> │ →  │ text_stream  │ →  │     rsvp     │ →  │   display    │
│ (file → raw  │    │ (tokenize,   │    │ (timing,     │    │ (single-word │
│  text)       │    │  normalize)  │    │  WPM, ORP)   │    │  render)     │
└──────────────┘    └──────────────┘    └──────────────┘    └──────────────┘
       ↑                                       ↓
   SD card                              bookmark (SD)
```

Design intent:
- **Parsers** are pure: file path in, sequence of paragraphs/sentences out. They do not know about RSVP.
- **`text_stream`** normalizes whitespace, splits into tokens (words), handles UTF-8 boundaries, exposes a cursor (advance, peek, seek). Books are not loaded fully into RAM — the stream pulls from the parser lazily.
- **`rsvp`** owns timing. Reads from the stream on its tick; computes ORP (Optimal Recognition Point) offset; yields render frames to `display`.
- **`display`** is render-only. Receives "render this word with ORP at column X" frames and draws them on the 3.49" LCD via LVGL.
- **`bookmark`** subscribes to position updates from `text_stream`, persists periodically (not every tick — see footguns).

## Book intake

```
Companion Android app
        │
        ├── BLE GATT (default path, smaller files)
        │     └── book_intake_ble  →  /sdcard/books/<id>.<ext>
        │
        └── Wi-Fi (LAN, larger files / faster)
              └── book_intake_wifi →  /sdcard/books/<id>.<ext>
```

Both intake components share the same output convention: a file appears on SD, optionally with a sidecar metadata JSON (title, author, format). The parser is invoked the next time the book is selected.

Wire protocol with the companion app is **TBD** — design lives in a future `dev/active/companion-protocol/PLAN.md`. Key constraints:
- BLE MTU is small; expect chunked transfer with resume.
- Pairing must be the only friction point. After first pair, "tap send" works.

## Hardware target

| Display    | Touch      | Controls          |
|------------|------------|-------------------|
| LVGL on 172×640 IPS LCD (AXS15231B, QSPI) | AXS15231B over I²C | touch + IMU gestures (QMI8658); serial as a dev shortcut |

One board: Waveshare ESP32-S3-Touch-LCD-3.49", connected now. Earlier docs split work across a 1.75" serial-only bench; that bench is retired.

## Error model

Every fallible function returns `std::expected<T, E>` where `E` is:
- a class-scoped `enum class ERRORS` for class components (`RSVPEngine::RSVP_ERRORS`)
- a `struct *Error { enum class Type; std::string context; std::string to_string(); }` for richer errors (file-path-bearing, e.g. parser errors)
- a namespace-scoped `enum class *_ERRORS` for namespace components (`WIFI::WIFI_ERRORS`)

`main.cpp` is the only place errors stop propagating — log and decide.

## Boot sequence

`main/main.cpp::app_main`:
1. `WIFI::init()` — sets up NVS, netif, event loop, Wi-Fi driver in STA mode. Hard-required.
2. `button_watcher_start()` — spawns the long-press watcher (see "Provisioning" below).
3. If `WIFI::has_credentials()` → `WIFI::connect_stored()`. On success, `online = true`.
4. Else (or on connect failure) → `WIFI::provision_and_connect()` — BLE comes up, blocks until creds + Wi-Fi join, BLE tears down.
5. If `online` → `NTP::sync()` (soft failure).
6. (Planned) `Bookmark::load()` + open last book + start `rsvp` task.

Wi-Fi failure is **never a hard abort** — the device runs offline (books from SD, controls work) and the user can retrigger provisioning via long-press.

## Provisioning (BLE on demand)

ESRead does not embed Wi-Fi credentials at build time. Credentials are sent over BLE from the companion app (or from Espressif's "ESP BLE Provisioning" Android app during bring-up) and persisted to NVS by `wifi_prov_mgr`.

**Radio policy: one at a time.** When provisioning, BT controller is up and Wi-Fi is in setup-only mode. When provisioning ends, `wifi_prov_mgr_deinit` frees the BT controller memory (via `WIFI_PROV_SCHEME_BLE_EVENT_HANDLER_FREE_BTDM`) and only Wi-Fi runs from that point. No BT/Wi-Fi coex.

**State machine:**

```
                       ┌──────────────────────────────┐
                       │  boot                        │
                       └──────────────┬───────────────┘
                                      │
                  NVS has creds?  Yes ┴ No
                  ┌───────────────────┴────────────────────┐
                  ▼                                        ▼
        ┌─────────────────┐                      ┌─────────────────┐
        │  connect_stored │                      │  provision_and_ │
        │  (STA only)     │                      │  connect (BLE)  │
        └─────────┬───────┘                      └────────┬────────┘
                  │                                       │
       got IP ────┤ disconnect (retry × N → fall through) │ phone sends creds
                  │                                       │ → NVS write
                  ▼                                       │ → BLE down
        ┌─────────────────┐                               │ → STA up → got IP
        │  app running    │◄──────────────────────────────┘
        │  (Wi-Fi up)     │
        └─────────┬───────┘
                  │ button long-press → erase_credentials() → esp_restart()
                  ▼
        ┌─────────────────┐
        │  reprovision    │
        └─────────────────┘
```

**Wire protocol:** `wifi_prov_mgr` with `scheme_ble`. Phone speaks Espressif's `protocomm` (protobuf over GATT). Security level **sec1** (Curve25519 ECDH + AES-CTR + PoP). PoP lives in Kconfig (`CONFIG_prov_pop`), default placeholder `"No PoP"`.

**Reset path:** `button.cpp` polls `CONFIG_button_gpio` (default 0 = BOOT). Hold for `CONFIG_button_long_press_ms` (default 5000) → `WIFI::erase_credentials()` + `esp_restart()` → device boots back into provisioning.

**Companion app path during dev:** install Espressif's "ESP BLE Provisioning" from Play Store; it speaks the same protocol. Custom companion app uses Espressif's `esp-idf-provisioning-android` library.

## Task topology (planned)

| Task             | Stack | Pin           | Job                                       |
|------------------|-------|---------------|-------------------------------------------|
| `rsvp_tick`      | 4 KB  | `APP_CPU_NUM` | Pull next token, schedule render          |
| `display_render` | 12 KB | `APP_CPU_NUM` | LVGL frame on the 3.49" LCD                |
| `intake_ble`     | 6 KB  | `APP_CPU_NUM` | GATT server loop                          |
| `intake_wifi`    | 8 KB  | `APP_CPU_NUM` | TCP/HTTP server                           |
| `wifi/ip`        | (IDF) | `PRO_CPU_NUM` | Wi-Fi driver (managed by IDF)             |

Cross-task signaling via FreeRTOS queues/event groups, never globals — see `.claude/rules/esp-idf.md`.

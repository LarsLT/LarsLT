# ESRead — what + why

## Pitch

ESRead is a small ESP32-S3 device that plays books one word at a time, **RSVP-style** (Rapid Serial Visual Presentation), mirroring the experience of the Android app **Reedy**. A long-strip IPS LCD (172×640, ~86 mm) shows a single focal word at a configurable WPM. In **v1**, books are loaded by SD-card sideload; a custom Android companion app that ships books over BLE / Wi-Fi is the **v2** north star, not part of v1.

## Who it's for

The author. Possibly also a less-technical reader in the author's life (e.g. a grandparent) — that constraint shapes the *reading* UX heavily: pick up the device, press play, read. In v1 the author loads books by SD card; the "drop a file in the companion app on your phone" upload model is the v2 goal for the less-technical reader. No SSH, no captive portals.

## What it does

| Surface             | Behavior                                                              |
|---------------------|-----------------------------------------------------------------------|
| Long-strip LCD      | Single focal word at the visual center; optional ORP highlight        |
| Touch / buttons     | Play / pause; WPM up/down; book picker (browse on-device library)     |
| SD card             | Holds books (sideloaded `.txt`/`.epub`); stores bookmarks (per-book position) |
| Companion app (BT/Wi-Fi) | **v2** — sends new books to the device; manages library on the phone |

**Library management (v1):** the device lists books on its SD card and lets you pick one to read. Curation (add, rename, organize, delete) is done by editing the SD card directly. Phone-side curation via the companion app is **v2**.

## File formats accepted (v1)

| Format        | Where parsing happens   | Notes                                                  |
|---------------|-------------------------|--------------------------------------------------------|
| `.txt` (UTF-8) | On-device              | Simplest path. First to land.                          |
| `.epub`        | On-device              | ZIP + XHTML; needs minimal HTML stripping              |
| `.fb2`        | On-device              | XML; structurally simple                               |
| `.mobi`       | On-device or phone-side| Older Amazon format; on-device feasible but complex    |
| `.pdf`        | **Phone-side conversion (v2)** | Needs the companion app to extract text and ship plain text; on-device PDF parsing is out of scope, so `.pdf` follows the companion into v2. |

v1 ships `.txt` + `.epub` only (see `v1.md`); `.fb2` / `.mobi` / `.pdf` are post-v1.

## Hardware target

One board: **Waveshare ESP32-S3-Touch-LCD-3.49"** — full UX. 172×640 IPS LCD long strip, capacitive touch, QMI8658 IMU, RTC, audio codec. Display + touch + IMU wired up. https://www.waveshare.com/esp32-s3-touch-lcd-3.49.htm?sku=32373 — in hand and connected (`/dev/ttyACM0`). Selection rationale: `.claude/dev/active/final-target-screen/PLAN.md`.

Early on, parser and intake bring-up ran serial-only on a 1.75" AMOLED bench with no display wired. That bench is retired; the 3.49" is the only board now.

## Why this exists (problem)

- The author already uses Reedy on Android; RSVP measurably increases read-through on long-form text.
- A dedicated, single-purpose hardware reader removes the phone's notification / app-switching pull during reading sessions.
- For a less-technical reader, an e-ink Kindle has too many menus; a phone has too many distractions. A device that does one thing — show the next word — has a lower failure mode for "I want to read for 20 minutes."

## Why hardware (not just a phone app)

- Single-purpose hardware = no other app can interrupt. The device's only job is the next word.
- A long-strip IPS LCD at typical reading distance gives a focal point that's hard to replicate alongside a phone's notification surface, and the horizontal real estate fits long words at a comfortable letter size (~3.5 mm tall) without scaling.
- Cheap enough ($30 board class) that the cost/value math works for a personal device.

## Current state (2026-05-18)

Working:
- **BLE-driven Wi-Fi provisioning** (`WIFI::provision_and_connect`) via `wifi_prov_mgr` + scheme_ble. Creds in NVS, no hardcoded SSID.
- Wi-Fi connect to stored AP (`WIFI::connect_stored`)
- Long-press BOOT button → erase credentials + reboot into provisioning
- NTP time sync (`NTP::sync`)
- `main/main.cpp` boot state machine (NVS check → connect or provision → NTP)
- `sd` component scaffolded
- **Build green** on IDF 5.5.4 (`/home/lqrslt/esp-idf-v5.5.4`).

Not yet implemented (planned):
- Parsers (`parser_txt`, `parser_epub`, `parser_fb2`, `parser_mobi`)
- Text-stream normalizer (`text_stream`) feeding the RSVP engine
- RSVP engine (`rsvp`) — timing, WPM, ORP calculation
- Display layer (`display`) — LVGL on the 3.49" LCD
- On-device book picker (browse + pick from `/sdcard/books/`)
- Home menu, IMU auto-rotate, off-button power-down, USB-C charging
- Bookmark store (`bookmark`) — per-book position persisted to SD
- (v2) Book intake (`book_intake_ble`, `book_intake_wifi`) + companion Android app — parked in `dev/proposed/companion-intake/`

## Planned next (rough order)

1. **`.txt` parser + RSVP timer over serial.** Prove the pipeline end-to-end before wiring the display.
2. **Bookmark on SD** — resume position survives reboot.
3. **`.epub` parser** — exercises the ZIP/XHTML path.
4. **Bring up the display** — add display component (AXS15231B over QSPI), IMU component (QMI8658 over I²C), render the RSVP stream.
5. **On-device book picker** — list `/sdcard/books/`, tap to open.
6. **Touch controls + WPM dial.**
7. **Home menu, boot target state, IMU auto-rotate, off-button power-down, USB-C charging** — the v1 device-polish milestones.
8. **(v2) Companion app + `.fb2` / `.mobi` parsers.**

## v1 contract

v1 shipped. The "final version" north star — companion-app intake, two-way sync, `.fb2`/`.mobi`
parsers, phone-side PDF, and the on-device polish — is scoped and sequenced in `v2.md`.

The exact v1 scope, locked design decisions, and the ordered milestone breakdown live in
`v1.md`. It extends the rough order above with features drawn out in planning: rewind
navigation (back-sentence / paragraph-start), per-book WPM, multiple books in progress, idle
screen-sleep, an on-device settings UI, a home menu, IMU auto-rotate, off-button power-down, and
USB-C charging. **Companion-app book intake** and the **two-way sync** (reading position, library,
reading stats) that builds on it are the post-v1 "final version" goal; v1 loads books by SD-card
sideload. The intake transport design is parked in `dev/proposed/companion-intake/`.

## Non-goals

- **Not a full e-reader.** No paginated layout, no font picker, no margin/leading config — RSVP doesn't paginate.
- **Library curation is off-device.** Device has a minimal picker (browse + open the next book). In v1, renaming, organizing, and deleting happen by editing the SD card; phone-side curation is v2.
- **Not DRM.** No Kindle/Adobe/Overdrive DRM handling. Bring DRM-free files (Project Gutenberg, Standard Ebooks, Calibre-converted, etc.); the device won't decrypt anything.
- **Not voice / TTS.** Text only.
- **Not multi-user.** One device, one reader.
- **Not cloud-managed.** No remote config, no telemetry.
- **Not PDF on-device.** Phone-side text extraction only.

## Constraints

- **Power:** USB or onboard battery (18650 / LiPo). v1 includes **USB-C charging** (enable the
  charge path via the TCA9554 expander), a battery **percentage indicator** (battery ADC), and an
  **off button** that powers the device down via the board's power latch. Deep-sleep power
  management is still post-v1. Because a dying battery can still cut power, persistence relies on
  periodic SD writes, not only a clean shutdown.
- **Network:** Wi-Fi stays active on boot for time sync and future use, but v1 has **no network
  book intake** — books are sideloaded onto the SD card. Companion-app intake over BLE / Wi-Fi is v2.
- **Latency:** word-to-word timing must be tight (≤ a few ms jitter at 600 WPM = ~100 ms/word). Display refresh and FreeRTOS scheduling both matter.
- **Privacy:** books on SD; SD is removable, treat physical access as a compromise vector. No remote upload paths in v1 = no token storage required.
- **Maintenance:** the author is the only operator (plus, possibly, one less-technical user who doesn't operate at all — they just read). Anything that requires a rebuild/flash to fix is acceptable for now.

## Success criteria

ESRead is "done enough" when:
- A `.txt` and a `.epub` both parse and play across the full (continuously adjustable) WPM range without visible glitches.
- A book copied to the SD card appears in the on-device picker and opens.
- Power cycle resumes at the previous bookmark, or lands on the home menu if no book is open.
- The screen auto-rotates right-side-up, the off button powers down, and the device charges over USB-C.
- The less-technical user can pick up the device, press play, and read for 20 minutes without asking for help.

## How this doc should guide decisions

When in doubt:
- **The next word matters more than anything else.** Timing jitter is the worst failure mode.
- **Phone is for management; device is for reading.** If a feature is about choosing or curating, it lives on the phone.
- **One step beats a menu.** Every additional tap to "start reading" is a regression.
- **Drop scope early.** If a parser or feature is fighting back, ship the simpler subset and document the gap.

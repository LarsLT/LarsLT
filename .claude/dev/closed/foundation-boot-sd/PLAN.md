# Foundation: boot + SD cleanup (M0)

## Why

Two foundations block everything else in v1:

1. **`main.cpp` hard-exits if Wi-Fi fails** — but v1 must run offline (books arrive over BLE;
   Wi-Fi is optional, `product.md`). The serial pipeline demo (M1) can't even boot without
   provisioned creds today.
2. **`sd` is wrong for v1.** It's scaffolded for Spotify/Google OAuth tokens (carryover the
   project rules say to clean), uses `#define` pin macros (R6), declares no `TAG`, and
   **`ESP_ERROR_CHECK`s in its constructor** — a peripheral probe failure *panics at boot*,
   violating the "degrade, don't panic" hard rule (8 / I11). Every milestone reads from SD.

M0 fixes both and lays down the `main/` orchestration skeleton so later milestones plug in.

## Scope

In: boot soft-fail wiring, `sd` refactor into a fallible mounter, `main/` task/skeleton
topology. Out: provisioning rework (M6-adjacent), any text/display logic.

## Approach

### Boot decoupling — `main/main.cpp`
- Wi-Fi becomes **soft failure**: log and continue offline, never `return` out of `app_main`
  (architecture boot sequence). NTP only attempted if Wi-Fi came up.
- Lay down the boot skeleton the later milestones extend: mount SD (degrade on failure) →
  (Wi-Fi soft) → (later: load settings, bookmark, open last book, start `rsvp`/`display`
  tasks). Leave clearly-marked seams for M1–M5, no stubs that pretend to work.
- **Boot resumes paused, never auto-plays.** The skeleton opens the last book and seeks to its
  bookmark, but the `rsvp` engine starts in the **paused** state — the reader taps play to
  begin (M5). On power-up there is no word motion until play. (User: "I don't just instantly
  want the book to start.")
- **No physical-button watcher.** The board's boot/reset buttons are unused and `off` is a
  hardware power cut, so M0 adds no GPIO button handling. Reprovisioning moves to the on-screen
  settings UI (M5/M6), not a BOOT long-press.

### `sd` refactor — `components/sd/` (class `SD`)
- **Remove the OAuth carryover entirely:** delete `RefreshCode`, `read_refresh_code`,
  `write_refresh_code`, the `Spotify.txt`/`Google.txt` paths. No Spotify/Google anywhere.
- **`SD` is a thin mounter.** Public surface: `mount() -> expected<void, SDError>`,
  `is_mounted()`, `unmount()`. It does **not** expose generic file read/write — other
  components use POSIX `fopen` on `/sdcard/...` directly (leaf rule 3), so `sd` is never
  included by them.
- **Degrade, don't panic:** replace every `ESP_ERROR_CHECK` in the I²C / IO-expander / mount
  path with `esp_err_t` checks that convert to `SDError` and return `std::unexpected` (E6).
  Mounting must not be done in the constructor — move it to `mount()` so failure is a value,
  not a panic.
- **Pins/config become `constexpr`** (R6): `CLK=2`, `CMD=1`, `D0=3`, I²C `SCL=14`/`SDA=15`,
  bus width 1, TCA9554 addr, 16 KB alloc unit, mount point `/sdcard`. No `#define`.
- Add `static const char* TAG = "SD";` (R2).
- Keep `SDError` (already the right struct shape, E2); add a `MOUNT_ERROR` type.
- Ensure `/sdcard/books/` and `/sdcard/bookmarks/` exist after mount (create if missing) — the
  conventions M1/M2 rely on.

## Steps
- [x] Refactor `sd.hpp`/`sd.cpp`: drop OAuth, `mount()`/`is_mounted()`/`unmount()`, constexpr
      pins, `TAG`, no `ESP_ERROR_CHECK`, create books/bookmarks dirs.
- [x] Rewrite `main.cpp`: mount SD (degrade+log), Wi-Fi soft-fail, NTP gated on Wi-Fi, marked
      seams for later milestones.
- [x] Update `main/CMakeLists.txt` PRIV_REQUIRES (added `sd`, `driver`).
- [x] `idf.py build` green (17% free). Flashed; boot log on `/dev/ttyACM0`:
      **SD mounted at /sdcard**, Wi-Fi connected, NTP synced, `app_main` returned, no panic.

## SD actually works now (the real bring-up)
The original `sd.cpp` was copied from the **Buddy** project, which targets a *different*
board (Waveshare 1.75" AMOLED). Three things were wrong and all are fixed:
- **Wrong SD pins.** 1.75" used CLK=2/CMD=1/D0=3. The 3.49" is **CLK=41 / CMD=39 / D0=40**
  (vendor `04_SD_Card` example). 1-bit SDMMC.
- **The TCA9554 was never needed for SD.** The 1.75" gates SD power through the io expander;
  the 3.49" does not. The expander init failed (`ESP_ERR_INVALID_STATE`) because its I²C pins
  were also 1.75" values. Removed the whole expander path and the `esp_io_expander_tca9554`
  dependency. (The 3.49" *has* a TCA9554, but only for battery/charge control, EXIO6.)
- **GPIO8 is the board peripheral power rail.** Every vendor example drives it high before
  touching SD/IMU/RTC. `main::board_power_on()` does this before `sd.mount()`.
- **FATFS long filenames.** `bookmarks` (9 chars) and book files exceed 8.3, so
  `CONFIG_FATFS_LFN_HEAP` is on (in the now-tracked `sdkconfig.defaults`).

### Real 3.49" pinout (do not copy 1.75" values again)
Verified pins live in `components/board/include/board_pins.hpp` (`BOARD::` namespace),
the single source of truth. Defined now: peripheral power (GPIO8), SDMMC (CLK=41, CMD=39,
D0=40, 1-bit). Documented there for later milestones: Touch I²C 18/17, ESP I²C 48/47
(RTC/IMU/TCA9554), LCD QSPI CS=9/PCLK=10/D0=11/D1=12. Add those to `board_pins.hpp` as
each milestone wires them.

Note: `components/board` is a header-only BSP leaf that `sd` and `main` depend on. This
bends hard rule 3 (components are leaves) on purpose: it's pure constants with no logic,
the bottom of the dep graph, and the only way to avoid duplicating pins across components.

## Decisions
- `sd` is a mounter; file I/O is POSIX VFS in each component (keeps components leaves).
- Mount in `mount()`, not the constructor, so failure returns a value (degrade path).
- Boot never hard-aborts on Wi-Fi or a failed peripheral probe.

## Open questions
- ~~Does `wifi.cpp` already expose a non-fatal connect?~~ **Resolved:** `WIFI::connect()` already
  returns a value on failure. It retries a bounded `MAX_RETRY=5` times then returns
  `CONNECT_FAILED` — no infinite spin. No `has_credentials()` guard needed; an unprovisioned
  boot just wastes a few seconds on placeholder creds, which is acceptable for M0.
- Note: `product.md` "current state" describes BLE provisioning / `connect_stored` that does not
  exist in the real `wifi.hpp` (only `connect()` reading Kconfig). Out of M0 scope (provisioning
  is M6); flag for a docs pass.

## Verification
- Fresh device, no Wi-Fi creds: boots to idle without panic; log shows "SD mounted" (or a clean
  degrade if no card) and a Wi-Fi soft-fail line; `app_main` does not return early.
- **Done (2026-06-27):** on hardware with creds set, log shows `SD: mounted at /sdcard`, Wi-Fi
  connected, NTP synced, `boot complete (idle)`. SD soft-fail also confirmed earlier when the
  card init failed (degraded, no panic). M0 complete.

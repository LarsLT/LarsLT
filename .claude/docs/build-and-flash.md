# Build & flash

## One-time setup

ESP-IDF 5.5.4 lives at `/home/lqrslt/esp-idf-v5.5.4` (a separate IDF 6.0 install at `/opt/esp-idf` exists but is not used — see "IDF version" below). The IDF 5.5 venv is Python 3.11, but the system `python3` is 3.14, so plain `source export.sh` fails. Use **pyenv** to pin Python 3.11 before sourcing.

```bash
# Confirm pyenv has 3.11.x
pyenv versions  # expect 3.11.9 or similar

# Verify IDF
echo $IDF_PATH  # /home/lqrslt/esp-idf-v5.5.4  (after sourcing — see wrapper below)
```

User group for serial (no sudo on flash):
```bash
sudo usermod -aG uucp $USER    # Arch
sudo usermod -aG dialout $USER # Debian/Ubuntu
# log out + back in
```

## Day-to-day — the build wrapper

Every `idf.py` invocation must run under pyenv-3.11 with `export.sh` sourced. The shared wrapper:

```bash
PATH="/home/lqrslt/.pyenv/shims:$PATH" PYENV_VERSION=3.11.9 bash -c '
  . /home/lqrslt/esp-idf-v5.5.4/export.sh >/dev/null && idf.py build
'
```

Swap the final command for other operations:

| Action                          | Final command                  |
|---------------------------------|--------------------------------|
| Incremental build               | `idf.py build`                 |
| Flash + monitor                 | `idf.py flash monitor`         |
| Flash explicit port             | `idf.py -p /dev/ttyACM0 flash` |
| Menuconfig                      | `idf.py menuconfig`            |
| Binary footprint                | `idf.py size`                  |
| Remove objects (keep sdkconfig) | `idf.py clean`                 |
| Remove **everything**           | `idf.py fullclean`             |

⚠ `fullclean` wipes `sdkconfig`. You'll lose menuconfig settings (Wi-Fi creds, NTP, anything else). Make sure you have a backup or know what defaults look like.

Slash commands wrap the wrapper: `/build`, `/flash`, `/monitor`, `/menuconfig`, `/clean`.

## Target

```bash
# inside the wrapper:
idf.py set-target esp32s3
```

This is set once after a fresh checkout. Don't change it — partition table, board support, and managed components are S3-specific.

## Hardware

One board: **Waveshare ESP32-S3-Touch-LCD-3.49"** — 172×640 IPS LCD, AXS15231B display+touch, QMI8658 IMU, RTC, audio codec. In hand and connected; see `.claude/dev/active/final-target-screen/PLAN.md` for selection rationale.

**Board support pin is stale:** `main/idf_component.yml` still pins `waveshare/esp32_s3_touch_amoled_1_75` from the retired bench board. Swap it to the 3.49" board support (or a vendored AXS15231B `esp_lcd` driver) when display bring-up starts — confirm a managed component exists first, then regenerate and commit `dependencies.lock`.

## IDF version (5.5 vs 6.0)

The project is pinned to **IDF 5.5.4**, not the newer 6.0, because `arduino-esp32` still requires `wifi_provisioning`, which IDF 6.0 dropped (renamed to `network_provisioning`). IDF 5.5 still has both `std::expected` (via GCC 13.2 / C++23) and the new I²C / SPI master drivers, so there's no functional cost.

Revisit IDF 6.0 when `arduino-esp32` ships a release built against IDF 6. No fixed ETA — Espressif-side change.

## Troubleshooting

| Symptom                                       | First thing to check                                  |
|-----------------------------------------------|--------------------------------------------------------|
| `Failed to connect... Wrong boot mode`        | Hold BOOT, press RESET, release BOOT, retry          |
| `Permission denied: '/dev/ttyACM0'`           | User not in `uucp`/`dialout` group                    |
| `fatal error: 'xxx.h' file not found`         | Missing `PRIV_REQUIRES` in `main/CMakeLists.txt`      |
| `undefined reference to ...`                  | Component not linked — same as above                  |
| `region 'iram0_0_seg' overflowed`             | Too many `IRAM_ATTR` functions or `-Og` blowing IRAM  |
| `Brownout detector was triggered`             | USB power / cable, not code                           |
| Backtrace doesn't decode                      | ELF stale — rebuild before re-running monitor         |
| `Failed to resolve component 'wifi_provisioning'` | IDF 6.0 vs arduino-esp32 mismatch — see above     |

## Backtrace decoding

`idf.py monitor` decodes against the project ELF automatically. If you need to decode manually:

```bash
xtensa-esp32s3-elf-addr2line -e build/<project>.elf 0x40123456 0x40123458
```

Replace `<project>` with the name set by `project(...)` in the top-level `CMakeLists.txt` — `ESRead`. So the artifacts are `ESRead.elf` / `ESRead.bin` / `ESRead.map`.

## CI

No CI yet. If/when added, `idf.py build` in a container with `espressif/idf:release-v5.5` is the baseline.

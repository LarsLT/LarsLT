---
name: build-troubleshoot
description: Diagnose ESP-IDF build, link, or flash failures for ESRead. Trigger when the user pastes idf.py output containing "error:", "undefined reference", "region overflowed", "Failed to connect", or asks "why won't it build".
---

# Build / link / flash troubleshooting

## Step 1 — classify

Skim the first error (not the last; the rest cascade).

| Marker                                          | Class                                 |
|-------------------------------------------------|---------------------------------------|
| `fatal error: '*.h' file not found`             | Missing `REQUIRES` / `PRIV_REQUIRES`  |
| `undefined reference to`                        | Component not linked                  |
| `redefinition of`                               | Header included twice / no `#pragma once` |
| `error: 'xxx' is not a member of`               | Wrong IDF version or wrong header     |
| `region 'iram0_0_seg' overflowed`               | Too much `IRAM_ATTR` / wrong opt level |
| `region 'dram0_0_seg' overflowed`               | Too much static data                  |
| `Failed to connect... timed out`                | Bootloader mode / cable / driver      |
| `Permission denied: '/dev/ttyUSB*'`             | Group membership                      |
| `OSError: [Errno 71] Protocol error`            | USB hub / re-plug                     |
| `A fatal error occurred: ESP32-S3 chip ...`     | Wrong target — check `idf.py set-target` |

## Step 2 — verify with grep

For "missing include" or "undefined reference":
1. Find the symbol in `/home/lqrslt/esp-idf-v5.5.4/components/` and `managed_components/`.
2. Note the component name (the folder it lives in).
3. Add to the appropriate `CMakeLists.txt`'s `REQUIRES` (if used in header) or `PRIV_REQUIRES` (if only in `.cpp`).

## Step 3 — propose ONE fix

Do not suggest five things at once. Pick the highest-probability fix and propose it. If wrong, iterate.

## Step 4 — when stuck, capture state

Useful diagnostic commands:
```bash
idf.py reconfigure          # force CMake re-run
idf.py fullclean            # nuke build/ — last resort
cat sdkconfig | grep TARGET  # confirm chip
ls managed_components/      # confirm deps installed
cat dependencies.lock | head # confirm lockfile fresh
```

## Things to NOT do

- Do NOT run `fullclean` casually — wipes user's Wi-Fi creds and other menuconfig values from `sdkconfig`.
- Do NOT bump component versions to "fix" a build — pin compatibility is the project's choice.
- Do NOT edit `managed_components/` — it's regenerated.
- Do NOT `sudo` anything. If permission errors appear, fix groups instead.

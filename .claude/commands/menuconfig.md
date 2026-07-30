---
description: Open idf.py menuconfig
---

Run `idf.py menuconfig` via the pyenv wrapper (system python3 is 3.14, IDF venv is 3.11):

```bash
PATH="/home/lqrslt/.pyenv/shims:$PATH" PYENV_VERSION=3.11.9 bash -c '. /home/lqrslt/esp-idf-v5.5.4/export.sh >/dev/null && idf.py menuconfig'
```

This is interactive — tell the user to switch to the terminal; you cannot drive the TUI.

Common edits live under:
- `Wifi → WiFi SSID / WiFi Password` — STA credentials (will move to NVS later)
- `NTP → NTP Server / Timezone` — time sync
- `Component config → ESP HTTPS OTA` — when/if OTA is wired up
- `Component config → FreeRTOS` — stack size and tick rate
- `Component config → Bluetooth` — needed for the planned BLE book intake

After exit, `sdkconfig` is regenerated. Diff it before committing — most lines are auto-managed and should not be staged unless the user changed something deliberately.

---
description: Flash firmware to ESP32-S3 (auto-detect port) and optionally start monitor
---

Run `idf.py flash` through the pyenv wrapper (system python3 is 3.14, IDF venv is 3.11):

```bash
PATH="/home/lqrslt/.pyenv/shims:$PATH" PYENV_VERSION=3.11.9 bash -c '. /home/lqrslt/esp-idf-v5.5.4/export.sh >/dev/null && idf.py flash'
```

Port auto-detection is the default — do not pass `-p` unless the user has told you which port to use.

If the user said "flash and monitor" or `$ARGUMENTS` contains `monitor`, swap `flash` for `flash monitor` in the command above.

On failure:
- "Failed to connect" / "Wrong boot mode" → suggest holding BOOT, pressing RESET, releasing BOOT.
- "Permission denied" on `/dev/ttyUSB*` → user is not in `uucp` / `dialout` group.
- "No serial ports found" → board not plugged in or kernel didn't enumerate it.

Never `sudo` the flash command — fix permissions properly instead.

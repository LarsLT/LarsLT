---
description: Start idf.py serial monitor (Ctrl+] to exit)
---

Run `idf.py monitor` in the foreground via the pyenv wrapper (system python3 is 3.14, IDF venv is 3.11):

```bash
PATH="/home/lqrslt/.pyenv/shims:$PATH" PYENV_VERSION=3.11.9 bash -c '. /home/lqrslt/esp-idf-v5.5.4/export.sh >/dev/null && idf.py monitor'
```

Backtraces are auto-decoded against the project ELF in `build/` (name comes from top-level `CMakeLists.txt`'s `project(...)`). If the user pastes a crash from a stale ELF, remind them to rebuild before re-decoding.

Exit sequence: `Ctrl+]`. Do not background this — the user needs the live stream.

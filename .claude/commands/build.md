---
description: Build the project with idf.py
---

Run an ESP-IDF build for the user.

Steps:
1. Always invoke through the pyenv wrapper — system `python3` is 3.14 but the IDF venv is 3.11:
   ```bash
   PATH="/home/lqrslt/.pyenv/shims:$PATH" PYENV_VERSION=3.11.9 bash -c '. /home/lqrslt/esp-idf-v5.5.4/export.sh >/dev/null && idf.py build'
   ```
2. If the build fails, summarize the first error (not the whole output) and propose a fix grounded in the actual error — do not guess.
3. On success, report binary size and free flash from `idf.py size` if the user asks for follow-up.

Do NOT run `idf.py fullclean` unless the user explicitly asks. Incremental builds are the default.

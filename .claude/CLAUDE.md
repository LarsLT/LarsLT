# ESRead — ESP32-S3 RSVP reader

ESP-IDF C++ project. ESRead is an on-device **RSVP** (one-word-at-a-time) reader that mirrors the Android **Reedy** app, with a custom Android companion that ships books to the device over BLE/Wi-Fi. Reusable components under `components/`, app glue under `main/`.

**Read `.claude/docs/product.md` first** — it explains what ESRead is, who it's for, and what "done" means. Architecture and rules are downstream of product intent; don't reason about code changes without that context.

## Quick context

- **Hardware:** Waveshare **ESP32-S3-Touch-LCD-3.49"** — 172×640 IPS LCD (long strip, ~86 mm long edge), capacitive touch, QMI8658 IMU, PCF85063 RTC, ES8311/ES7210 audio codec with dual mic + speaker; https://www.waveshare.com/esp32-s3-touch-lcd-3.49.htm?sku=32373. **In hand and connected** (`/dev/ttyACM0`) as of 2026-06-25. Selection rationale: `.claude/dev/active/final-target-screen/PLAN.md`.
- **Framework:** ESP-IDF 5.5.4 + arduino-as-component (`espressif/arduino-esp32` >=3.0). IDF 6.0 was tried first but `arduino-esp32` doesn't yet support it (it still requires the removed `wifi_provisioning` component); 5.5.4 builds cleanly.
- **C++ standard:** C++23 (`std::expected`, `std::optional`, ranges)
- **Build:** `idf.py` CLI — see "Build flow" below. System `python3` is 3.14 but the IDF venv is 3.11, so plain `source /home/lqrslt/esp-idf-v5.5.4/export.sh` fails. Use the wrapper.
- **Style enforcement:** `.clang-format` (LLVM base, Allman braces, 4-space indent, 100 col)

## v1 scope (what we're building)

- **RSVP engine** — render one word at a time, configurable WPM
- **Parsers** — `.txt` (UTF-8), `.epub`, `.fb2`, `.mobi`; PDF is text-only via phone-side conversion (parsing PDF on-device is out of scope)
- **Text sources** — SD card and HTTP/companion-app push
- **Controls + bookmark resume** — capacitive touch on the 3.49". IMU gestures (tap/flick via QMI8658) are an option for secondary controls — design when display work starts. Serial commands stay useful as a dev shortcut.
- **Book intake** — **custom Android companion app** sends books over **BLE/Wi-Fi**. UX goal: a non-technical user (e.g. a grandparent) can receive a book without help.

There is **no OAuth, no Spotify/Google/Weather** in this project. If you see references to those in old code or docs, it's carryover from a previous project — flag and clean.

## Where to read before acting

Treat these as required reading depending on the task:

| Task type                       | Read first                                                           |
|---------------------------------|----------------------------------------------------------------------|
| Understanding the project       | `.claude/docs/product.md` (what + why + non-goals)                   |
| Any code change                 | `.claude/rules/coding-style.md`, `.claude/rules/error-handling.md`   |
| New component                   | `.claude/docs/architecture.md`, `.claude/skills/component-scaffold/` |
| Parser / RSVP / text pipeline   | `.claude/docs/architecture.md` (text pipeline section)               |
| Book intake (BLE / Wi-Fi)       | `.claude/docs/architecture.md` (intake section), `.claude/rules/security.md` |
| ESP-IDF / RTOS / drivers        | `.claude/rules/esp-idf.md`, `.claude/docs/esp-idf-cheatsheet.md`     |
| Build / flash / target switch   | `.claude/docs/build-and-flash.md`                                    |
| Touching credentials / TLS / BLE pairing | `.claude/rules/security.md`                                 |
| Multi-session feature work      | `.claude/dev/` (lifecycle: `proposed/` → `active/` → `closed/`)      |

## Hard rules (do not violate)

1. **Never invent ESP-IDF APIs.** If unsure, grep `managed_components/` or `/home/lqrslt/esp-idf-v5.5.4/components/`.
2. **All fallible ops return `std::expected<T, E>`** — no exceptions, no out-params, no sentinel returns.
3. **Components are leaves.** They do not include other components in the project. App glue lives in `main/`.
4. **Naming is non-negotiable.** See `.claude/rules/coding-style.md`. Do not "modernize" existing names.
5. **No new dependencies** without explicit approval. Check `main/idf_component.yml` first.
6. **Secrets via Kconfig only.** Defaults must read `"No Key"` / `"No ID"` style placeholders, never real values.
7. **Always write plans to disk before executing.** For any task that needs a plan (multi-step work, refactors, new features, investigations with more than ~3 steps), write the plan as `.claude/dev/active/<slug>/PLAN.md` BEFORE starting implementation. Update it as decisions are made. Do not keep plans only in chat — the user must be able to review and edit them in their editor. Feature notes move through `proposed/ → active/ → closed/`: park an idea we're not building yet in `dev/proposed/`; when a feature **lands** (implemented, PR reviewed, merged), move its folder to `dev/closed/` (with `git mv` — all three stages are tracked in git so work syncs across machines) after promoting any lasting insight to `.claude/docs/`. See `.claude/dev/README.md` for the lifecycle and `.claude/dev/active/README.md` for the template.
8. **Single target.** The Waveshare 3.49" LCD is the one and only board, connected now. Display, touch, and IMU are wired. Boot must not hard-fail if a peripheral probe fails — degrade and log, don't panic.
9. **Git.** Commit your own work in small units, one commit per unit, when done, directly on `main`. This repo is single-branch: always work on `main`, never create or switch to a side branch, no PR or merge workflow. One-line messages with a Conventional Commits type prefix (`feat:`, `fix:`) but no parentheses/scope, no emojis, no Claude co-author attribution. Never push — the user pushes. `git diff` allowed. See `.claude/rules/git.md`.

## Build flow

System `python3` is 3.14; the IDF 5.5 venv is 3.11 (`~/.espressif/python_env/idf5.5_py3.11_env`). `export.sh` matches the venv to the active Python, so we **must** pin Python 3.11 via pyenv first. Run every `idf.py` invocation inside this wrapper:

```bash
PATH="/home/lqrslt/.pyenv/shims:$PATH" PYENV_VERSION=3.11.9 bash -c '
  . /home/lqrslt/esp-idf-v5.5.4/export.sh >/dev/null && idf.py build
'
```

Same pattern for `flash monitor`, `menuconfig`, etc. — swap the final command. **Verification rule:** every non-trivial code change must end with `idf.py build` (and ideally `flash monitor` if hardware reachable) before reporting complete. A green build is the proof.

Last known-good build: 2026-07-04 on IDF 5.5.4. App binary ~1.3 MB, 68% free in the 4 MB factory partition (custom partitions.csv, 16 MB flash).

Slash commands available: `/build`, `/flash`, `/monitor`, `/menuconfig`, `/clean`, `/new-component`.

## Folder map (.claude/)

```
.claude/
├── CLAUDE.md          # this file — always loaded
├── README.md          # human-facing overview
├── settings.json      # shared (tracked) — agents, hooks, perms
├── settings.local.json# personal (gitignored) — extra perms
├── agents/            # specialized subagents
├── commands/          # slash commands
├── dev/               # feature notes: proposed/ → active/ → closed/
│   ├── proposed/      #   parked ideas (tracked)
│   ├── active/        #   in-flight work (tracked, syncs across machines)
│   └── closed/        #   landed + merged, kept as record (tracked)
├── docs/              # project knowledge Claude reads
├── hooks/             # project-local hooks (shared ones ship via claude-kit)
├── marketplaces/      # consumes the claude-kit marketplace
├── plugins/           # repo-local plugins (currently none)
├── rules/             # hard coding rules
└── skills/            # project-specific skills (shared ones ship via claude-kit)
```

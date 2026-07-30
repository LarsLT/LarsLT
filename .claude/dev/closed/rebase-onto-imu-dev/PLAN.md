# Rebase epub branch onto development (with IMU) and adopt the class convention

## Why
development merged IMU auto-rotate (PR #8). It also set a convention: app/feature
glue is a global CLASS, not a namespace (the IMU work dropped its namespace, and the
new AutoRotate glue lives in main/autorotate/ as a class). The epub branch must
rebase onto this and match that convention.

## Conflicts expected
- `main/main.cpp` / `app.cpp`: dev wires AutoRotate inline in main; epub has the slim
  main + app split. Fold `auto_rotate.start(display)` into the epub boot sequence.
- `main/CMakeLists.txt`: dev adds imu req + autorotate/auto_rotate.cpp source.

## Reconciled end state
- Keep slim `main.cpp` (PR #6 ask), boot glue in `app`.
- Fold AutoRotate into the boot sequence after display.init, like dev.
- Adopt the class convention for the epub-only glue:
  - `namespace APP` -> `class App` holding the component instances as members,
    board_power_on as a method, `run()` for the boot sequence. main.cpp keeps one
    `static App app;` and calls `app.run()`.
  - `namespace WIFI_SERVICE` -> `class WifiService` (owns its `Wifi`, in-flight flag,
    constants in-class, task trampoline), mirroring AutoRotate. Stays dormant: App
    owns it but does not wire it to the pipeline yet (PR #6: Wi-Fi not run yet).

## Steps
- [x] Backup, rebase onto origin/development, resolve to slim-main + AutoRotate folded.
- [x] Convert APP -> App class and WIFI_SERVICE -> WifiService class (own commit).
- [x] Build, host tests, clang-format, comment audit.

## Decisions
- App becomes a class too (not just wifi): the global-class convention applies to glue,
  and it also encapsulates the former loose globals into one owner.
- WifiService owned by App but unwired: keeps it ready (one line to enable) while
  honoring "Wi-Fi is not run yet".

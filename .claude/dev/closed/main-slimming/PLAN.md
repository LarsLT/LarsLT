# Slim main.cpp to just the entry point

## Why
PR #6 review: main.cpp does too much (loose globals, board power, the whole boot
sequence, and the on-demand Wi-Fi glue). The user wants main.cpp to hold only
`app_main`, everything else in its own file. Wi-Fi is not needed for the epub PR
and should not run yet, but must be a drop-in to enable later.

## Approach
- `main/main.cpp`: only `app_main`, which calls `APP::run()`.
- `main/app.{hpp,cpp}`: owns the lifetime objects (sd, display, pipeline),
  `board_power_on`, and the boot sequence in `APP::run()`. No Wi-Fi.
- `main/net/wifi_service.{hpp,cpp}`: the on-demand connect + NTP glue, owns its own
  `Wifi`. Exposes `WIFI_SERVICE::request_connect()`. Compiled but not called, so it
  stays valid; enabling later is one line in `APP::run()`. Own folder, like pipeline.
- Keep the pipeline's `set_connect_handler` seam; it is the wiring point. With no
  handler set the `w` key is a harmless no-op.
- main/CMakeLists.txt: add app.cpp and wifi_service.cpp to SRCS.

## Steps
- [x] Add `main/net/wifi_service.{hpp,cpp}` (move the on-demand glue out of main).
- [x] Add `main/app.{hpp,cpp}` (move globals + board power + boot sequence).
- [x] Reduce `main/main.cpp` to `app_main -> APP::run()`.
- [x] Update main/CMakeLists.txt SRCS.
- [x] Build, flash: reading ready at 1.57s; `w` is a no-op (Wi-Fi dormant, no crash).

## Decisions
- Wi-Fi stays compiled-but-dormant rather than deleted, so re-enabling is one line
  and CI keeps the code honest. The user asked for easy-later, not removal.
- Lifetime objects remain file-scope statics (dtors never run on hard power-off);
  moving them into the app glue TU is the fix for "not in main", the rationale
  comment travels with them.

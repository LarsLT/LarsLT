# Rebase epub branch onto development (with power/battery) and match its style

## Why
development merged the power/battery feature (Power rail latch + generic Button for
the off button, and a pipeline flush-on-power-off). The epub branch must rebase onto
it and adopt development's now-stricter conventions (constants inside the class,
classes in headers, one-line comments, single-responsibility splits).

## Conflicts expected
- `main/main.cpp`: dev wires power + button + boot wifi inline; epub slimmed it to
  `app_main -> APP::run()` with boot glue in `app.cpp`.
- `main/CMakeLists.txt`: dev adds power/button reqs; epub adds parser_epub/esp_rom +
  app.cpp/net/wifi_service.cpp/paragraph_source.cpp sources.
- `main/pipeline/text_pipeline.{hpp,cpp}`: dev adds `request_flush()`/`FLUSH`; epub
  adds `set_connect_handler()`/`CONNECT`, the ParagraphSource abstraction, and the
  display render. Keep all of them.

## Reconciled end state
- `main.cpp` stays the entry point only (PR #6 ask): `APP::run()`.
- `app.cpp` owns the boot sequence and folds in dev's power/button:
  board_power_on -> power.init -> power_button.init(long press -> power_off(flush))
  -> sd.mount -> display.init -> pipeline.start. No Wi-Fi at boot.
- Wi-Fi stays on-demand and dormant in `net/wifi_service` (PR #6 ask). Keep the
  re-entrant `Wifi::connect()` + `is_connected()`.
- `text_pipeline` keeps both FLUSH (power) and CONNECT (wifi) commands plus the
  ParagraphSource + display render.

## Steps
- [ ] Backup branch, rebase onto origin/development, resolve conflicts to the above.
- [ ] Review epub-only code against dev style: magic numbers in-class, classes in
      headers, one-line comments, single responsibility.
- [ ] Build, host tests, flash: reading works; off button flushes + powers down;
      `w` still dormant.

## Decisions
- Keep the slim main + app split rather than dev's inline main: the user asked for it
  in PR #6 and it is the more-refined structure.
- Keep Wi-Fi on-demand rather than dev's boot connect: PR #6 said Wi-Fi is not needed
  yet and must not run at boot.

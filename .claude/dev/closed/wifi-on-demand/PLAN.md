# Wi-Fi on demand (lazy connect, save power)

## Why
Wi-Fi at boot blocked the screen for ~15s and burned the radio retrying when no AP
was in range. The user reads on a 2000 mAh battery and is often somewhere with no
Wi-Fi at all. Reading needs nothing from the network; only NTP (clock) and future
book intake do. So: boot straight into reading, connect Wi-Fi only when the user
asks (to pair the phone / upload a book), and tolerate "no AP now, retry later".

Power context (estimates, no profiler): always-on associated Wi-Fi costs ~15-25% of
runtime; the worst case is retrying-with-no-AP (~120-200 mA sustained for nothing).
The LCD backlight dominates overall draw, so Wi-Fi is a real but secondary lever.

## Approach
- Boot no longer connects Wi-Fi or runs NTP. Order: power -> SD -> display ->
  pipeline. Screen and reading are live in ~1.5s. (Display reorder already landed.)
- Make `Wifi::connect()` safe to call repeatedly so on-demand connect can be retried
  after a failure (today it re-runs one-time IDF init under ESP_ERROR_CHECK and would
  abort on a second call). Split one-time init from the (retriable) associate+wait.
- Add `Wifi::is_connected()` so we don't re-attempt when already up.
- Trigger on demand through the existing console command path: press `w` to request a
  connect. The pipeline owns the only serial-input driver, so route it there via an
  injected `std::function<void()>` so the pipeline keeps no Wi-Fi dependency.
- `main` wires the handler to a one-shot background task (roomy stack) that calls
  `wifi.connect()` then `NTP::sync()`. A guard flag stops overlapping attempts.
- Later (with the upload feature, v2 intake): tear Wi-Fi down after the transfer to
  reclaim the idle draw, and replace the `w` key with a touch / BLE-wake trigger.

## Steps
- [x] Reorder boot so display + reading come up before any network (done earlier).
- [x] Remove the Wi-Fi/NTP block from `app_main` boot path.
- [x] Make `Wifi::connect()` re-entrant (one-time init guard, retriable associate),
      add `is_connected()`.
- [x] Pipeline: add a CONNECT command + `w` key + injected connect handler + help text.
- [x] main: on-demand connect task with an in-flight/connected guard; wire handler.
- [x] Build, flash, verify: boot complete at 1.7s with no radio; `w` starts a connect;
      a second `w` after a failure retries cleanly (no abort). Real association still
      needs valid Kconfig creds (build carried the `myssid` placeholder).

## Decisions
- On-demand over background-connect-at-boot: user picked "off at boot, on-demand only"
  for lowest power; matches frequently having no Wi-Fi.
- Route the trigger through the pipeline command loop rather than a second serial
  reader: there is one USB-Serial-JTAG FIFO and the pipeline already owns it.
- Inject a `std::function` instead of giving the pipeline a `Wifi&`: keeps the reading
  glue free of network deps and is the same pattern as the display render target.

## Open questions
- Teardown/stop for power after upload: deferred to the intake feature. Leave Wi-Fi up
  once connected for now.

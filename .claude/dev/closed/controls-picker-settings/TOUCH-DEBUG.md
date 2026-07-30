# Touch bring-up debugging log (AXS15231B)

## SESSION 8 (2026-07-03) — full reference re-read; the inversion is probably TWO bugs

All three references re-cloned and read end to end this session (V1 demo `10_LVGL_V9_Test`
incl. `i2c_bsp`/`lcd_bl_pwm_bsp`, V2 demo same example, rsvpnano complete 349 platform +
axs15231b display/touch drivers + Tca9554 + App/Input boot path), plus the managed
`esp_lcd_axs15231b.c` and our four components. New facts and corrections first.

### Corrections to earlier sessions

- **BT was never on.** `sdkconfig` has `# CONFIG_BT_ENABLED is not set`. The session-4
  suspect list carried a stale claim; radio bisection is moot.
- **The esp_lcd QSPI framing equals rsvpnano's.** `tx_param` sends `0x02<<24 | cmd<<8`
  (32-bit phase, params single-line), `tx_color` sends `0x32<<24 | cmd<<8` with QIO data;
  draw_bitmap sends CASET then RAMWR for y=0 / RAMWRC for y>0 and no RASET in QSPI mode.
  That is command-for-command what rsvpnano hand-rolls (CASET window, 0x2C first chunk,
  0x3C continue). The driver stack is NOT the delta; the V1/V2 demos use this exact
  managed driver on this exact board and work.
- **Backlight PWM-vs-static is a dead lead at full brightness.** rsvpnano at 100% writes
  LEDC duty 0 = constant LOW = electrically identical to our static GPIO42 low. The
  session-6 "fade is probably the backlight drive" hypothesis cannot explain anything at
  full brightness.
- **rsvpnano pulses GPIO21 on EVERY boot (cold and warm) and its touch works warm.** So
  "driving GPIO21 wedges warm touch" (session 7) cannot be right as stated. The build
  that produced that conclusion also held GPIO21 LOW as an output for the whole 2.8 s
  settle window before pulsing — a die held in (or glitched around) reset through its
  boot, then given only 30 ms before init. Dirty test, conclusion retracted.

### The reframe: two independent bugs, one per symptom axis

**Bug A — touch dead on warm boots is (probably) a tethering artifact, not a boot bug.**
Every warm-boot touch observation in sessions 1-7 was made with USB attached: post-flash
boots by definition, RST-button boots at the dev desk. DTR/USB-JTAG attach is a proven
touch killer on this die (session 3: dead from the first read when tethered; session 2:
even the vendor demo reads stuck 0xEA over a DTR link yet works standalone). Cold tests
inherently required unplugging, i.e. every cold test was untethered — and cold touch
works. **Warm-untethered touch on a no-SWRESET build has never been tested once.** The
"inversion" may simply be: tethered → touch dead (both warm cases), untethered → touch
alive (the cold case).

**Bug B — display dead on cold boots tracks the reset line, which we currently leave
floating.** HEAD never configures GPIO21 at all, so if this unit routes IO21 → RSTN (the
schematic's option resistors allow it, and the V1 demo — which pulses GPIO21 and never
touches the expander — passed the cold-drain A/B on this unit), the panel's reset input
floats all session. Both proven firmwares drive it solidly HIGH from early boot (V1 demo
even enables the pull-up in gpio_config) and pulse H30/L250/H30 immediately before panel
init, every boot. rsvpnano additionally reaches display init ~2.2 s after power-on
(serial wait), so a cold die is done booting when init lands. A floating RSTN explains
boot-order-dependent chaos on both halves of the die better than any single sequencing
theory tried so far.

### The combination no build has ever had

GPIO21 parked HIGH from early boot + cold-only settle + clean H30/L250/H30 pulse right
before init + minimal init + no SWRESET + no disp_on_off extras. Session builds each had
at most two of those. This session's build has all of them:

1. `board_pins.hpp`: `LCD_RST_OR_TE` → `LCD_RST` (GPIO21), documented as
   park-high-then-pulse.
2. `app.cpp`: parks GPIO21 high (level first, then config, so it never drives 0) as the
   first hardware act of boot.
3. `display.cpp` `init_panel`: settle wait only when reset reason is POWERON/BROWNOUT
   (warm boots skip straight through), then the H30/L250/H30 pulse, then
   `esp_lcd_panel_init`. `esp_lcd_panel_disp_on_off(true)` dropped — no reference sends
   an extra DISPON; the init list already ends with 0x29.
4. `controls.cpp`: probe the touch half in `init_bus`, before the display half gets its
   reset+init, matching rsvpnano's boot order (it probes before display init on every
   boot; the old "poking it mid-boot can wedge it" comment was a guess rsvpnano
   disproves). The post-init probe in `start()` stays for pre/post comparison in logs.

### Verification protocol (MUST be untethered — this is the part we kept getting wrong)

Flash, then unplug from the PC and power from a wall charger or battery for ALL checks:

1. Cold: power fully off (unplug + power off, wait ~10 s), power on. Expect: boot word
   renders and HOLDS (no fade), tap toggles play/pause.
2. Warm: RESET (EN) button, still untethered. Expect: same. This is the never-run test
   that decides Bug A.
3. Only after 1 and 2, observe over the monitor and expect touch to possibly read dead
   there — that is the known DTR confound, not a regression.

If cold display STILL fails with this build: flash the V1 demo and cold-drain test it
watching the screen (nobody has ever confirmed ANY firmware rendering on a true cold
boot on this unit except the user's rsvpnano report). If the demo fails too, it is a
board trait; if it passes, the remaining delta to port is rsvpnano's raw-SPI path.

### RESULT (same day): CONFIRMED ON HARDWARE — display and touch work, cold and warm

User ran the untethered protocol on the f9c80e7 build: it works. Cold power-on renders
and holds, warm reset renders and holds, touch works. The two-bug reframe stands:

- Cold display was the floating/never-pulsed reset line (+ the cold settle). Fixed by
  park-high at boot start and the H30/L250/H30 pulse right before init.
- Warm "touch dead" never reproduced untethered on this build, consistent with the
  tether confound. Strictly, the every-boot pulse could also have cleared it; either
  way the shipped sequence is the one all working references use.

Keep for the future: judge touch with NO monitor attached, and never reintroduce
SWRESET (`esp_lcd_panel_reset`) or remove the park/pulse without re-running the full
untethered cold+warm protocol.

Open follow-ups, deliberately not in the fix commit:
- Boot blink (old word flashes ~80 ms, then black, then the boot word): enable the
  backlight only after the first real frame, with a fallback enable on pipeline failure.
- `DIE_BOOT_SETTLE_MS` 2500 could likely shrink (try 1500, then 1000); each step needs
  an untethered cold test.

## SESSION 7 (2026-07-03) — the EXIO5 reset is a no-op; the die reset is GPIO21 after all

Symptom with the session-6 tree flashed: flash -> screen works, touch dead. Cold power
cycle -> touch works, screen shows random colors that fade out, never a word. That is the
exact "panel never got a real reset, GRAM writes discarded" signature session 6 opened
with — meaning the EXIO5 pulse we switched to does not reset the panel on this board.

### Why session 6's "confirmed working" was a false positive

Session 6 verified right after flashing, i.e. warm boots. Warm boots coast on panel state
left by a previous real init in the same power session (session 6's own words). Cold boot
was never in the confirmation loop, and cold is where the EXIO5-reset build fails.

### Ground truth from rsvpnano source (cloned and read in full this session)

rsvpnano is confirmed working on this physical unit (user flashed it: display holds,
touch works, cold and warm). Its board code says:

- **Die reset is GPIO21 on BOTH revisions.** `rev1/` and `rev2/` differ in exactly one
  constant: the backlight PWM pin (8 vs 42). `kResetPin = 21` is shared. EXIO5 is never
  driven anywhere in the codebase.
- The TCA9554 exists and is used on both revs, but only for EXIO1 (backlight enable,
  raised after panel init), EXIO6 (battery hold), EXIO7 (audio, on demand).
- Reset sequence in `Axs15231b::init`: backlight PWM off -> GPIO21 H30/L250/H30 ->
  QSPI bus init -> the same minimal 5-command list (11/36/3A/11/29) -> EXIO1 high ->
  backlight PWM on. **No SWRESET.**
- Touch: probed over I2C before display init, polled every 20 ms from the main loop.
  Legacy Wire driver, 10 ms timeout, no INT pin, no reset pin.

So the session-6 claim "GPIO21 is TE, the reset is EXIO5" came from reading the V2 demo
repo, not from this hardware, and it does not match the firmware that actually runs on
this unit.

### The combination that was never flashed: GPIO21 reset + no SWRESET

Session 4 tested GPIO21 reset but still had `esp_lcd_panel_reset` (SWRESET) in the boot,
and SWRESET is what wedges the touch core. Session 6 removed SWRESET but also moved the
reset to EXIO5 (a no-op). rsvpnano's working combination — GPIO21 hardware reset,
no SWRESET, minimal init, backlight enabled after init — was never tried. Trying it now.

### Change this session

1. `board_pins.hpp`: `LCD_RST = GPIO_NUM_21` again (rsvpnano uses it on both revs).
   `LCD_RST_EXIO` removed; EXIO5 is left completely untouched, like rsvpnano.
2. `display.cpp`: GPIO21 pulse H30/L250/H30 between panel create and panel init, owned
   by the display again. `DisplayConfig::reset_panel` callback removed.
3. `power.cpp`: `reset_lcd` and the EXIO5 parking removed. Backlight handling unchanged
   (EXIO1 + GPIO42 low = on, enabled by the app after display init).
4. `app.cpp`: the `reset_panel` wiring removed.

Verify (cold is the test that matters): full power drain, power on -> expect the boot
word to show AND hold, touch toggles play/pause. Then RESET button and a reflash ->
expect the same, since GPIO21 now hard-resets both die halves every boot, which should
also clear the stale touch wedge that made touch dead after every flash.

If cold boot still shows fading noise with the GPIO21 reset in, the fallback lead is
timing: rsvpnano waits ~2 s (serial wait) before display init on cold boot; try a settle
delay before the reset pulse. If touch dies again with the GPIO21 reset in, poll touch
earlier or probe before the reset like rsvpnano does.

### Addendum, same day: the schematic ends the V1/V2 reset debate

Read the schematic PDF in the V2 demo repo (sheet title "ESP32-S3-Touch-LCD-3.49 V1.1").
The panel's RSTN and TE pins connect to IO21 and EXIO5 through a block of option
resistors: R74/R72 route IO21 to LCD_RST or LCD_TE, R73/R75 route EXIO5 to LCD_RST or
LCD_TE, populated 0R vs NC per production run. **Both routings exist in copper; the
stuffed resistors decide.** That is why V1 docs say GPIO21, V2 docs say EXIO5, and
rsvpnano gets away with GPIO21 on both revs. On OUR unit the observed behavior decides:
the V1 demo (GPIO21, never touches the expander) works cold, and our EXIO5 build did
not, so this unit routes IO21 -> LCD_RST.

### Boot captures on the GPIO21 build (this session, over USB with DTR)

- Post-flash boot: panel create, init cmds ok, flush 1 ok, touch probe ESP_OK, SD
  mounted, boot word, and a tap started playback. Touch alive right after a flash is
  new; the GPIO21 reset clears the stale wedge as predicted.
- RTS warm reset capture: identical healthy sequence from line one. **"LCD panel
  create success" appears on every boot.** The "panel doesn't get created at restart"
  reading came from the monitor losing early lines while the USB console re-enumerates
  after reset; grep the full capture, don't trust the monitor tail.

### Init-order comparison (all three references read end to end, no race found)

- V1 demo: backlight PWM init (GPIO8) -> touch I2C init -> GPIO21 config -> SPI bus ->
  panel IO -> panel create -> GPIO21 pulse H30/L250/H30 -> panel init (cmd list is ONLY
  SLPOUT+DISPON) -> LVGL + indev -> draw -> backlight enable.
- V2 demo: backlight PWM init (GPIO42) -> touch I2C init -> TCA9554 (EXIO0 input,
  BL_EN out low, EXIO5 out high) -> SPI bus -> panel IO -> panel create -> EXIO5 pulse
  -> panel init (SLPOUT+DISPON only) -> LVGL + indev -> draw -> BL_EN high.
- rsvpnano: buttons -> backlight GPIO low -> both I2C buses -> EXIO6 high -> touch
  probe -> NVS prefs -> backlight PWM off -> GPIO21 pulse -> SPI bus -> 5-cmd init ->
  EXIO1 high -> render -> SD after display.
- Ours now matches rsvpnano's structure; every step in our boot also exists in a
  working reference at the same relative position. No ordering hazard identified.

Also noted: both vendor demos run a 2-command init list (SLPOUT, DISPON). Our 5-command
list is rsvpnano's. If the fade persists on this unit, the delta to test is the
backlight drive (LEDC PWM vs static GPIO42 low), not more init commands.

### Hardware result of the GPIO21 build: same inverted pattern, so reset was not it

User test, GPIO21 build: warm boots (flash, RST) render the word but touch is dead;
cold power-on shows fading noise, never a word, and touch WORKS (taps logged over the
monitor). Identical to the EXIO5 build. Neither pulse changes the die's behavior, so
on this unit probably neither line reaches the panel's RSTN at all.

Also read the managed driver source (esp_lcd_axs15231b.c): with external init_cmds set
it sends ONLY SLPOUT+MADCTL+COLMOD plus our list. Its big vendor table (gate block,
SWRESET) is not sent; SWRESET lives only in esp_lcd_panel_reset, which we do not call.
So cold and warm boots send byte-identical init, yet warm renders and cold does not.

### Current hypothesis: cold init lands while the AXS die is still booting

The only cold/warm difference left is die age when init arrives (~1.2 s after first
power vs seconds-to-minutes). A cold die still in its internal boot ignores the init;
the session then starts unrendered (noise fade). The next warm RST re-runs the same
init on a settled die and it works. rsvpnano's cold success on this unit fits: it waits
~2 s for serial before display init; that delay, not its (likely no-op) GPIO21 pulse,
may be its entire cold-boot magic.

Change to test (built, user flashes): `DIE_BOOT_SETTLE_MS = 2500` wait in init_panel
before the reset pulse and panel init. Single variable. Expect: cold power-on now shows
the word and it holds. If confirmed, tune the delay down (try 1500/1000) and consider
paying it only when reset reason is POWERON.

Warm-boot touch death is a separate open question (it predates all reset changes; no
warm boot in any build has ever had working touch except one unexplained post-flash
session). Tests for it, untethered: RST with a finger held on the glass (the session-4
kick) vs RST without. Note real usage is power-on/power-off, both cold paths, so warm
touch mainly hurts dev flow.

### Result: 2.5 s delay + GPIO21 pulse still fails cold. But the delay test was dirty.

User flashed the settle-delay build: init now lands at ~3.7 s, later than rsvpnano's
~2.2 s, and cold boot STILL shows noise, no word, touch alive. Warm boots render, touch
dead. So timing alone (with the pulse present) is not it.

The confound: that build held GPIO21 as a LOW output through the whole 2.8 s window
while the die booted, then pulsed it. Against session 6's confirmed-good state (EXIO5
pulse, GPIO21 never touched, warm touch WORKED), driving GPIO21 is the one hardware
change in today's builds, and today warm touch died in every boot. If this unit is
stuffed V2-style, GPIO21 is the panel's TE OUTPUT and we are fighting it; that can
wedge the touch core warm and may also have broken the cold init the delay was meant
to fix.

Next build (ready, user flashes): NO reset of any kind, GPIO21 left completely alone,
keep the 2.5 s settle delay. Predictions: warm touch returns (session-6 state), and
cold render is the real test of the delay. If cold STILL fails, flash the V1 demo and
cold-drain test it watching the screen: nobody has ever actually confirmed any firmware
rendering on a true cold boot on this unit; if the demo also fails cold, this is a
board-level trait (e.g. RSTN on a slow RC, or panel needs longer), not our code.

### The 0.1 s word blink at boot, then 0.5 s black, then the word (user dislikes it)

Not a double render bug. Order: backlight enables right after panel init while GRAM
still holds the PREVIOUS session's frame (the old word blinks ~80 ms), flush 1 then
paints LVGL's initial empty black screen, and the boot word only arrives with flush 2
once the SD is mounted and the book is opened (~1.3 s later). Fix when the cold-boot
bug is closed: enable the backlight only after the boot word has rendered, with a
fallback enable if the pipeline fails so the screen never stays dark.

## SESSION 6 (2026-07-02) — root cause found: the panel never gets a real reset

Symptom this session: flash -> display works, touch dead. Power cycle -> touch works,
display shows backlight + random fading pixels, never a word.

### Evidence gathered on hardware (logs in the session scratchpad)

- **Warm boot log:** `Power: power latched on, backlight enabled` — the TCA9554 answers
  at 0x20. The "board is V1, no expander" conclusion from the A/B session was a misread:
  IDF 5.5 returns `ESP_ERR_INVALID_STATE` for a plain I2C NACK (`i2c_master.c:727`), it
  does not prove the chip is absent.
- **Cold boot log (true power-on, reset reason 1):** everything succeeds — expander,
  SD, panel init, LVGL flushes complete in ~17 ms, boot word rendered, taps register
  (`Ui: tap, playing`). The firmware is healthy while the physical screen shows fading
  noise. The panel silently discards GRAM writes.
- **V2 vendor demo (ground truth, confirmed working on this board):** GPIO21 is
  **LCD_TE**, `LCD_RST = -1`. The die reset is **EXIO5 on the expander**, pulsed
  H30/L250/H30 between panel create and `esp_lcd_panel_init`. No SWRESET anywhere.
  Backlight PWM is **GPIO42, active low** (`LCD_PWM_MODE_255 = 0xff-255 = duty 0`),
  EXIO1 is the enable, held OFF during init and enabled after LVGL is up. GPIO8 is the
  **expander INT line**, not a power rail.
- V1 factory program and rsvpnano both use the same minimal SLPOUT/DISPON init list,
  so the init command list was never the problem.

### The root cause chain across sessions

1. Sessions 2-3 ran EXIO5 reset + SWRESET. SWRESET wedges the touch core (survives
   everything but die power loss). EXIO5 got blamed.
2. Session 4 replaced EXIO5 with a GPIO21 pulse. On this V2 board GPIO21 is TE — the
   pulse resets nothing. SWRESET stayed, so displays kept working (SWRESET is a real
   reset) and touch kept wedging.
3. This week SWRESET was removed. Now nothing resets the panel at all: warm boots coast
   on state left by a previous real init in the same power session; cold boots come up
   lit-but-non-rendering and fade. Touch works cold because no SWRESET wedges it.
4. The correct combination — EXIO5 reset, no SWRESET, GPIO21 untouched — was never tried.

Touch-dead-after-flash is expected: flashing (DTR) never cuts die power, so a stale
wedge or DTR disturbance persists until a physical power cycle.

### Fix plan (this session)

1. `board_pins.hpp`: GPIO21 -> `LCD_TE` (panel output, never drive). `LCD_RST_EXIO = 5`.
   GPIO8 -> `EXPANDER_INT` (input, unused). Add `BACKLIGHT_PWM = GPIO_NUM_42`, active low.
2. `power.cpp`: add `reset_lcd()` (EXIO5 H30/L250/H30). Init drives EXIO5 high, keeps
   the backlight off; new `set_backlight(bool)` drives EXIO1 + GPIO42 (low = on). App
   enables the backlight after display init so boot noise stays invisible.
3. `display.cpp`: drop all GPIO21 code and take a `reset_panel` callback in
   `DisplayConfig`, invoked between panel create and `esp_lcd_panel_init` (demo order).
4. `app.cpp`: stop driving GPIO8; wire `cfg.reset_panel` to `Power::reset_lcd`.

Verify: build, flash, power cycle. Expect word + touch cold AND warm (RESET button),
plus no fade (the fade was the un-reset panel's analog state decaying).

Touch now works reliably (tap toggles play/pause) with the minimal 5-command panel init
and no reset dance. Backlight was never enabled: EXIO1 is the enable and nothing drove it,
so the panel rendered into the dark. Power::init now drives EXIO1 high. Confirmed on
hardware: SD, touch, and the reader all work; the reader shows the resume word at boot.

### The open bug: a static image fades out in a few seconds

With the minimal init the picture shows, then fades when idle. A 1s keepalive reflush did
NOT help (VCOM/charge is not restored by rewriting GRAM).

Bisected the esp_lcd vendor init against touch:
- Minimal init (SLPOUT/MADCTL/COLMOD/DISPON): touch works, image fades.
- Power/VCOM block (0xA0..0xCF): touch works, still fades.
- Add gate timing (0xD5..0xDF): image HOLDS, but touch dies.
- Any partial gate set (D5/D6 alone, or D7..DF alone): panel does not display at all, and
  touch still dies. So the gate block is all-or-nothing for the panel AND it stops the
  shared touch scanner. Irreducible via the init command list.

### Why the fade is probably the backlight, not the panel

rsvpnano runs the SAME 5-command init with NO gate timing and NO continuous refresh, and
its image holds. The one hardware difference: its backlight is **active-low PWM on a GPIO
(rev1 = 8, rev2 = 42), driven by the AP3032 driver**, not the EXIO1 digital enable we use.
`digitalWrite(kBacklightPin, LOW)` = on. See rsvpnano
`src/platforms/waveshare_lcd_349/` (WaveshareLcd349.h, BoardSystem.cpp) and
`src/drivers/display/axs15231b/axs15231b.cpp` writeBacklightPwm.

Next time, start here: drive the real backlight pin (GPIO 8 or 42 for our rev) as
active-low PWM at ~25 kHz instead of (or with) EXIO1, and see if the fade goes away with
the minimal, touch-safe init. rsvpnano also has a `resetTouchHardware()` helper in
`BoardInput.cpp` if the touch ever needs a re-kick after display bring-up.


## SESSION 4 (2026-06-30) — root cause found via a third working reference

**The reset line is wrong.** We reset the AXS die over **EXIO5** (`Power::reset_lcd`) and pass
`panel.reset_gpio_num = GPIO_NUM_NC`. On this board EXIO5 is not the die reset, so the touch
core never gets a hardware reset and returns constant `0xEA` / wedges ~1s after power-on. The
display still comes up because SLPOUT/DISPON over QSPI wakes only the display half of the die.

### The reference that settles it: `ionutdecebal/rsvpnano`

Repo: https://github.com/ionutdecebal/rsvpnano (`git@github.com:ionutdecebal/rsvpnano.git`)

A full working RSVP firmware for this exact board. `src/platforms/waveshare_lcd_349/`:

- **Die reset = GPIO21**, sequence `HIGH 30ms -> LOW 250ms -> HIGH 30ms`, driven inside display
  init (`drivers/display/axs15231b/axs15231b.cpp` lines 100-107). Identical to the V1 demo.
- **EXIO5 is never driven.** The TCA9554 is used only for EXIO1 (backlight enable), EXIO6 (sys
  enable / battery hold), EXIO7 (audio enable). Grep-confirmed across the whole 349 platform.
- **rev1 and rev2 both use GPIO21 reset.** The only per-revision difference is the backlight
  GPIO (8 vs 42). So the V1/V2 reset ambiguity that burned sessions 1-3 is moot: regardless of
  revision, the working firmware resets the die on GPIO21, not the expander.
- Touch is **pure polling**: `kTouchResetPin = -1`, `kTouchIrqPin = -1`, `touchReady()` returns
  true. So the EXIO0 INT line (session 3's top lead) is NOT required for working touch.
- Read command byte[7] = `0x08`, reads an **8-byte** packet (we send `0x0e`, read 32). Count at
  `buff[1]`, coords at `buff[2..5]` — same decode. Our 32/0x0e matches the demo and works there,
  so this is a secondary difference, not the bug.
- A read-failure recovery state machine exists (5 fails -> backoff -> re-init) but keys on I2C
  read failures, not on the `0xEA` value, so it would not detect our wedge either. Not the fix.

### Why the prior "tried GPIO21, still 0xEA" result was a false negative

Session-1 tried a GPIO21 reset and saw constant `0xEA`. That test ran over a serial monitor with
DTR asserted, which session 3 proved wedges touch from the first read. The GPIO21 reset was never
fairly evaluated untethered. Three working firmwares (rsvpnano rev1+rev2, V1 demo) reset on GPIO21
and run fine with no serial attached.

### Fix applied this session (needs untethered hardware confirmation)

1. `board_pins.hpp`: added `LCD_RST = GPIO_NUM_21` (the real die reset). EXIO5 demoted, no longer
   the reset.
2. `display.cpp` `init_panel`: pulse GPIO21 `H30/L250/H30` before panel creation, matching the
   reference and the V1 demo. `reset_gpio_num` stays NC (we drive the pin ourselves to get the
   long 250ms low the AXS wants, longer than esp_lcd's default 10ms).
3. `app.cpp`: dropped the `_power.reset_lcd()` (EXIO5) call from boot.

**Verification (must be untethered):** flash, unplug from the PC, power from a charger / power
bank, RESET (EN), tap repeatedly to play/pause. Do NOT judge over `idf.py monitor` — DTR wedges
the die and will mask a working fix.

### RESULT: flashed, touch still dead — but a decisive new reproduction

The GPIO21-reset build was flashed. Touch is still dead on a normal boot, **tethered and
untethered both**, so DTR is no longer the variable. The new, repeatable reproduction:

- **Hold/keep tapping the screen *during* the reset -> touch works.**
- **Reset with no finger on the screen -> touch never starts (constant 0xEA), forever.**

So the touch core boots into a non-scanning state and a **physical touch present as the die
leaves reset** is what kicks it into scanning. Without that, it never wakes on its own.

### What this means: it is the touch core's scan state at reset, not our read or the reset pin

rsvpnano boots with no finger and gets live touch. So its boot leaves the scanner running; ours
does not. Confirmed by reading rsvpnano's recovery path (`src/input/Input.cpp`): `readPacket`
returns **success** on an all-0xEA frame (the I2C transaction ACKs and the byte count is right),
so `consecutiveReadFailures` stays 0 and its failure-recovery (5 fails -> re-init) **never fires
on the 0xEA wedge**. Its recovery only catches hard I2C NAKs. So recovery is not the magic; the
difference is purely that rsvpnano's die comes out of reset already scanning and ours does not.

The reset *pin* is therefore likely correct now (GPIO21, matching the reference). What is missing
is whatever puts the touch core into continuous-scan after reset. Candidates, to investigate:

1. **A touch init / wake command stream.** rsvpnano streams its own `kQspiInit` command list to
   the die right after the GPIO21 reset (`drivers/display/axs15231b/axs15231b.cpp`). Our esp_lcd
   path streams a different `LCD_INIT_CMDS`. One of those init commands may enable the touch
   scanner. Diff rsvpnano's `kQspiInit` against our `LCD_INIT_CMDS` byte for byte.
2. **EXIO0 / touch INT state at reset.** A finger during reset drives the INT line. The touch
   core may sample or require the INT line at reset-release. We leave EXIO0 as a floating
   expander input. rsvpnano sets `kTouchIrqPin = -1` and ignores it, but its board may pull it
   differently. Check the expander default and whether INT needs a defined level at reset.
3. **Reset-to-first-read timing / scan-mode latch.** The core may auto-sleep if not addressed
   quickly enough after reset. rsvpnano polls from very early and continuously (20 ms); we only
   start reading when the LVGL indev registers at the very end of boot. The gap may let the core
   idle into the stuck state. A finger holds it awake across that gap.

### STOP: we are past the 3-failed-fix threshold — bisect, do not keep swapping

Across sessions 1-4 we have tried EXIO5 reset, no reset, direct GPIO21 reset, read rewrites, and
input hardening. None fixed the core wedge. Per the debugging rules this is the point to stop
guessing single changes and either question the architecture or bisect. Concrete next step:

**Minimal touch+display-only image.** Strip SD, NVS, IMU/auto-rotate, pipeline, BT, the power
latch/EXIO7 — match rsvpnano's lean boot. GPIO21 reset, poll touch continuously from early boot
(not just from the LVGL indev). Test untethered, no finger. If touch works, add our subsystems
back one at a time until it breaks; the one that breaks it is the disturber. If it still fails
even minimal, the remaining delta is the init command stream (lead 1) or our esp_lcd vs their
hand-rolled QSPI driver — port rsvpnano's `kQspiInit` + raw read path verbatim.

### What this rules in (if untethered): something in our boot disturbs the die

rsvpnano resets on GPIO21 and works; we now reset on GPIO21 and don't. So the remaining delta is
not the reset, it is the rest of our boot. Differences from rsvpnano, ranked as suspects:

1. **TCA9554 EXIO7.** We drive EXIO6 *and* EXIO7 high at boot (`Power::init`). rsvpnano drives
   only EXIO6 (sys-enable); EXIO7 is the **audio enable** and it raises it only when audio is
   needed, never at boot. Driving the audio rail at boot is a real, unexplained difference on the
   shared expander. Try: stop raising EXIO7 at boot, raise EXIO6 only.
2. **Bluetooth.** Enabled in our sdkconfig, absent from rsvpnano. Flagged in session 3 too.
3. **IMU / port-0 I2C traffic + auto-rotate** running before/around touch bring-up.
4. **esp_lcd managed AXS driver vs rsvpnano's hand-rolled QSPI init.** Different init command
   stream to the shared die; could leave the touch core in a different state.

### Cheapest decisive next step: minimal-boot bisection

Build a touch+display-only image (no SD, NVS, IMU/auto-rotate, pipeline, BT, power latch/EXIO7),
GPIO21 reset, untethered. If touch works, add subsystems back until it breaks. Add EXIO7, then
BT, then IMU last. This is the only way to stop guessing across the many boot-order differences.

If touch works untethered, fold the cleanup: the `LCD_RST_EXIO` constant and `Power::reset_lcd`
are already removed; confirm nothing else referenced them.

## SESSION 3 (2026-06-30, later) — instrumented the read, hardened input, wedge still open

**Where we landed:** the dead state is now positively identified, several confounders are
ruled out, the input path is hardened, but the core "touch wedges ~1s after reset" fault is
still present untethered. Firmware is flashed with the hardening below.

### Confirmed this session (decisive, stop re-testing)

- **Dead state = `err==ESP_OK` + constant `0xEA` (234).** Added a throttled diagnostic to
  `Controls::read` logging `err`, `count` (`buff[1]`), `buff[0]`, x, y. Captured over serial
  on multiple clean boots: every read is `err=0 count=234 b0=234`. The I2C transaction
  succeeds and ACKs; the AXS touch core has simply stopped scanning and returns filler. The
  bus and our read code are fine. This matches the byte-identical, known-working vendor V2
  demo read path, so the read is not the bug.
- **The USB serial cable (DTR) wedges touch instantly.** With a monitor / DTR-asserting
  reader attached, touch is dead from the *first* read (~35 ms after `touch ready`); there
  is no "working second" at all over the cable. This is confounder #1 from Session 2, now
  proven hard. You cannot observe working touch over serial. Every prior on-cable capture of
  constant 234 was partly this.
- **Idle and wedged are indistinguishable by the read value.** A healthy, untouched screen
  also reads `0xEA`. So a "detect stuck reads -> reset the die" watchdog keyed on the read
  value is impossible: it would re-pulse the reset (and reset the shared *display* die)
  constantly during normal no-touch idle. Any real wedge detection needs an out-of-band
  signal (the touch INT line, EXIO0), not the I2C frame.

### Ruled out by hardware testing (do not revisit)

- **Auto-rotate rotation flush** — user tested flat vs upright; identical. Flat sits in the
  IMU dead zone so no `set_flipped` fires, yet touch still wedges.
- **USB tether / DTR / phantom serial bytes as the *primary* cause** — user ran untethered
  (no PC); touch still wedges ~1s in. (DTR still makes it worse, see above.)
- **Power management / light sleep** — `# CONFIG_PM_ENABLE is not set`; CPU fixed 160 MHz.

### Code changes this session (in working tree, build green, flashed)

1. `controls.cpp` / `controls.hpp`: **hardened the tap.** Validate the frame (press only on
   `err==OK`, count 1..4, x/y within the 640x172 view; the `0xEA` filler now reads as
   released). Debounce a state change over 2 reads. 250 ms tap cooldown. This kills the
   phantom double-fire (1 tap = 2 toggles) and the self-sustaining ~6 Hz play/pause
   oscillation. Replaces the old fire-on-every-raw-edge logic.
2. `controls.cpp`: added `i2c_master_bus_wait_all_done(_bus, 100)` before the read, matching
   the demo. `controls/CMakeLists.txt`: added `esp_timer` to `PRIV_REQUIRES` (cooldown clock).
3. `text_pipeline.cpp`: `run_commands` now **drains the USB-Serial-JTAG RX FIFO at startup**
   so stray host bytes can't post play/pause at boot (the boot-time `Pipeline: play/pause`
   with no `Ui: tap` line was this). The serial harness is a dev shortcut, not real input.
4. The temporary diagnostic logging in `read` was removed (the hardened read supersedes it).

### Still open: the ~1s wedge, and the next leads

Touch still wedges ~1s after reset untethered. The vendor demo runs untethered on this same
board without wedging, and our read is byte-identical, so something in our surrounding system
still disturbs the shared die. Leads, in order:

1. **Touch INT line (EXIO0).** The demo configures it as an input. It is the only signal
   that distinguishes a real touch from the `0xEA` idle, so it is the key to both reliable
   detection and a usable wedge watchdog. Bring EXIO0 in (read via the TCA9554 / Power) and
   drive touch detection from INT plus the I2C read, not the I2C read alone.
2. **Minimal-firmware bisection.** Build a touch+display-only image (no SD, NVS, IMU,
   pipeline, BT, power latch) like the demo, confirm touch is stable, then add subsystems
   back until it wedges. Prime suspects to add last: BT (enabled in our sdkconfig, absent
   from the demo), the IMU/port-0 traffic, the power latch.
3. **Die-reset + display-reinit recovery** as a last resort (re-pulse EXIO5 then re-init the
   panel), only viable once INT gives a trustworthy wedge signal so it does not fire on idle.

## SESSION 2 (2026-06-30 PM) — current state for the next chat

**Where we landed:** touch is partially working but unreliable. After a clean RESET you
can tap and the reader starts (tap -> play registers), but a later tap to pause often
does not register. Same "works right after reset, then stops" pattern the vendor demo
also shows. Not root-caused yet. Firmware is flashed and clean (no debug scaffolding).

### What is now CONFIRMED (stop re-testing these)

- **Right chip.** Waveshare docs confirm the AXS15231B is both the QSPI display driver
  and the I2C capacitive touch (one combo die). Our probe answers `ESP_OK` at `0x3B`.
- **Read protocol is correct.** Command `{0xb5,0xab,0xa5,0x5a,0x00,0x00,0x00,0x0e,0x00,
  0x00,0x00}`, count in `buff[1]` (1..4 = pressed), `x=(buff[2]&0x0F)<<8|buff[3]`,
  `y=(buff[4]&0x0F)<<8|buff[5]`. Matches the vendor demo and every public driver
  (Processware, F1ATB, espressif esp_lcd_axs15231b).
- **`0xEA`/234 = idle (no finger), not a fault.** The working vendor demo reads all-234
  when untouched too.
- **The touch -> tap -> play/pause path is wired correctly end to end.** Proven: a tap
  fired the handler and started the book (saw it on hardware).
- **The board is V2.** `board_pins.hpp` matches the V2 demo `user_config.h`: touch port 1
  (GPIO18/17) @ 0x3B 300 kHz, LCD/touch reset on **EXIO5**, backlight EXIO1, touch INT
  EXIO0, **GPIO21 = LCD_TE** (not reset). EXIO6/7 driven high matches the factory app.

### CONFOUNDERS that wasted hours (do not repeat)

- **`idf.py monitor` / USB-DTR hangs the touch.** Asserting DTR over USB-Serial-JTAG
  coincides with touch reads going stuck. The working demo also read stuck-234 over a
  DTR link yet worked fine with no serial attached. **Judge touch with NO serial monitor
  running** — power from a charger or a plain cable, look at the screen only.
- **The POWER button (GPIO16) is our power-off latch**, not a reset. Pressing it on
  battery powers the device down (releases EXIO6). Use the dedicated **RESET (EN)**
  button to reboot. The board also has a BOOT button (flashing only).
- **"Black screen, backlight on" is the normal PAUSED state** (empty label on black bg),
  not a display fault. Display rendering is confirmed working (the "TAP" probe showed).

### What changed in the code THIS session (all committed-worthy, in working tree)

1. `app.cpp`: replaced the GPIO21 reset experiment with `_power.reset_lcd()` (EXIO5),
   and added `_controls.init_bus()` **before** the reset so the touch I2C lines are
   pulled high when the AXS die latches its state at reset release. Dropped unused
   FreeRTOS includes.
2. `controls.{hpp,cpp}`: **rewrote to match the demo exactly** — split `init()` into
   `init_bus()` (early, before reset) and `start()` (after LVGL). Removed the separate
   20 ms poll task; touch I2C is now read **inside the LVGL input callback**, in the same
   task that flushes the panel, so a read can never overlap a QSPI transfer on the shared
   die. All debug scaffolding (on-screen readout, green-flash, read-guard) removed.
3. `ui.{hpp,cpp}`: removed debug probes; clean tap -> play/pause.

This is the architecturally-correct, demo-matching version. It builds clean and is
flashed. It did NOT fully fix the "stops after a while" symptom.

### RANKED hypotheses for the remaining unreliability (next session, in order)

1. **Hardware: marginal display/touch flex connector.** Both display AND touch are
   intermittent and react to physical pressure on the board. They share one FPC. This
   fits everything. **First action next session: reseat the FPC, then test off a wall
   charger (no PC).** If a charger + reseated flex is reliable, it was hardware all along.
2. **Taps missed under render load when playing.** When the reader plays, the tick task
   calls `show_frame` per word; the LVGL task spends its time flushing, so the indev read
   (which now lives in the LVGL callback) samples sparsely and can miss a quick
   press->release edge — hence "starts but won't stop." Test: tap and HOLD ~1 s to pause;
   if hold works but quick tap doesn't, it's sampling rate. Fix options: detect the tap on
   the press edge, or go back to a dedicated poll task that ONLY does I2C + atomic state
   (no rendering) under the display lock, decoupling sampling from render load.
3. **AXS touch engine genuinely wedges and only a die reset recovers it** (documented for
   this chip under display/touch contention, e.g. lvgl_micropython issue #519). If 1 and 2
   are ruled out, add a touch watchdog: on a run of clearly-stuck reads, re-pulse EXIO5.
   Downside: that also resets the display (shared die) and will glitch the screen, so only
   do this as a last resort and gate it carefully.

### Exact experiments to run next session (cheap -> decisive)

1. Power from a **USB wall charger / power bank** (no PC, no monitor). Tap to play, tap to
   pause, repeat 10x. Reliable? -> not a software bug; was USB-host/DTR.
2. **Reseat the display FPC connector**, clean RESET, repeat. Improves? -> flex connector.
3. If still flaky on charger + reseated flex: build a **minimal touch+display-only
   firmware** (no SD, NVS, IMU, pipeline, power latch — just like the demo) and bisect by
   adding subsystems back until touch breaks. Prime suspects to add last: the IMU/auto-
   rotate (port 0 I2C + calls set_flipped) and the power latch.

---

## RESOLUTION (2026-06-30 AM) — superseded by Session 2 above

The earlier premise was wrong. Findings after flashing the **V2** demo to the board:

- **The board is V2, not V1.** `board_pins.hpp` matches the V2 demo `user_config.h`
  exactly: touch on port 1 (18/17) at 0x3B 300 kHz, LCD/touch reset on **EXIO5**,
  backlight EXIO1, touch INT EXIO0, and **GPIO21 = LCD_TE** (tearing effect), not reset.
- **A constant `0xEA` (234) frame is the normal idle/released value**, not a fault. We
  confirmed the freshly flashed V2 demo also reads all-234 when no finger is down, yet
  its touch responds to taps. So 234 just means "no touch."
- **The V2 demo's touch works on this board** (confirmed visually after a reset, with no
  serial attached). Hardware and the V2 conventions are good.
- **The USB serial / DTR connection coincides with the touch engine hanging.** Our raw
  captures all showed stuck 234 because asserting DTR over USB-JTAG disturbs the shared
  AXS die. Judge touch visually, not over a live serial monitor.

**Root cause in our firmware:** `app.cpp` was pulsing **GPIO21** (the TE pin on V2),
not EXIO5. The AXS die never got a hardware reset, so the touch MCU never started
scanning. The display still came up because `esp_lcd_panel_init` sends SLPOUT/DISPON
over QSPI, which wakes only the display side of the die.

**Fix:** restored `_power.reset_lcd()` (EXIO5 pulse, same timing as the demo) in
`app.cpp` before `display.init()`, and dropped the GPIO21 experiment.

**Still open:** the intermittent hang (touch stops, reset recovers) seen on the demo
too. If our firmware hangs the same way in use, add a stuck-state detector that
re-pulses EXIO5, and consider serializing touch reads against the QSPI flush via the
display lock. Don't chase this until the EXIO5 fix is confirmed on hardware.

---

Status as of 2026-06-30: **touch not yet working in our firmware.** The vendor demo's
touch works on the physical board, so the hardware is fine. Our firmware reads a
constant filler frame. This doc captures everything tried so the next person doesn't
re-walk the same ground.

## Goal

Get the capacitive touch on the Waveshare ESP32-S3-Touch-LCD-3.49 working as an LVGL
input device so a tap toggles play/pause (and later drives the M5 pictograms).

## Which demo worked

Two vendor repos exist, one per board revision:

- **V1:** https://github.com/waveshareteam/ESP32-S3-Touch-LCD-3.49
- **V2:** https://github.com/waveshareteam/ESP32-S3-Touch-LCD-3.49-V2

We flashed the **V1 demo `Examples/ESP-IDF/10_LVGL_V9_Test`** to the board and
**touch worked** (confirmed by the user: display + touch both functional). So the
board behaves like **V1**. We did not get to flash the V2 demo for comparison.

This matters because the two revisions reset the AXS die differently (see below), and
our firmware is currently wired for **V2** conventions while the board acts like V1.

## Hardware facts learned

- The **AXS15231B is one combo die**: QSPI display + I2C touch. Resetting the die
  resets both.
- **Touch I2C:** SCL = GPIO18, SDA = GPIO17, address **0x3B**, **300 kHz**, new I2C
  master driver. The sensor bus owns port 0, so touch is on **I2C port 1**.
- **Touch read protocol** (same in the demo and our code): write an 11-byte bridge
  command, then read 32 bytes in one `i2c_master_transmit_receive` (write, repeated
  start, read):
  - command: `{0xb5, 0xab, 0xa5, 0x5a, 0x00, 0x00, 0x00, 0x0e, 0x00, 0x00, 0x00}`
  - `buff[1]` = touch count. `1..4` = pressed, anything else = released.
  - `x = (buff[2] & 0x0F) << 8 | buff[3]`, `y = (buff[4] & 0x0F) << 8 | buff[5]`.
- **Reset, V1:** dedicated **GPIO21**, pulsed `high(30ms) -> low(250ms) -> high(30ms)`
  then `esp_lcd_panel_init`.
- **Reset, V2:** over the **TCA9554 expander** (an EXIO line). On V2, **GPIO21 is
  LCD_TE** (tearing-effect), not reset.
- **Expander map we use (V2 style), `board_pins.hpp`:** EXIO0 = touch INT (input,
  unused, we poll), EXIO1 = backlight, EXIO5 = LCD reset, EXIO6 = power latch,
  EXIO7 = power-enable.

## The symptom

Our firmware's touch read returns a **constant frame: every byte = 0xEA (234)**,
whether idle or pressed.

- `i2c_master_probe(0x3B)` returns **ESP_OK** (the chip answers on the bus).
- The read returns **err = 0** (the transaction ACKs), but all 32 bytes are `0xEA`.
- Because `buff[1] = 234` is outside `1..4`, it always reads as "released", so no
  press is ever detected.

Interpretation: the `0xb5ab...` command is a **bridge** to the AXS touch MCU. Constant
`0xEA` means the bridge reaches the chip but the touch MCU is not producing live data
(not scanning / not initialised).

One early boot (first rewrite, `touch2.log`) showed real-looking coordinates
(count `1..4`, `x=315 y=110`) repeated ~163 times. That was likely a single latched
touch, not healthy live data. Every capture since has been constant `0xEA`.

## What we tried (none fixed it)

1. **Rewrote `Controls` to be byte-identical to the V1 demo's read path** — single
   device at 0x3B on I2C port 1, raw `i2c_master_transmit_receive`, 300 kHz, dropped
   the vendor `esp_lcd_touch_new_i2c_axs15231b` driver entirely (it was registering a
   second device at 0x3B and going unused). Result: still constant `0xEA`.
2. **Removed the EXIO5 expander LCD reset** from boot. Result: display still comes up
   (SLPOUT/DISPON is enough to wake the display side), touch still `0xEA`.
3. **Added a direct GPIO21 reset pulse** (V1 timing: high30/low250/high30) before
   display init. Result: display up, touch still `0xEA`.

## The key missing experiment (do this first)

**Capture the V1 demo's own raw touch frame, idle vs touched.** We added a log to the
demo but never captured it (flash was declined at the end of the session). Steps:

1. Re-clone the V1 repo (the scratchpad copy is ephemeral and gone).
2. In `Examples/ESP-IDF/10_LVGL_V9_Test/main/main.c`, `TouchInputReadCallback`, add a
   throttled `ESP_LOGI` of `buff[0], buff[1], pointX, pointY`.
3. Build, flash, capture idle (no touch) and touched frames.

This answers the decisive question: **is the demo's idle frame also `0xEA`** (then our
problem is purely that our chip never starts scanning, so look at init/reset/state
differences) **or is it structured with `count=0`** (then our chip is in a
fundamentally different state than the demo's, e.g. wrong mode/asleep)?

## Other leads for the next person

- **Resolve the board revision contradiction.** The board acts like V1 (V1 demo's
  GPIO21 reset path works), but `board_pins.hpp` / `power.cpp` are configured for V2
  (reset on EXIO5, backlight EXIO1, touch INT EXIO0). Check the physical board's
  silkscreen/sticker and the schematic PDF (`schematic/` in each repo). If it is V1,
  the LCD/touch reset belongs on **GPIO21**, not the expander.
- **Diff the full boot sequence.** The demo does very little before reading touch
  (touch I2C init -> GPIO21 reset -> QSPI panel init -> read). Our firmware does a lot
  before touch: `board_power_on`, TCA9554 power latch (EXIO6/7), backlight, SD, NVS,
  IMU + auto-rotate (port 0 I2C), display, pipeline, then touch. Something in there may
  leave the AXS touch MCU in a bad state. Prime suspects:
  - The **TCA9554 expander writes** (`power.cpp` drives EXIO6/7). The V1 demo never
    touches the expander. Confirm our config/output register writes don't disturb a
    touch-related line, and check the expander's power-on default vs what we set.
  - **QSPI display flush concurrent with I2C touch reads on the shared die.** We read
    touch from a dedicated 20 ms task; the demo reads from the LVGL task, serialized
    with the flush. A bridge read during a display transfer could return `0xEA`. Test:
    move the touch read into the LVGL task, or gate it on the display lock.
- **Cross-check with the vendor driver.** Try `esp_lcd_touch_read_data` +
  `esp_lcd_touch_get_coordinates` instead of the raw bridge read, to rule out a
  protocol detail.

## Serial capture gotcha (you will hit this)

Console primary is **UART0**, with **USB-Serial-JTAG as secondary**. Plain
`cat /dev/ttyACM0` returns nothing because the USB-JTAG console only streams when
**DTR is asserted**. `idf.py monitor` needs a TTY and fails when backgrounded.

Workaround used: a small pyserial reader at 115200 with `p.dtr = True` (optionally
pulse RTS to reset and capture the boot probe line). The poll task only logs on a
detected press, plus a temporary `raw err=... b0..b5` dump every ~1 s — grep for
`raw err` / `press raw` / `probe`.

## Current state of the working tree (uncommitted, branch `feat/controls-picker-settings`)

- `components/controls/controls.cpp` — rewritten to demo-style raw read. **Contains
  temporary diagnostics**: the `raw err=...` per-second dump and `press raw` log.
  Remove once touch works.
- `components/controls/include/controls.hpp` — dropped `_io`/`_touch`/`_dbg_dev`, now a
  single `_dev`.
- `components/controls/CMakeLists.txt` — dropped `esp_lcd` / `esp_lcd_touch` /
  `espressif__esp_lcd_axs15231b` deps (no longer used by `controls`).
- `main/app.cpp` — replaced `_power.reset_lcd()` (EXIO5) with an **experimental direct
  GPIO21 reset pulse**; added FreeRTOS includes. Note: `power.reset_lcd()` is now unused
  in boot — decide whether to keep it for V2 or remove.

None of these are committed. The GPIO21 reset and the dropped EXIO5 reset are
experiments, not conclusions — revisit once the demo-frame comparison clarifies the
root cause.

## 2026-07-01: A/B test proves our firmware wedges the touch die

Ran the decisive A/B on hardware. Result is unambiguous.

- Cold power drain (unplug, let rails bleed ~10s, replug), then the V1 demo boots with
  **touch working: 0 I2C read failures**.
- Run our ESRead firmware once. Touch dies (`probe 0x3B: ESP_ERR_NOT_FOUND`).
- Reflash the demo over ESRead with a **soft reset only** (no power drain). The demo's
  touch is now **broken too: 102 `write_read_dev` failures in 7s**.
- Cold-drain again and the demo is back to 0 failures.

Conclusion: **our firmware puts the AXS touch engine into a hung state that survives an
ESP soft reset (RTS) and only clears when the die loses power.** The touch hardware, the
FPC, and the vendor demo are all fine. The bug is ours, and it leaves persistent
die-internal state.

Note: the RTS reset (`rst:0x15 USB_UART_CHIP_RESET`) reboots the ESP32 but does not cut
power to the combo die, which is why every soft-reset recovery attempt failed and only a
physical unplug fixed it.

### Prime suspect: our extra `esp_lcd_panel_reset` SWRESET on the shared die

Compared our `Display::init_panel` (`components/display/display.cpp`) against the V1 demo
`10_LVGL_V9_Test/main/main.c` line by line. Touch pins/addr/reset are identical (I2C
18/17, addr 0x3B, die reset GPIO21, touch RST/INT both -1). The display init differs in
two ways that both land on the shared die:

1. **We call `esp_lcd_panel_reset(_panel)` (display.cpp:249); the demo never does.** That
   sends a SWRESET (0x01) over QSPI to the combo die *after* the GPIO21 hardware reset has
   already brought the touch engine up. On a shared die a controller SWRESET can halt the
   touch scan engine. Being die-internal, that state persists across an ESP soft reset and
   only clears on power loss — which is exactly the wedge we measured.
2. **Reset ordering.** The demo creates the panel object first, then pulses GPIO21
   (1/30ms, 0/250ms, 1/30ms) right before `esp_lcd_panel_init`, with a single reset. We
   pulse GPIO21 at the very start (before `spi_bus_initialize` exists), then add the extra
   SWRESET. The demo's die reset happens immediately before init; ours is separated from
   init by the whole bus/panel setup plus a second reset.

The `esp_lcd_panel_reset` was added deliberately (see comment at display.cpp:247) to fix a
*display* symptom — "lit but non-rendering panel after a power-button cycle." So this is a
**trade-off, not a clean bug**: that line likely fixes rendering and breaks touch. Needs a
human design call, not a blind removal.

### Proposed fix to test (needs hardware + a person)

Match the demo's exact sequence in `init_panel`: create the panel object first, then do
the single GPIO21 pulse right before `esp_lcd_panel_init`, and **drop
`esp_lcd_panel_reset(_panel)`**. Then verify BOTH:

1. Touch works — cold-drain first, boot, tap the screen, confirm presses register.
2. The original display symptom does not return — power-button cycle (not just RTS
   reset) a few times, confirm the panel still renders. If it regresses, we need a touch-
   safe way to clear the display state (e.g. re-pulse GPIO21 instead of SWRESET, or send
   the display SWRESET before the GPIO21 pulse so touch re-inits after it).

Every test must start from a cold power drain, or a stale wedge from the previous boot
will mask the result.

### Secondary finding: GPIO8 and the phantom TCA9554 (V1/V2 wiring mismatch)

The V1 demo defines `GPIO_NUM_8` as `BK_LIGHT` (backlight) and **references no TCA9554 /
0x20 / EXIO anywhere**. Our firmware treats GPIO8 as a generic `PERIPHERAL_POWER` rail and
routes backlight (EXIO1), power latch (EXIO6) and audio (EXIO7) through a TCA9554 at 0x20.
On this V1-behaving board our `Power::init` fails at the first expander write
(`EXPANDER_IO: ESP_ERR_INVALID_STATE`) — consistent with no expander answering at 0x20.
This is the V1/V2 mismatch flagged earlier in this doc. It is probably not the touch wedge
(the demo drives touch fine while using GPIO8 as backlight, and we drive GPIO8 high
anyway), but it means our power-latch / battery-hold and audio-enable paths do not work on
this board and need reconciling with the real board revision separately.

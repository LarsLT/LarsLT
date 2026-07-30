# Power and battery (M9 off-button + M10 USB-C charging)

Combines the former `off-button-power` (M9) and `usb-c-charging` (M10) plans. They share one
foundation: the ESP I²C bus and the TCA9554 expander, neither of which exists in the project yet.
Doing them as one feature avoids standing up that foundation twice.

## Why

The author wants to run the device on its battery with a tidy desk: plug in to charge, press the
button to turn off. Two things make that real:

- **Hold power on battery + power off on demand** — the board gates power through the TCA9554. On
  battery, power only stays on if firmware latches the expander; the off button should cut it.
- **Charge over USB-C and show battery level** — charging itself is automatic; the device just
  needs to read and display the battery.

## Wiring (resolved from vendor `07_BATT_PWR_Test` + `01_ADC_Test`, ESP-IDF versions)

Pulled from `waveshareteam/ESP32-S3-Touch-LCD-3.49` on GitHub (not in our tree). All verified:

- **ESP I²C bus:** I2C_NUM_0, SCL **GPIO48** / SDA **GPIO47**, internal pull-ups, glitch filter 7.
  Serves the TCA9554 now, IMU + RTC later.
- **TCA9554 expander:** address **0x20** (`..._ADDRESS_000`).
- **Power latch = EXIO6 (output).** Drive **HIGH at boot** to hold power on battery; drive **LOW**
  to power the device off. Without the boot latch, the board drops power when the PWR button
  releases on battery.
- **PWR button = GPIO16**, active-low, internal pull-up. Long-press is the off gesture. **BOOT =
  GPIO0.** Buttons are real GPIOs, not on the expander.
- **Power-off is a battery-mode action.** Vendor gates it on GPIO16 high at boot (battery present);
  on USB the device stays on. We match that.
- **Battery sense = GPIO4 = ADC1 channel 3** (verified against the IDF S3 ADC channel map). Config:
  `ADC_ATTEN_DB_12`, 12-bit, curve-fitting calibration. Volts = `raw_mV * 0.001 * 3` — on-board
  **×3 divider** (200K/100K).
- The ADC example drives **EXIO1 LOW** before reading, likely a battery-measurement enable. Confirm
  on device whether it is required for a valid reading.
- **USB-C charging is automatic in hardware.** No firmware charge-enable bit exists anywhere; the
  charge IC does CC/CV when USB is present. (An earlier draft guessed EXIO6 enabled charging — it
  does not; EXIO6 is the power latch.)
- **Battery power needs TWO expander lines high: EXIO7 and EXIO6.** The single-purpose
  `07_BATT_PWR_Test` demo only drives EXIO6, which is enough on USB (USB powers the rail) but NOT on
  battery. The vendor **factory** firmware (`11_FactoryProgram`) drives EXIO7 and EXIO6 high
  together; EXIO7 is the battery-rail enable. With only EXIO6 high, the device runs on USB but dies
  the instant USB is unplugged even with a charged battery. Verified on device 2026-06-29: adding
  EXIO7-high fixed it. Power-off still releases EXIO6 only (factory does the same).
- **Power-on is by button press, not USB.** On battery the device is off until a deliberate button
  press; the hardware holds power long enough for firmware to grab the latch (~960ms). The author
  prefers this (a press is deliberate, a tap could be accidental).

## Approach

Three layers, foundation first.

1. **Shared foundation in `components/board`** (unblocks everything, including future IMU/RTC):
   - `BOARD::esp_i2c_bus()` brings up the ESP I²C master bus (I2C_NUM_0, SCL48/SDA47) once and
     returns the same handle after, so power/RTC/IMU share it without a global in `main`.
   - Pins, address and EXIO lines are `constexpr` in `board`, never inlined.

2. **Off-button power-down (M9), split into two single-purpose components:**
   - `components/power` (`Power`): a native **TCA9554 driver** (no new dependency) that controls the
     rail. `init()` latches **EXIO7 and EXIO6 HIGH** so the device holds power on battery (EXIO7
     enables the rail, EXIO6 is the latch); `power_off()` drives **EXIO6 LOW** to cut power.
   - `components/button` (`Button`): a generic active-low long-press watcher on a GPIO; fires a
     callback once per hold past the threshold.
   - `main/` glues them: a long press flushes the bookmark, waits a grace window, then calls
     `power.power_off()`. Periodic SD writes (M2) remain the persistence guarantee since a dying
     battery can cut power without the button.
   - A failed power-off path degrades and logs, never panics; reset/boot buttons stay untouched.

3. **Battery + charging (M10):**
   - A `battery` reader on GPIO4 (ADC1 ch3, 12dB, 12-bit, curve-fit, ×3 divider) → voltage + coarse
     percent.
   - Infer charging vs discharging (no charge-status line is exposed); feed it to the on-screen
     battery icon. The icon depends on display/M4 UI work, so the reader can land before the glyph.

## Steps

Foundation + M9 first (the "point 1" the author wants now); M10 follows.

- [x] Resolve wiring from the vendor examples (documented above).
- [x] `board`: ESP I²C bus pins (SCL48/SDA47), TCA9554 addr, EXIO7 enable, EXIO6 latch, GPIO16
      button as `constexpr`; `BOARD::esp_i2c_bus()` owns the shared bus handle.
- [x] `power` component: native TCA9554 driver (OUTPUT/CONFIG regs), `init()` latches EXIO7+EXIO6
      high, `power_off()` releases EXIO6.
- [x] `button` component: generic active-low long-press watcher, fires once per hold.
- [x] `main/`: `power.init()` + `power_button.init(...)`; the long-press callback flushes the
      bookmark via a thread-safe `TextPipeline::request_flush()` (FLUSH command), then powers off.
- [x] On device (2026-06-29): unplug USB → stays on battery; long-press powers down; button press
      boots on battery. Battery measured 4.20V (GPIO4 ADC, ×3 divider) confirming the sense path.
- [ ] `battery` reader: GPIO4 / ADC1 ch3; confirm whether EXIO1-low is needed for a valid reading.
- [ ] Feed level + inferred charging state to the M4 battery icon when that UI lands.
- [ ] On device: plug USB-C, percentage rises / icon shows charging; unplug, it tracks discharge.

## Decisions

- Off-button + power latch is built **first** because latching EXIO6 high at boot is what makes the
  battery usable at all (the desk-cleanup goal). Charging is automatic, so M10 mostly waits on the
  battery icon UI.
- Latch is **drive-EXIO6** (TCA9554), not a held GPIO. Boot latches HIGH; off-event drives LOW.
- Power-off is a battery-mode action; on USB the device stays on (matches vendor).
- TCA9554 access is a **native driver in `board`** — no new managed dependency.
- Charging is automatic in hardware; firmware only reads and reports, never enables a charge path.
- Battery sense is GPIO4 (ADC1 ch3), ×3 divider; reuses the board I²C/expander foundation.

## Open questions

- Does **EXIO1** have to be driven low for a valid battery reading, or is the divider always live?
  Confirm on device.

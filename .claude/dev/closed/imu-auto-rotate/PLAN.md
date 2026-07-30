# IMU auto-rotate (M8)

## Why

The long strip can be held either way up. The focal word must always read right-side-up so the
reader never has to think about orientation. The QMI8658 is wired on the ESP I²C bus; v1 already
plans an `imu` component for orientation. This milestone makes the display follow it.

## Scope

Depends on **M4** (display + LVGL) and an `imu` component reading the QMI8658. In: read gravity
vector / orientation, map to a 0°/180° (and optionally 90°/270°) display rotation, apply it to the
LVGL display with hysteresis and debounce so small tilts don't flip the screen mid-word. Out: tap /
flick IMU gestures (post-v1), per-app rotation lock UI, any motion beyond up/down orientation.

## Approach

- `imu` component (class `Imu`) over the new I²C master driver on the ESP I²C bus (SCL 48 / SDA 47).
  Confirm the QMI8658 address and the register read against the vendor example before coding; a
  failed probe **degrades and logs** (no panic) — the screen just stays at its default rotation.
- `main/` polls or subscribes to orientation, debounces (e.g. require a stable reading for a short
  window + an angular threshold) and calls the LVGL rotation setter. Avoid flipping while a word is
  on screen if it would cause a visible jolt; rotate on the next frame boundary.
- Orientation logic lives in `main/`; `imu` returns raw orientation, `display` just applies a
  rotation. Components stay leaves.

## Steps
- [x] `imu` component, one `imu.{hpp,cpp}` pair: the QMI8658 driver over the shared
      ESP I2C bus, exposing `read_accel` as the raw `AccelG` gravity vector.
- [x] `imu` init probes WHO_AM_I at 0x6B then 0x6A, keeps the one that answers 0x05,
      configures the accelerometer, degrades and logs on a failed probe.
- [x] `display` gains `set_flipped`, a thread-safe 90/270 rotation swap.
- [x] `main/autorotate` `AutoRotate` class: owns the IMU, polls it, debounces with a
      dead-zone + sample count, flips the display on a stable change. Off if probe fails.
- [ ] On device: flip the board, screen reorients within the debounce window, no
      mid-word flicker. Confirm the gravity axis sign and the QMI8658 address.

## Decisions
- `imu` returns the raw gravity vector; the orientation policy (`Orientation` enum,
  dead-zone hysteresis, debounce) and the display flip live in `main/autorotate`.
- `AutoRotate` is a main-side class, not a component: it needs both `imu` and
  `display`, and components stay leaves.
- Probe failure degrades to the default rotation, never panics.
- 2-way (up/down) only for v1. The orientation enum can grow to 4-way later.
- QMI8658 register map from the datasheet, address confirmed at runtime by WHO_AM_I
  rather than hardcoded, since no vendor example is in the tree.
- No host test for the debounce: review chose simpler in-glue code over the extra
  component split that a host test needs. Covered by the on-device check instead.

## Open questions
- Gravity axis + sign for the landscape hold: first guess is accel.y, confirm by
  flipping the board on hardware. One-line change if wrong.
- 250 Hz / +/-4g accel config is fine for gravity sensing; revisit only if noisy.

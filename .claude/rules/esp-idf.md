# ESP-IDF / FreeRTOS rules

## I1. Verify the API exists

Before writing `esp_*_foo()`, confirm it exists in *this* IDF version:
```bash
grep -rn "esp_xxx_foo" /home/lqrslt/esp-idf-v5.5.4/components/
grep -rn "esp_xxx_foo" managed_components/
```

`dependencies.lock` pins versions; do not assume newer-version APIs are available.

## I2. Drivers

- Use the **new I2C master driver** (`driver/i2c_master.h`), not the legacy `driver/i2c.h`.
- Use the **new SPI master driver** (`driver/spi_master.h`).
- For GPIO, prefer the typed `gpio_config_t` API over raw register writes.

## I3. Tasks

- `xTaskCreatePinnedToCore` — always pin unless you have a reason not to. Default to `APP_CPU_NUM` for app logic, `PRO_CPU_NUM` for IDF/wifi.
- Stack sizing (see cheatsheet): polling 4K, JSON 6K, TLS 8K+, display 12K+.
- Never `vTaskDelay(0)` — use `taskYIELD()` if you want to yield. Use `vTaskDelay(pdMS_TO_TICKS(ms))` for waits.
- No blocking calls inside ISRs. ISR-safe variants end in `FromISR`.

## I4. Synchronization

- Cross-task signaling → `EventGroup` or `Queue`, not globals + busy-wait.
- Counters protected by `portMUX_TYPE` or a mutex, not `volatile`.

## I5. HTTP / TLS

- Buffers passed to `esp_http_client_set_post_field` must outlive `esp_http_client_perform`.
- TLS root CAs live inline in the component that uses them. Document the expiry in a comment.
- `timeout_ms` is required — never leave it at the default for production endpoints.

## I6. Kconfig

- New tunables → `main/Kconfig.projbuild`, not hardcoded.
- Default values for secrets are placeholders (`"No Key"`, `"No ID"`). Real values live in user's `sdkconfig` (gitignored).
- Read with `CONFIG_FOO` from `sdkconfig.h`.

## I7. Component registration

```cmake
# components/foo/CMakeLists.txt
idf_component_register(
    SRCS "foo.cpp"
    INCLUDE_DIRS "include"
    REQUIRES "esp_http_client" "json"     # public deps
    PRIV_REQUIRES "other_component"        # private deps
)
```

- `REQUIRES` if header exposes the dependency's types.
- `PRIV_REQUIRES` if dependency is only used in `.cpp`.

When `main/` uses a new component, add it to `main/CMakeLists.txt::PRIV_REQUIRES`.

## I8. Partition tables

The project uses ESP-IDF defaults. Custom partitions require coordination — do not modify `partitions.csv` (when added) without explicit approval.

## I9. OTA

Not yet implemented. When added, design must keep refresh tokens valid across update — they live on SD, not the firmware partition, so this is mostly free.

## I10. arduino-as-component

`arduino-esp32` is a library, not the runtime. There is no `setup()` / `loop()` in this project. Do not include `Arduino.h` unless you are using a specific Arduino API (e.g. `Serial`) — prefer the ESP-IDF equivalent.

## I11. Hardware target

ESRead targets one board: Waveshare ESP32-S3-Touch-LCD-3.49" — 172×640 IPS LCD long strip, capacitive touch, QMI8658 IMU, RTC, audio codec. Display + touch + IMU wired. It's connected now.

Implications:
- A peripheral probe (display, touch, IMU) can fail at runtime. Degrade and log, don't panic at boot.
- Pin numbers, I²C addresses, framebuffer sizes — all `constexpr`, never hardcoded in the middle of a function.

# Error handling — HARD RULES

## E1. `std::expected` everywhere

Every fallible function returns `std::expected<T, E>`.

- `T` is the success type. Use `void` if there is no value.
- `E` is the component's error enum or error struct.

```cpp
std::expected<std::string, MyError> read_token();
std::expected<void, MyError> write_token(const std::string& tok);
```

## E2. Error type shape

- **Simple components:** `enum class XXX_ERRORS { ... }` with SCREAMING_SNAKE values.
- **Rich context needed:** `struct XxxError { enum class Type {...}; std::string context; std::string to_string() const; };` (see `SDError`).

The error enum lives inside the class or namespace it belongs to — no global error enums.

## E3. Propagation

```cpp
auto sub = do_sub_step();
if (!sub)
{
    ESP_LOGE(TAG, "sub failed: %d", (int)sub.error());
    return std::unexpected(sub.error());
}
```

Or with C++23 monadic ops:
```cpp
return do_sub_step()
    .and_then([](auto x) { return next_step(x); });
```

## E4. Forbidden patterns

- `optional<vector<T>>` — vector already conveys emptiness. Pick one signal.
- Returning `{}` to mean "error" — use `std::unexpected`.
- Returning bool + out-param — use `expected<T, E>`.
- `try`/`catch` — exceptions are disabled in ESP-IDF builds anyway.
- Swallowing errors silently — at minimum log them. Preferably propagate.

## E5. Logging on error

Log at the level matching severity:
- `ESP_LOGE` — operation failed, caller will need to recover.
- `ESP_LOGW` — degraded but continued.

Don't log AND return — pick the layer that owns the message. Usually the caller logs because it has more context.

## E6. ESP-IDF interop

ESP-IDF returns `esp_err_t`. Convert at the boundary:

```cpp
esp_err_t err = esp_wifi_connect();
if (err != ESP_OK)
{
    ESP_LOGE(TAG, "wifi connect: %s", esp_err_to_name(err));
    return std::unexpected(WIFI_ERRORS::CONNECT_FAILED);
}
```

Don't bubble `esp_err_t` up through the project's public API — it leaks IDF as a dependency.

## E7. `main.cpp` is the wall

Errors can stop propagating at `main.cpp` — log and decide (abort, retry, degrade). Nowhere else gets to silently consume errors.

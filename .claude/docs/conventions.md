# Conventions

Quick reference. The enforced version lives in `.claude/rules/`. This is the "why" + examples.

## Naming

| Thing               | Style                  | Example                            |
|---------------------|------------------------|------------------------------------|
| Class               | PascalCase             | `RSVPEngine`, `ParserEpub`, `SD`           |
| Namespace           | UPPERCASE              | `WIFI`, `NTP`                              |
| Error enum (name)   | UPPERCASE              | `RSVP_ERRORS`, `WIFI_ERRORS`               |
| Error enum (values) | SCREAMING_SNAKE_CASE   | `NO_DATA`, `PARSE_FAILED`, `OUT_OF_RANGE`  |
| Private member      | `_snake_case`          | `_cursor`, `_wpm`, `_last_persist_ms`      |
| Public method       | `snake_case`           | `next_word()`, `set_wpm()`, `seek()`       |
| Local var           | `snake_case`           | `result`, `word_count`                     |
| Log tag             | matches class/ns name  | `"RSVPEngine"`, `"WIFI"`                   |

## File layout per component

```cpp
// components/foo/include/foo.hpp
#pragma once
#include <expected>
// other deps...

class Foo  // or: namespace FOO {
{
public:
    enum class FOO_ERRORS { /* SCREAMING_SNAKE values */ };

private:
    // _snake_case members

public:
    Foo();
    std::expected<ReturnT, FOO_ERRORS> do_thing();
};
```

```cpp
// components/foo/foo.cpp
#include "foo.hpp"
#include "esp_log.h"

static const char* TAG = "Foo";

Foo::Foo() { /* ... */ }
```

## Brace style

Allman (opening brace on next line). Enforced by `.clang-format`.

```cpp
if (condition)
{
    do_thing();
}
else
{
    other_thing();
}
```

## Includes

Group order: standard library → ESP-IDF → managed components → project headers. Blank line between groups.

## Logging

- `ESP_LOGE` — only on actionable failures returned to caller
- `ESP_LOGW` — degraded operation that the user should know about
- `ESP_LOGI` — significant lifecycle events (`"start"`, `"connected"`)
- `ESP_LOGD` — per-call diagnostic, off by default

Don't log inside tight loops. Don't log secrets, tokens, or full HTTP bodies.

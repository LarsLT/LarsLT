---
name: component-scaffold
description: Scaffold a new ESP-IDF component (class-based or namespace-based) under components/<name>/, conforming to ESRead's conventions. Trigger when the user says "new component", "scaffold component", or runs /new-component.
---

# Component scaffolding

Generates the 3-file skeleton for a new component. Ask the user one question first if not clear from context: **class or namespace?**

- **Class** when the component holds state (connection, token, calibration, handle).
- **Namespace** when the component is a stateless utility.

## Files to create

### 1. `components/<name>/CMakeLists.txt`

```cmake
idf_component_register(
    SRCS "<name>.cpp"
    INCLUDE_DIRS "include"
    REQUIRES        # public deps — headers expose them
    PRIV_REQUIRES   # internal deps
)
```

### 2. `components/<name>/include/<name>.hpp`

**Class flavor (`<Name>` PascalCase, e.g. `Foo`):**

```cpp
#pragma once
#include <expected>
#include <string>

class <Name>
{
public:
    enum class <NAME>_ERRORS
    {
        NOT_INITIALIZED,
        OPERATION_FAILED
    };

private:
    // _snake_case members

public:
    <Name>();

    std::expected<void, <NAME>_ERRORS> do_thing();
};
```

**Namespace flavor (`<NAME>` UPPERCASE, e.g. `FOO`):**

```cpp
#pragma once
#include <expected>

namespace <NAME>
{
    enum class <NAME>_ERRORS
    {
        OPERATION_FAILED
    };

    std::expected<void, <NAME>_ERRORS> do_thing();
}
```

### 3. `components/<name>/<name>.cpp`

```cpp
#include "<name>.hpp"
#include "esp_log.h"

static const char* TAG = "<Name>";

// class flavor:
<Name>::<Name>()
{
    ESP_LOGI(TAG, "init");
}

std::expected<void, <Name>::<NAME>_ERRORS> <Name>::do_thing()
{
    return {};
}
```

## After scaffolding

1. Add the component to `main/CMakeLists.txt::PRIV_REQUIRES`.
2. Tell the user: "Skeleton ready. Define the public API surface before I wire it into `main.cpp`."
3. Do NOT call the new component from `main.cpp` until the user describes its behavior.

## Things to NOT do

- Do NOT add includes the user didn't ask for.
- Do NOT pre-populate error enums with speculative variants — only what the scaffold needs.
- Do NOT generate test files (project has no test harness yet).
- Do NOT add the component to `idf_component.yml` — that's for managed (external) components only.

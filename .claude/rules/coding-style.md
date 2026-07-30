# Coding style — HARD RULES

These are non-negotiable. Violating them is grounds for a review rejection.

## R1. Naming

- Classes: PascalCase
- Namespaces: UPPERCASE
- Error enums: UPPERCASE name (`GOOGLE_ERRORS`) + SCREAMING_SNAKE values (`NO_DATA`)
- Private members: `_snake_case`
- Methods & functions: `snake_case`
- Local variables: `snake_case`

## R2. File layout

- One component = one folder under `components/`
- Header in `include/<name>.hpp`, implementation in `<name>.cpp`
- `#pragma once` at top of every header — no include guards
- `static const char* TAG = "ComponentName";` at the top of every `.cpp` that logs

## R3. Brace style

Allman braces. Always. Even for one-line bodies — write the braces.

```cpp
// YES
if (x)
{
    do_it();
}

// NO
if (x) do_it();
if (x)
    do_it();
```

`.clang-format` enforces this — run it before commit.

## R4. Headers

- Public headers expose ONLY what callers need. No private structs leaking out.
- No transitive includes from public headers — include what you use directly.
- Don't `using namespace` in a header. Ever.

## R5. No raw `new` / `delete`

- Use stack allocation, `std::unique_ptr`, or static lifetime.
- ESP-IDF handles (`esp_http_client_handle_t`, etc.) are C — RAII-wrap them in a class destructor if you store one as a member.

## R6. No magic numbers

- GPIO pins, timeouts, buffer sizes → `constexpr` or Kconfig.
- Status code literals (200, 401) get named constants when checked in more than one place.

## R7. `auto` policy

- Use `auto` for iterator types, `std::expected` results, lambda return types.
- Avoid `auto` for primitive return values where the type is non-obvious to a reader.

## R8. Const-correctness

- Methods that don't mutate → `const`.
- Pass-by-`const &` for non-trivial types unless you need a copy.

## R9. No commented-out code

If it's not running, delete it. Git has the history.

## R10. No CI-only formatting commits

Run `clang-format -i` before staging. The post-edit hook does this automatically — don't fight it.

## R11. Comment style

- Keep comments short. One line where you can.
- Plain English. Say what the code does, not how clever it is.
- No em dashes in comments. Use a comma, a period, or a new line.
- No rule or milestone codes in comments. Never write `R5`, `S5`, `E3`, `I7`, `M0`..`M6`
  or `rule 3`. Say the reason in plain English. The rules live in `.claude/rules/`, not in
  the code, and milestones live in `.claude/dev/`.
- Skip the comment if the code already says it. Comment the why, not the what.
- Write like a person, not a generated summary.

```cpp
// YES
// retry once, SD mounts slow on cold boot
mount_sd();

// NO
// This function attempts to mount the SD card — note that the SD card
// subsystem can be slow to initialize on a cold boot, so we retry.
mount_sd();

// NO (rule/milestone codes leak the meta-process into the code)
// cap the read so a giant file can't exhaust RAM (S5); removed in M6
read_file();
```

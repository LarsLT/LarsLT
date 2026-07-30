# Error to_string() boilerplate reduction (parked)

> Parked 2026-06-29. A cleanup, not a v1 milestone. Keep hand-written `to_string()` for now;
> revisit when the number of error types makes the boilerplate annoying enough to pay for a
> shared mechanism.

## Why

Every rich error type (`struct XxxError { enum class Type {...}; std::string context;
std::string to_string() const; }`, see `rules/error-handling.md` E2) hand-writes a `to_string()`
that switches over `Type` and concatenates the context. As components grow this is repetitive and
easy to get out of sync when a new enum value is added but the switch isn't updated.

## What we want

Add a value once (the enum entry) and have its string name available to `to_string()` without a
second hand-maintained switch. Ideally a compile error if a value is added but not named.

## Sketch of approaches (pick at design time)

- **X-macro table.** Each error lists its values in one `#define XXX_ERRORS(X) X(OPEN) X(READ)...`
  macro; expand it once for the `enum class` and once for a `name_of(Type)` lookup. Single source
  of truth, no extra deps, ugly macro. Most idiomatic for embedded C++.
- **`constexpr` name array** next to the enum, indexed by the enum value, with a `static_assert`
  that the array size matches the enum count (needs a `COUNT` sentinel). Simple, but the assert is
  the only thing tying them together.
- **A small `to_string()` helper template** that takes the enum + a `std::array<const char*, N>`
  of names + the context string and formats `"<name>: <context>"` uniformly, so each error type
  only supplies its name table, not the formatting logic.

## Open questions
- Do we want the prefix format unified across all error types (`"<Type name>: <context>"`), or do
  some need bespoke messages? If unified, the helper-template option removes the most code.
- Is the macro ugliness acceptable given the no-`new`/no-clever-metaprogramming leanings of the
  codebase? Weigh against the `constexpr` array + `static_assert`.

## Out
- Not changing the `std::expected<T, E>` contract or the E2 error shape, only how `to_string()`
  is produced.

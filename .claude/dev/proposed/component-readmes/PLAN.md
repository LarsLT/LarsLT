# Per-component README (parked)

> Parked 2026-06-29. The author wants a README per component, but not a big-bang backfill of all
> existing components now. Set up the structure here; roll out as a convention when components are
> created or substantially changed, rather than writing 14 READMEs in one pass that then rot.

## Why

Components under `components/` are leaves with a clear public surface (one class or namespace, one
error type, a short list of deps). A small README per component makes that surface readable without
reverse-engineering the header, and gives the companion docs (`docs/components.md`) per-leaf detail.

## Rollout (when promoted to active)

1. Add the template below to the `component-scaffold` skill so every new component is born with a
   `README.md` stub.
2. Add a soft rule (likely in `rules/coding-style.md` or `docs/conventions.md`): a component README
   is written or updated whenever a component is created or its public API changes.
3. Backfill existing components opportunistically, when one is next touched, not all at once.

## Template (`components/<name>/README.md`)

```markdown
# <name>

One-line purpose. What this leaf is responsible for, and explicitly what it is not.

## Public API
- `Class::method(...) -> std::expected<T, E>` — what it does, when it fails.
- Key types the caller passes or receives.

## Error type
`XxxError` / `XXX_ERRORS` — the failure modes and what the caller should do with each.

## Dependencies
- REQUIRES: <public deps that leak through the header>
- PRIV_REQUIRES: <deps used only in the .cpp>

## Notes
- Hardware touched (pins, I2C address), degrade behavior, any gotcha worth one line.
```

## Constraints
- Keep it short. A README that restates the header verbatim is worse than none. Purpose + public
  API + error type + deps is enough; link to `docs/` for anything longer.
- Leaf rule still holds: the README documents the component in isolation, it does not describe how
  `main/` wires it.
- Same comment hygiene as code (no em dashes, no rule/milestone codes).

## Open questions
- README per component vs a single richer `docs/components.md` table. Decide whether the per-leaf
  README replaces or supplements the central doc (likely supplements: README = detail, doc = index).

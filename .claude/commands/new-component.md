---
description: Scaffold a new ESP-IDF component following ESRead conventions. Usage / new-component <name>
---

Create a new component named `$ARGUMENTS` under `components/$ARGUMENTS/` following the project's conventions.

Layout to produce:
```
components/<name>/
├── CMakeLists.txt
├── <name>.cpp
└── include/
    └── <name>.hpp
```

Use the skill `.claude/skills/component-scaffold/SKILL.md` for the exact templates (naming, error enum, `std::expected` signature, TAG declaration).

After scaffolding:
1. Add the component to `main/CMakeLists.txt` `PRIV_REQUIRES`.
2. Tell the user what to fill in (the public API surface — error variants, methods).
3. Do NOT add it to `main.cpp` until the user defines what it should do.

---
name: plan-interrogation
description: Interrogate the user before writing any plan, until every unknown that could break the work later is closed. Use when starting multi-step work, a refactor, a new component, a parser, an intake feature, or whenever a plan is needed. Surfaces hidden assumptions, dependency chains, and ESP-IDF/hardware failure modes early, then writes the plan to disk per the project rules.
---

# Plan interrogation

Do not start implementing, and do not write `PLAN.md`, until the design has no open unknowns that could waste time later. Drive that out by interrogating the user first.

## How to ask

- Use the **AskUserQuestion** tool for every question. No plain-text questions.
- **One question at a time.** Acknowledge the answer in one line, then ask the next.
- Give 2-4 concrete options that reflect realistic choices for this project, not generic yes/no. Put your recommended option first and mark it `(Recommended)` with a one-line reason.
- If the answer is discoverable in the repo (`managed_components/`, `/home/lqrslt/esp-idf-v5.5.4/components/`, existing components, `idf_component.yml`), find it yourself instead of asking.

## What to drive out before planning

Keep going until each of these is resolved for the task at hand:

1. **Goal and done.** What exact behavior proves this is finished? Tie it back to `.claude/docs/product.md` intent, not a vague "it works".
2. **Scope edges.** What is explicitly out of scope for this pass? Catch the "we'll also need X later" before it silently expands.
3. **Component boundary.** New component or change to an existing one? Components are leaves, glue lives in `main/`. Confirm where each piece lands.
4. **Error surface.** What can fail, and what `std::expected<T, E>` error type carries it? Name the enum/struct now, not mid-implementation.
5. **Hardware reality.** Touching display, touch, IMU, SD, BLE, Wi-Fi, audio? Which pins, I2C addresses, partitions, stack sizes? A failed peripheral probe must degrade and log, never panic. Confirm the degrade path.
6. **ESP-IDF API existence.** If the plan leans on an `esp_*`/driver API, confirm it exists in 5.5.4 before committing to it. New managed component or dependency needs explicit approval.
7. **Memory and build budget.** Will this fit the 1 MB app partition? Big buffers, TLS, framebuffers, parser allocations? Flag heap/flash cost now.
8. **Untrusted input.** Anything from a file, BLE write, or HTTP body is untrusted. What validates length/keys/allocations before use?
9. **Integration order.** What depends on what? Sequence the steps so nothing is built before the thing it needs.

Skip any item that genuinely does not apply, but say why rather than silently dropping it.

## When the interrogation is done

1. Give a short summary of every decision made.
2. Write the plan to `.claude/dev/active/<slug>/PLAN.md` (per project rule 7) before touching code, with the resolved decisions, the step order, and the open risks that remain.
3. Only then start implementing.

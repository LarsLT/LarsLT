# What the map should do while a rocket is actually flying

## The decision

`internal/sources/launches.go`, `aheadOfUs()` currently drops any launch whose feed
status is `Success`, `Failure`, `Partial Failure` or **`In Flight`**.

Dropping the first three is clearly right — they have already happened. `In Flight`
is the odd one out: it means the rocket is airborne *right now*, which is arguably
the single most interesting moment the launch layer will ever have, and the map's
response is to make it disappear.

## Why it is currently dropped

Because the alternative was worse. `In Flight` records carry the planned `net`,
which by then is in the past, so drawing one meant the map printing a confident
`01 Aug 02:00Z` for a rocket that left twelve minutes ago. Between "vanishes" and
"states a time that is wrong", vanishing was the safer default.

## Options

| | shows | lies about |
|---|---|---|
| **A. drop it** (current) | nothing | its existence — vanishes mid-ascent |
| **B. draw it unchanged** | pad + a past T-0 | the time |
| **C. draw it, no clock** | pad + "lifting off" | nothing |

**C is recommended.** The status is a fact the feed gives us; the timestamp is the
thing that has gone stale. Print the fact, drop the stale part. It matches how
`Hold` is already handled — that one is kept but forced through the `Vague` path so
it prints a day instead of a clock.

## How small this actually is

`launchWhen()` in `main.go` already branches on `Launch.Vague` to decide between
`02 Jan 15:04Z` and `02 Jan (TBD)`. C is a third branch on the same function plus
letting `In Flight` through `aheadOfUs()`. Roughly ten lines and a test case in
`TestLaunchesDropsFlightsThatAlreadyWent`, which already tables every status.

## Why it may not be worth doing at all

`In Flight` is set for a window of minutes, and those records almost always carry a
`net` already in the past — so `at.Before(now)` filters them **before** status is
ever consulted. For this to fire you need a rebuild to land inside the ascent
window *and* the feed to still hold a future `net`.

At a 30-minute cron that is a rare coincidence. The captured feed of 60 records
contained no `In Flight` entry at all (`TBD` 42, `Go` 11, `TBC` 5, `Success` 2).

So: correct, cheap, and it will almost never be seen. Worth doing only if the idea
of the map going blank at liftoff bothers you more than the ten lines cost.

## Landed — 2026-07-31

Option C, in `feat: keep a rocket on the map while it is climbing`.

**Bigger than the ten lines, on purpose.** The section above is right that
`at.Before(now)` fires before status is ever read — but that makes letting
`In Flight` through `aheadOfUs()` alone a no-op in every case anyone would
actually see. Built as described it would have gone blank at liftoff exactly as
before, which is the one thing this plan exists to stop. So the past-T-0 test
now yields to the status:

```go
flying := r.Status.Abbrev == statusInFlight
if at.Before(now) && !(flying && now.Sub(at) < ascentWindow) {
    continue
}
```

`ascentWindow` is 30 minutes. It has to be bounded: `aheadOfUs`'s own comment
notes the feed holds a launch for hours after liftoff, and a cached feed may
stand in for the live one for a day, so an unbounded rule would fly one rocket
until the cache expired.

`aheadOfUs` was renamed `worthDrawing` — it no longer means "still to come" now
that an airborne rocket passes it.

**Shape of it.** `Launch.Flying` carries the fact; `launchWhen()` gained a branch
ahead of `Vague` that prints `lifting off` and no time.

**Tests.** `TestLaunchesDropsFlightsThatAlreadyWent` gained a `flying` column and
now expects `In Flight` drawn. New `TestLaunchesDrawsARocketThatIsAirborne` covers
a T-0 just past, one inside the window, one beyond it, and a past T-0 without the
status. New `main_test.go` holds `TestLaunchWhen`, the first test in the main
package — the label branch had nowhere else to live.

**Verified.** `go test ./...` green, `gofmt`/`go vet` clean. Real and `-offline`
builds both well-formed XML with zero `<script>`/`<animate>`.

One gap worth naming: there is no end-to-end proof that `lifting off` reaches the
SVG. The cache is only a fallback, so a patched cache file is overwritten by the
live fetch, and `-offline` skips the layer entirely — there is no way to feed the
real binary a synthetic launch without making `launchesURL` injectable. Selection
and label are each unit-tested; only their meeting is not.

The live feed carried no `In Flight` record at the time this landed (`TBD` 44,
`Go` 11, `TBC` 5), so the profile looks identical today. As predicted.

## Related, already decided

The **launch ascent arc** (`../launch-arc/PLAN.md`) is the feature that would make
liftoff actually visible. If that gets built, revisit this — an arc is far more
useful when the map admits the rocket is on it.

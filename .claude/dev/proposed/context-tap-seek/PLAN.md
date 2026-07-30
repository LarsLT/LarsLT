# Tap a context word to seek (v2 M2)

> Phase A cheap win in `docs/v2.md`. Builds directly on the v1 paused context strip.

## Why

The paused context strip (shipped in v1's home-menu work) shows the words around the current one
but is display-only: you read back with the sentence / paragraph buttons. Letting a tap on any
shown word jump the reading position straight there would make skipping and rewinding far more
direct, the strip becomes a scrubber.

## Scope

In: per-word touch targets on the paused strip, a seek-to-arbitrary-word path through the pipeline
and stream, resume playing from the tapped word. Out: everything the M6 strip already delivers.

## Sketch

- Each word in the strip is its own clickable LVGL object carrying its `StreamPosition` (or an index
  into the context window the pipeline handed up).
- `TextStream` already has `seek(StreamPosition)`; the window returned by `context_window` must
  carry each token's position so the tap has a concrete target.
- `TextPipeline::seek_to(StreamPosition)`: enqueue a command, the tick task seeks the stream, renders
  that word paused, refreshes the strip around the new spot.
- Center tap then plays from the new position, unchanged.

## Precondition already met (2026-07-04)

The resume-stale-position bug this plan wanted to fold in is fixed on `development`
(`439c140`): `TextStream::seek()` now sets `position()` to the seek target, guarded by the
host test `seek_updates_position_to_target`. Tap-to-seek can rely on a truthful position
after any seek. One follow-on to verify here: after a tap-seek into a paragraph, the
rewind rings only know boundaries replayed inside the target paragraph, so back-nav
across earlier paragraphs right after the jump is limited until words are read.

## Open questions

- Positions must survive the round trip (UI holds a `StreamPosition` while the tick task owns the
  stream). Copy by value into the widget user-data or an index table; do not hand out a live pointer.
- Touch target size on a 640-wide strip with many small words, may need a minimum hit width.

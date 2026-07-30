# How the book is read, and chunk-size tuning (parked)

> Parked 2026-06-29. Answers the author's question about how a book is read into memory, and
> records an optional tuning task. No v1 behavior change needed: the current path already reads in
> chunks, not byte-by-byte off the SD card.

## The question

Is the whole book loaded at once, is the SD read on every character (constant tiny reads), or is it
loaded in bigger chunks? The author wants bigger chunks, to spare the SD card and stay within RAM.

## The answer (current behavior)

The book is **never** fully loaded. `parser_txt` opens the file with stdio (`fopen`) and pulls one
paragraph at a time via `next_paragraph()`. That loop calls `fgetc()` per character, but `fgetc`
does **not** hit the SD card per character: the C library keeps an internal buffer (`BUFSIZ`,
typically a few KB) and refills it from the card in block-sized reads. So the SD is already read in
chunks; only the in-RAM buffer is walked byte by byte. `text_stream` then drains paragraphs from
the parser, so at most a paragraph (capped at `MAX_PARAGRAPH_BYTES` = 64 KB) plus the stdio buffer
is resident. This matches the intended "bigger chunks" model.

`.epub` (added after this was parked) is chunkier still: `ParserEpub` inflates one whole spine
entry (a chapter, capped at `MAX_XHTML_BYTES` = 1 MB) into RAM in a few large POSIX reads, then
serves paragraphs from that buffer with no SD I/O until the next entry. Nothing to tune there.

## Optional tuning (only if profiling shows SD churn)

- **`setvbuf` a larger buffer.** After `fopen`, set an explicit buffer (e.g. one FAT cluster, a few
  KB to tens of KB) so each SD read pulls more at once. Measure first; the default `BUFSIZ` is
  usually fine and a bigger buffer costs RAM.
- **Confirm stdio vs POSIX.** The `sd_io_gotchas` memory notes FATFS short reads and a bad
  backward-seek-after-EOF with FATFS, and recommends POSIX `lseek`/`read` for some paths.
  `parser_txt` uses stdio `fseek`/`ftell` for `seek()`/`byte_offset()`. Verify the resume seek path
  is not exposed to that gotcha (it seeks backward to a paragraph start, which has worked in M1/M2
  tests). If it ever misbehaves on hardware, switch the parser read path to POSIX `read`/`lseek`
  with an explicit chunk buffer. `ParserEpub` already made exactly that switch (unbuffered POSIX
  `_fd` reads) because the ZIP layout seeks backward constantly, so the precedent is in-tree.

## Out
- No change to the `StreamPosition` / resume contract. This is read-strategy only.

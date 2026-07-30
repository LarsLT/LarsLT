# Companion protocol + intake (v2 Phase B — parked)

> **Deferred out of v1 (2026-06-29); scoped as v2 Phase B (2026-07-04).** v1 shipped and loads
> books by SD-card sideload into `/sdcard/books/`. This is the on-device spine of the companion,
> now split across four v2 milestones in `docs/v2.md`: **M5** pairing + Wi-Fi provisioning,
> **M6** BLE chunked transfer, **M7** Wi-Fi LAN transfer + mDNS, **M8** `total_words` metadata.
> The hybrid BLE/Wi-Fi design, pairing, mDNS, and atomic temp-then-rename below still hold; when a
> milestone goes active it gets its own `dev/active/<slug>/PLAN.md` carved from this doc.
>
> The **Android companion app** that targets this protocol is a v2 deliverable in a **separate
> repo**; device firmware dev pauses after M8 to build it (it also does phone-side PDF → text), then
> resumes for Phase C sync. See `docs/v2.md`.
>
> Carry-over: `seed_sample_if_missing()` in `main/pipeline/text_pipeline.cpp` is **still present**
> (it seeds an embedded sample book on boot). Delete it when M6 lands — once real books arrive over
> intake, the seed is dead weight.

## Why

A non-technical user must receive a book without help (`product.md`). This is the device side of
the custom Android companion: pair once, then books land on the SD card and appear in the picker.
It is the headline of v2. Two-way sync and reading stats are Phase C and build on the pairing +
transport laid down here.

## Transport (gate resolved)

Resolved to a **hybrid, with Wi-Fi as the primary at-home path and BLE as the trust anchor +
offline fallback** (author decision, 2026-06-27):

- **Pair once over BLE.** Pairing derives an authentication secret stored in NVS, and provisions
  the home Wi-Fi credentials onto the device. BLE is always the trust anchor.
- **At home (common case): Wi-Fi LAN.** Both phone and device are on the same Wi-Fi. The device
  advertises over mDNS; the phone opens an authenticated socket and streams the book. Fast for
  multi-MB `.epub`s.
- **Away from home: BLE fallback.** No shared LAN → the phone sends the whole book over BLE GATT,
  chunked. Slower, but it works anywhere.

The same pairing-derived secret authenticates both paths.

## Scope

Depends on shipped v1: SD mounted with Wi-Fi soft-fail, the settings screen (hosts the "re-pair /
Wi-Fi setup" entry point, now the home bluetooth icon), and the picker (a newly received book must
show up). Also depends on **v2 M0** (flash budget / partition table). In: BLE pairing +
provisioning, BLE chunked book transfer, Wi-Fi LAN receive server, mDNS discovery, authenticated
write to `/sdcard/books/`. Out: **two-way sync** (position/library/stats — Phase C), reading
stats, resumable/interrupted-transfer recovery (v1 restarts a failed transfer), the Android app
itself (this plan specs the on-device protocol the app targets; the app is a separate project).

### Remove the sample-seed scaffolding when M6 lands
`seed_sample_if_missing()` in `main/pipeline/text_pipeline.cpp` writes an embedded sample book to
SD on boot so the harness had something to read before intake existed. **Delete it when M6 lands**
— once real books arrive over intake, the seed is dead weight.

## Approach

`main/` owns orchestration; the network/BLE work lands in leaf components. Everything from the
phone is **untrusted** (S5): authenticate before accepting, validate lengths, cap sizes, sanitize
filenames, write atomically.

### Dependencies (present, but mostly transitive — pin as direct deps when work starts)
- **NimBLE host** — `components/bt/` is in IDF 5.5.4 (BLE GATT server for pairing + transfer).
- **`network_provisioning`**, **`mdns`**, **`libsodium`** — all three are in
  `managed_components/` today, but only as **transitive deps of `arduino-esp32`** (checked
  2026-07-04; `main/idf_component.yml` does not list them). An arduino bump can drop or
  re-version them, so the first Phase B milestone must add the ones it uses as direct
  entries in `main/idf_component.yml`. Not a new-dependency approval in spirit, but call
  it out in that milestone's plan. Uses: provisioning carries Wi-Fi creds over BLE during
  pairing; mdns advertises `_esread._tcp`; libsodium derives/verifies the pairing secret
  and can AEAD the LAN socket.
- TLS for the LAN socket: a pairing-derived **PSK** (TLS-PSK via mbedTLS in IDF) avoids a cert
  bootstrap. Decide PSK-TLS vs libsodium AEAD over a plain socket at design time (both LAN-only,
  both keyed by the pairing secret). No plain HTTP for the transfer (S2).

### Pairing + provisioning — `components/companion_pair/` (class `CompanionPair`)
- BLE GATT service exposing a **pairing characteristic**. On explicit pairing (user taps "re-pair"
  on the M5 settings screen; not any GATT writer — S8), run a key agreement, derive the shared
  secret, store it in **NVS** (never SD, never logged — S3/S4).
- During the same flow, accept Wi-Fi SSID/PSK via `network_provisioning` → persist to NVS so the
  device joins the home LAN (this is also how an unprovisioned M0 boot gets online).
- **Reject unauthenticated GATT writes silently** — don't leak that a paired device is present (S8).
- **Error:** `struct PairError { enum class Type { BLE_INIT, AGREE_FAILED, NVS_WRITE,
  UNAUTHORIZED }; std::string context; std::string to_string() const; }` (E2).

### BLE transfer (offline path) — `components/book_intake_ble/` (class `BookIntakeBle`)
- GATT **book-transfer characteristic**: header (filename, total size, hash) then chunked body,
  every chunk authenticated against the pairing secret. Respect negotiated MTU; chunk accordingly.
- Stream chunks to a **temp file**, verify the hash, then atomically rename into `/sdcard/books/`
  so a partial transfer never shows a corrupt book in the picker.
- **Untrusted (S5):** cap total size, validate each chunk length before the SD write, sanitize the
  filename (FAT-illegal chars, no path traversal), bound the in-RAM chunk buffer.
- **Error:** `BookIntakeError` (shared shape) with `Type { UNAUTHORIZED, BAD_CHUNK, TOO_LARGE,
  HASH_MISMATCH, SD_WRITE, BAD_NAME }`.

### Wi-Fi transfer (home path) — `components/book_intake_wifi/` (class `BookIntakeWifi`)
- Advertise `_esread._tcp` over **mDNS** once on the LAN. Run a small **authenticated socket
  server** (PSK-TLS or libsodium-AEAD) that receives the same header+chunk framing, to the same
  temp-then-rename `/sdcard/books/` path. Reuse the `BookIntakeError` shape.
- **LAN-only** assumption documented (S2): the device only listens on the local network; the
  pairing secret authenticates the peer.
- `timeout_ms` set on the socket (I5); buffers outlive the transfer.

### `main/` glue
- After pairing, on boot: if Wi-Fi creds exist, connect (soft-fail per M0) and start the mDNS +
  Wi-Fi receive server; always keep the BLE transfer service available as fallback.
- On a completed transfer (either path): the new file is in `/sdcard/books/`; the M5 picker shows
  it on next open (or refresh). No bookmark yet → opens at the top with the default WPM.

## Steps
- [ ] Resolve PSK-TLS vs libsodium-AEAD for the LAN socket; confirm NimBLE + mDNS + provisioning
      sizes against the flash budget (see risk below). **Get author approval for any new dep.**
- [ ] `companion_pair`: BLE GATT pairing service, key agreement, secret → NVS, Wi-Fi provisioning,
      silent reject of unauthorized writes.
- [ ] `book_intake_ble`: chunked authenticated transfer → temp file → hash → atomic rename.
- [ ] `book_intake_wifi`: mDNS advertise + authenticated socket server → same temp/rename path.
- [ ] `main`: start servers post-pairing (Wi-Fi when online, BLE always); wire M5 "re-pair" entry.
- [ ] `idf.py build` green; on hardware: pair from the companion app, push a book over Wi-Fi at
      home and over BLE with Wi-Fi off; both appear in the picker and play; an unauthenticated
      write is silently rejected; a malformed/oversized push fails cleanly with no corrupt file.

## Decisions
- Hybrid transport: Wi-Fi LAN primary at home, BLE fallback away; BLE is always the trust anchor.
- One pairing-derived secret authenticates both transports; stored in NVS, never logged (S3/S4/S8).
- Atomic temp-then-rename into `/sdcard/books/` so partial transfers never surface in the picker.
- Reuse IDF/managed deps (NimBLE, network_provisioning, mdns, libsodium); no new managed dep
  without explicit approval (rule 5 / S6).
- Two-way sync and stats are Phase C; resumable/interrupted-transfer recovery is out of v2 (a
  failed transfer restarts).

## Carries the word-count metadata for picker progress / ETA

The picker's word-based progress % and time-to-finish were deferred from v1 to here because they
need a **total word count per book**, which can't be known without reading the whole book. The
companion side scans the book **once at upload** and ships `total_words` as metadata alongside the
file (and the bookmark gains a `total_words` field). With that:

- **Progress %** = words-read (from the resume position) ÷ `total_words`.
- **ETA** = (`total_words` − words-read) ÷ current WPM.

This is **v2 M8**. On-device fallback (a byte estimate) is not needed once intake provides the exact
count. v1 rows are title + in-progress order only; see the shipped picker in
`dev/closed/controls-picker-settings/PLAN.md`.

## Open risks (resolve before committing deps)
- **Flash budget: largely resolved by v1** (updated 2026-07-04). The custom `partitions.csv`
  already shipped in v1 with a **4 MB factory app slot** on the 16 MB flash; the v1 binary
  is ~1.3 MB (68% free). The radio stacks (NimBLE + Wi-Fi + provisioning + mDNS +
  TLS/libsodium, roughly 0.7–1 MB together) fit comfortably. v2 M0 shrinks to a quick
  confirm-by-building measurement, not a partition redesign.
- **Transport security choice** (PSK-TLS vs AEAD) is unresolved above — pick before writing socket
  code; both must key off the pairing secret (S2/S8).
- **Concurrent BLE + Wi-Fi coexistence** on the ESP32-S3 (shared radio) — verify throughput and
  stability when both stacks are up; the device may need to keep BLE advertising minimal while a
  Wi-Fi transfer runs.

## Verification
- First-run pairing from the companion app derives a secret and provisions Wi-Fi; the secret is in
  NVS and never appears in logs. At home, a multi-MB `.epub` pushes over Wi-Fi in seconds and shows
  in the picker. With Wi-Fi off, the same book pushes over BLE (slower) and shows in the picker.
  An unauthenticated GATT write gets no acknowledgement. A truncated or oversized transfer leaves
  no file in `/sdcard/books/` and does not crash the device.

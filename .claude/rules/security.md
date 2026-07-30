# Security rules

## S1. No secrets in code or git

- Wi-Fi creds, BLE pairing keys, any future API keys → Kconfig (`menuconfig`) or NVS.
- Kconfig defaults are placeholders: `"No Key"`, `"No ID"`, never real values.
- `sdkconfig` is gitignored — never `git add -f` it.
- If you ever paste a real secret into chat or commit, rotate it.

## S2. TLS

- Always use HTTPS for external APIs. No plain HTTP for any auth flow.
- CA certificate pinning: when fetching from an external endpoint, fetch its CA from the issuer's public bundle, not from random tutorials. Bundle the PEM inline in the component, document expiry in a comment.
- `esp_http_client_config_t::cert_pem` must be set for HTTPS. If it isn't, you're trusting any cert.
- `skip_cert_common_name_check` must NOT be set to `true` in production code.
- Book intake over Wi-Fi between the companion app and the device is LAN-only; still use TLS once we have a pairing-derived cert, otherwise restrict to a trusted local network and document the assumption.

## S3. On-device storage

- Books on SD at `/sdcard/books/`. Bookmarks at `/sdcard/bookmarks/` (or a single JSON, TBD).
- SD card is removable — treat physical access as compromise. Acceptable for current threat model (personal device); document if that changes.
- ESRead has **no OAuth refresh tokens** in v1. If a feature ever needs one (e.g. cloud sync), revisit this section before storing it.
- BLE pairing key / Wi-Fi PSK for the companion app pairing flow (when implemented) must not be logged. Persist via NVS, not plain SD.

## S4. Logging hygiene

- Never `ESP_LOGI(TAG, "token=%s", token)` — not even at DEBUG.
- Truncate or hash if you need to confirm "a token is present": `ESP_LOGI(TAG, "token len=%d", token.length())`.
- Full HTTP bodies often contain tokens / PII — log status code + URL, not body.

## S5. Inputs from the network

Anything coming from an HTTP response, BLE characteristic write, or parsed book file is untrusted. Validate length before copying, check JSON/XHTML keys exist before dereferencing, cap allocations on parse. A malformed `.epub` should fail the parser cleanly, not crash the device.

## S6. Build / supply chain

- No new managed components without checking the publisher.
- `dependencies.lock` is committed — review diffs to it like code.
- `arduino-esp32` is huge; do not add full Arduino libraries without specific need.

## S7. Wi-Fi credentials

Currently in Kconfig (build-time). Long-term: move to NVS so creds don't require a rebuild — ideally provisioned by the companion Android app during BLE pairing. When that happens, document the provisioning flow in `docs/`.

## S8. Companion-app pairing (planned)

When BLE intake lands, the device must:
- Require explicit pairing (not just any GATT writer can push books).
- Derive an authentication secret during pairing, store it in NVS.
- Reject unauthenticated GATT writes silently — don't leak that a device is present-and-paired.

Design this before writing the GATT server. See `dev/active/companion-protocol/` when it exists.

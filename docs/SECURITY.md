# Security notes

## Implemented in the MVP

- Short-lived signed capture sessions.
- HMAC-SHA256 session tokens.
- Atomic one-time session consumption to block concurrent replay of a valid capture session.
- Randomized per-session illumination challenge with enforced high/low transitions.
- Server-side reconstruction of the expected challenge index from capture timestamps.
- Client-provided challenge index and light values are not trusted by the biometric engine.
- Capture timing and challenge coverage validation.
- Passive PAD is required for approval by default (`FACEPROOF_REQUIRE_PASSIVE_PAD=true`).
- AES-256-GCM encrypted biometric templates at rest.
- Raw camera frames are processed in memory and not persisted by the application.
- 1:1 verification only.
- Configurable liveness and face-match thresholds.

## Browser trust model

The browser is untrusted. WASM, JavaScript, timestamps, local quality metrics and browser metadata can all be modified by an attacker. They are useful for UX, telemetry and raising attack cost, but they are not a hardware-backed proof that frames came from a physical camera.

The API therefore remains authoritative for session state, challenge definition, challenge normalization and final decision orchestration.

## Not solved by this MVP

- Browser or OS camera injection.
- Virtual camera detection.
- Rooted/jailbroken device integrity.
- Advanced deepfake stream injection.
- 3D mask attacks.
- Replay attacks tailored to the randomized screen illumination sequence.
- Application/tenant authentication for session creation.
- Distributed rate limiting.
- Hardware-backed key storage.
- Certified PAD evaluation.

## Production direction

Before high-assurance production use, add application/tenant authentication, rate limiting, audit events, KMS/HSM-backed template keys, a governed attack dataset and independent PAD evaluation. Native mobile SDKs with device attestation remain the higher-assurance option for flows where browser camera provenance is insufficient.

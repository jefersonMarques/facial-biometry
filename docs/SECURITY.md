# Security notes

## Implemented in the MVP

- Short-lived signed capture sessions.
- Server-generated capture `runId` bound to the session.
- HMAC-SHA256 session tokens bind `sessionId`, `runId` and expiration.
- Atomic one-time session consumption blocks concurrent replay of a valid capture session; concurrency tests assert exactly one consumer succeeds.
- Capture completion also has a minimum elapsed time measured by the server clock, independent of client timestamps.
- Randomized per-session illumination challenge with enforced high/low transitions.
- Server-side reconstruction of the expected challenge index from capture timestamps.
- Client-provided challenge index and light values are not trusted by the biometric engine.
- Capture timing and challenge coverage validation.
- Passive PAD is required for approval by default (`FACEPROOF_REQUIRE_PASSIVE_PAD=true`).
- AES-256-GCM encrypted biometric templates at rest.
- Raw camera frames are processed in memory and not persisted by the application.
- 1:1 verification only.
- Configurable liveness and face-match thresholds.
- Public identity endpoints have bounded in-memory rate limiting keyed by SHA-256 of the public token, with no cleartext token stored in limiter state.
- Identity completion enforces request/body/frame size limits and JPEG content type.
- Runtime fingerprint validation pins the supported SDK, liveness WASM, MediaPipe runtime and Face Landmarker build.
- API responses use no-store/no-cache, nosniff, frame denial and no-referrer security headers.
- Secure Core production runtime requires a signed FaceProof Model Pack; Ed25519 signature, model sizes and SHA-256 digests are verified before model loading.

## CNH Digital identity checks

The identity-check flow adds issuer-authenticated link creation, high-entropy public tokens stored only by SHA-256 hash, AES-GCM encrypted check state, strict PDF signature/full-document validation, metadata consistency checks, VIO signature verification, expected-CPF matching and signed-PDF freshness enforcement.

The uploaded PDF and VIO portrait are not persisted by the application. The portrait is converted into a reference embedding and discarded; the reference embedding is removed after the final identity decision.

PDF metadata is never the primary authenticity or freshness signal. The digitally signed PDF timestamp is used for the minimum-document-date rule, while metadata is treated as a consistency signal.

The current PDF trust profile is deliberately strict and must be regression-tested across legitimate CNH Digital PDFs from multiple DETRAN issuers. The MVP does not claim complete historical OCSP/CRL or qualified timestamp validation.

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
- Application/tenant authentication and tenant isolation.
- Distributed rate limiting across multiple API instances / edge gateway.
- Hardware-backed key storage and governed secret rotation.
- Certified PAD evaluation.

## Tenant isolation

Issuer authentication supports a tenant registry. Production tenant files store only SHA-256 hashes of tenant API keys; raw API keys are not persisted in the registry. Each newly created identity check is bound to a `TenantID` inside the encrypted transactional state.

Issuer status queries require authentication as the same tenant that created the check. Cross-tenant lookups intentionally return `404` so the API does not reveal whether another tenant's check ID exists. The private super-admin surface remains separate from tenant issuer access.

The legacy `FACEPROOF_IDENTITY_ISSUER_KEY` remains only as a development fallback mapped to the `default` tenant.

Each tenant may define `monthlyCheckLimit`. In the current single-instance Production Candidate, quota enforcement is performed inside the encrypted authoritative identity repository: quota counting and check creation share the same process lock, so concurrent requests cannot exceed the configured limit within that instance. The analytics database is not consulted for quota decisions. A future multi-instance deployment must move this control to a shared transactional control plane.

## Production direction

Before high-assurance production use, extend tenant isolation into analytics/billing/quotas, add distributed/edge rate limiting, KMS/HSM-backed runtime and signing keys, a governed attack dataset and independent PAD evaluation. Native mobile SDKs with device attestation remain the higher-assurance option for flows where browser camera provenance is insufficient.

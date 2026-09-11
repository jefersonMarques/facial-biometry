# Security notes

## Implemented in the MVP

- Short-lived signed capture sessions.
- HMAC-SHA256 session tokens.
- Randomized per-session illumination challenge.
- AES-256-GCM encrypted biometric templates at rest.
- Raw camera frames are processed in memory and not persisted by the application.
- 1:1 verification only.
- Configurable liveness and face-match thresholds.

## Not solved by this MVP

- Browser or OS camera injection.
- Virtual camera detection.
- Rooted/jailbroken device integrity.
- Advanced deepfake stream injection.
- 3D mask attacks.
- Replay attacks tailored to the randomized screen illumination sequence.
- Distributed rate limiting.
- Hardware-backed key storage.
- Certified PAD evaluation.

## Production direction

For high-assurance deployments, add native mobile SDKs, device attestation, server-side abuse controls, HSM/KMS-backed template keys, audit events, independent PAD testing and a dataset representative of the deployment population and devices.

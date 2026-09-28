# Architecture

## Trust boundary

The browser is treated as an untrusted capture client. It may guide the user, perform local quality checks and render the illumination challenge, but it never owns the final biometric decision.

The API is authoritative for session lifetime, one-time session consumption, illumination challenge values and frame-to-challenge mapping. Client-provided `challengeIndex` and `clientLight` values are overwritten before the biometric engine receives the capture.

## CNH Digital identity pipeline

```text
Issuer: expected CPF + minimum signed-PDF date
    |
    v
Opaque public link
    |
    v
CNH Digital PDF
    |
    +--> PDF signature / metadata integrity
    +--> VIO signature / CNH template / CPF
    +--> official embedded portrait
    |
    v
temporary reference embedding
    |
    v
browser liveness capture
    |
    v
CNH portrait <-> live face
    |
    v
approved / review / rejected
```

This identity flow does not require permanent biometric enrollment. Raw PDF bytes and the extracted portrait are discarded after the reference embedding is created, and that embedding is cleared after final completion.

## Capture pipeline

```text
Browser camera
    |
    +--> local brightness / contrast / sharpness preflight
    |
    v
Short multi-frame capture
    |
    +--> server-generated high/low randomized illumination pattern
    +--> timestamps + client quality telemetry
    |
    v
Go API
    |
    +--> signed session validation
    +--> strict timestamp / duration validation
    +--> server-side challenge normalization
    +--> atomic one-time session consumption
    |
    v
Python biometric engine
    |
    +--> YuNet detection + landmarks
    +--> frame quality
    +--> temporal motion
    +--> regional illumination response
    +--> MiniFASNet passive PAD
    +--> SFace embedding
    |
    v
Risk decision
```

## Enrollment

```text
POST /v1/biometric/enrollments
POST /v1/biometric/enrollments/{sessionId}/complete
```

A successful enrollment persists only the encrypted SFace embedding and metadata. Raw frames are not persisted by the MVP.

When `FACEPROOF_REQUIRE_PASSIVE_PAD=true`, an enrollment cannot be approved if passive PAD is unavailable.

## Verification

```text
POST /v1/biometric/verifications
POST /v1/biometric/verifications/{sessionId}/complete
```

A live embedding is compared to the enrolled template using cosine similarity. A capture session is single-use even if biometric processing later fails; the client must create a new session for another attempt.

## Browser-first direction

The browser SDK is intentionally split into capture, local quality analysis and API transport. This keeps the path open for a WASM runtime that can later provide face tracking, landmarks, pose, occlusion and richer capture guidance without moving the final trust decision to the browser.

CNH Digital identity checks are implemented as a separate product flow. Physical-document camera capture and OCR are intentionally outside the current scope. See `IDENTITY_CHECK.md` and `BROWSER_FIRST_ROADMAP.md`.

## Deliberate seams

The following components are isolated so they can be replaced without changing the product API:

- Face detector.
- Face encoder.
- Passive PAD model.
- Liveness fusion logic.
- Template repository.
- Capture challenge.
- Browser quality analyzer.

The intended path is to replace or ensemble third-party models only after collecting a governed dataset and building an evaluation harness.

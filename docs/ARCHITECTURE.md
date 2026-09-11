# Architecture

## Capture pipeline

```text
Browser camera
    |
    v
Short multi-frame capture
    |
    +--> randomized illumination pattern
    |
    v
Go API session validation
    |
    v
Python biometric engine
    |
    +--> YuNet detection + landmarks
    +--> frame quality
    +--> temporal motion
    +--> illumination response
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

## Verification

```text
POST /v1/biometric/verifications
POST /v1/biometric/verifications/{sessionId}/complete
```

A live embedding is compared to the enrolled template using cosine similarity.

## Deliberate seams

The following components are interfaces or isolated modules so they can be replaced without changing the product API:

- Face detector.
- Face encoder.
- Passive PAD model.
- Liveness fusion logic.
- Template repository.
- Capture challenge.

The intended path is to replace third-party models with proprietary models after collecting a governed dataset and building an evaluation harness.

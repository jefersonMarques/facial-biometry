# FaceProof product and browser-first roadmap

The FaceProof core product is facial biometric enrollment, liveness and 1:1 verification.
Document identity flows such as CNH Digital are optional modules and are not a dependency of the generic SDK.

The target core flow is:

```text
Customer application
    |
    v
FaceProof Browser SDK
    |
    +--> local face guidance
    +--> local quality pre-filter
    +--> local stability gate
    +--> C++/WASM geometry
    |
    v
selected JPEG blobs
    |
    v
FaceProof Server
    |
    +--> server-side face detection and quality
    +--> passive PAD
    +--> temporal/guided evidence
    +--> 1:1 face matching
    +--> final risk decision
    |
    v
approved / review / rejected
```

The trust rule remains:

> Browser filters and guides. Server validates and decides.

## Phase 1 - Web capture foundation

Status: implemented.

- Local brightness, contrast and sharpness preflight.
- Guided capture UX.
- Server-provided illumination settle timing for the legacy flow.
- Capture metadata and client quality telemetry.
- Server-side challenge normalization.
- Atomic single-use sessions.
- Passive PAD fail-closed by default.
- Architecture ready for a WASM capture runtime.

## Phase 2 - Optional CNH Digital identity module

Status: MVP implemented.

- Issuer creates a check with expected CPF and minimum PDF signing date.
- Opaque verification link.
- Original CNH Digital PDF only.
- PDF digital-signature validation.
- Full-document signature coverage requirement.
- Official signer and metadata consistency profile.
- VIO v1-v6 decode and signature validation.
- Expected CPF comparison.
- Embedded official portrait extraction.
- Reference SFace embedding.
- Browser liveness capture.
- CNH portrait vs live face 1:1 comparison.
- Issuer result lookup by check ID.
- Raw PDF and portrait are not persisted.
- Reference embedding is cleared after completion or expiration.

Physical CNH capture, generic OCR and document onboarding remain outside the SDK core.

## Phase 3 - Browser SDK local-first capture

Status: implemented / calibration in progress.

- MediaPipe supplies face landmarks locally.
- C++/WASM liveness geometry core.
- TypeScript x WASM parity validated to floating-point noise.
- Local face guide replaces continuous `/identity/guide` polling.
- Server guide retained as technical fallback.
- Local stability gate before capture.
- Required far-to-near scale separation.
- Browser quality remains a permissive pre-filter only.
- Final server-side quality and liveness remain authoritative.
- JPEG capture uses `canvas.toBlob()`.
- Final frames remain binary in browser memory.
- Browser to Go uses multipart JPEG blobs.
- Go to Python uses multipart binary frames.
- Python decodes JPEG bytes directly with OpenCV.
- No Base64 is required in the main identity-capture path.

## Phase 4 - Browser SDK hardening

- Self-host and pin MediaPipe JS, WASM and model artifacts.
- Restrictive Content Security Policy.
- Release artifact hashes and reproducible builds.
- Explicit SDK/WASM/model version reporting.
- Signed release packages.
- Explicit capture lifecycle events instead of inferring phase from UI text.
- Worker/OffscreenCanvas optimization when justified.
- Tamper/debug/browser-environment signals as telemetry only, never as final authority.
- Fuzzing of SDK message boundaries and WASM ABI.
- Compatibility matrix for supported browsers/devices.

## Phase 5 - Advanced liveness

- Multiple PAD models or calibrated ensemble.
- Optical-flow consistency.
- Landmark temporal analysis.
- Regional illumination response.
- Replay and repeated-frame detection.
- Screen/moire/specular artifact signals.
- Injection-oriented browser telemetry.
- Native SDK/device attestation option for higher-risk use cases.
- Native C++ geometry redundancy on the server.

## Phase 6 - Evaluation lab

Build a governed dataset and repeatable harness for:

- genuine captures across devices and lighting conditions;
- printed photos;
- photos displayed on phones and monitors;
- replayed videos;
- moved photos and moved screens;
- prerecorded far-to-near protocol attacks;
- partial occlusion;
- masks where operationally appropriate;
- deepfake and virtual-camera injection attempts;
- representative ages, skin tones, camera qualities and environments.

Measure at minimum FMR/FNMR for identity matching and APCER/BPCER for PAD.
Production thresholds and fusion weights must be calibrated from measured results rather than treated as fixed defaults.

## Phase 7 - Generic enrollment and verification product API

The SDK core must no longer depend on CNH.

### Enrollment

```text
Customer backend
    |
    | create enrollment(subject_id)
    v
FaceProof Server
    |
    | short-lived session
    v
Browser SDK
    |
    | liveness + selected frames
    v
FaceProof Server
    |
    | revalidate everything
    | generate encrypted template
    v
enrollment approved
```

### Verification / re-verification

```text
Customer backend
    |
    | verify(subject_id)
    v
FaceProof Server
    |
    | session
    v
Browser SDK
    |
    | liveness + selected frames
    v
FaceProof Server
    |
    | PAD + quality + 1:1 match
    v
approved / review / rejected
```

The CNH flow becomes an optional `FaceProof Identity` module that supplies an authenticated reference identity.

## Phase 8 - Commercial licensing and Control Plane

The commercial protection must live on the server, not in obfuscated JavaScript or WASM.

Planned entities:

- tenant;
- application;
- environment: sandbox / production;
- installation;
- license;
- entitlement;
- quota;
- usage ledger;
- release/version policy.

A signed production license should bind at minimum:

- tenant ID;
- application ID;
- environment;
- enabled features;
- allowed origins/package identifiers where applicable;
- issue and expiry dates;
- grace period;
- license ID;
- optional installation/server fingerprint.

The server validates the signed license before issuing new biometric sessions.

Usage metering should be signed and contain no biometric data or PII by default. Suggested usage events include:

- tenant;
- installation ID;
- timestamp;
- operation type;
- SDK/server version;
- technical result;
- signature.

### Deployment modes

1. **FaceProof Cloud** - FaceProof hosts the biometric server.
2. **Customer Hosted** - customer runs the biometric server with signed licenses and usage reporting.
3. **Hybrid** - biometrics stay inside the customer environment while FaceProof Control Plane manages licensing, entitlements, versions and anonymous usage accounting.

The hybrid model is the preferred enterprise direction.

## Phase 9 - FaceProof Console in Go + HTMX

The administration plane should stay in Go and HTMX to avoid introducing another application stack.

Roles:

- FaceProof superadmin;
- tenant admin;
- tenant viewer/auditor;
- restricted support role.

Initial screens:

- dashboard;
- verification list;
- verification technical detail;
- subjects/enrollments;
- applications;
- API keys and rotation;
- allowed origins;
- webhooks;
- usage/quota;
- license and expiration;
- SDK/server versions;
- retention configuration;
- audit log;
- users and permissions;
- model/service health.

Security requirements:

- strong password hashing;
- MFA mandatory for FaceProof superadmin;
- secure HTTP-only sessions;
- CSRF protection;
- rate limiting and progressive lockout;
- tenant-scoped RBAC;
- audited privileged actions;
- future OIDC/SAML support for enterprise customers.

## Phase 10 - SDK packaging and sample application

Every commercial SDK release should include:

- versioned Browser SDK;
- C++/WASM artifacts;
- integration documentation;
- changelog;
- compatibility matrix;
- sandbox configuration;
- production licensing instructions;
- example backend;
- example application.

Reference sample app:

```text
examples/
└── go-htmx/
    ├── cmd/server/
    ├── templates/
    ├── static/
    └── README.md
```

It should demonstrate:

- create subject;
- enrollment;
- verification;
- start Browser SDK session;
- receive webhook;
- query result;
- handle recapture;
- handle license/quota errors;
- sandbox vs production setup.

No API secret may be embedded in browser JavaScript.

## Phase 11 - Biometric data governance

Production verification data and product-improvement datasets must be treated as separate purposes.

Defaults:

- encrypted biometric templates;
- short retention for raw frames;
- automatic lifecycle/deletion;
- tenant-specific retention within policy limits;
- audit trail;
- separate controlled R&D dataset;
- explicit legal basis/consent/contractual permission before reuse for R&D;
- restricted access to evaluation datasets;
- deletion/export workflows when applicable.

The licensing Control Plane must not receive biometric images, biometric templates, CPF, name or other PII by default.

## Product trust and commercial boundary

```text
SECURITY BOUNDARY
Browser is untrusted.
Server recalculates and decides.

COMMERCIAL BOUNDARY
Distributed SDK does not own licensing.
Server + signed license + Control Plane own entitlement and usage.
```

WASM, minification and obfuscation raise reverse-engineering cost, but they are not the billing or licensing mechanism.

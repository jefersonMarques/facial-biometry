# Browser-first identity roadmap

The target product flow is:

```text
CNH Digital PDF
    |
    +--> PDF signature / metadata integrity
    +--> VIO signature / expected CPF / official portrait
    |
    v
Guided live face capture
    |
    +--> passive PAD
    +--> temporal signals
    +--> randomized illumination challenge
    |
    v
Live face <-> signed VIO portrait
    |
    v
approved / review / rejected + evidence
```

## Phase 1 - Web capture foundation

Status: implemented.

- Local brightness, contrast and sharpness preflight.
- Guided capture status.
- Server-provided illumination settle timing.
- Capture metadata and client quality telemetry.
- Server-side challenge normalization.
- Atomic single-use sessions.
- Passive PAD fail-closed by default.
- Architecture ready for a WASM capture runtime.

## Phase 2 - CNH Digital identity check

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

Physical CNH capture, front/back OCR and generic document onboarding are intentionally outside the current product scope.

## Phase 3 - Identity hardening

- Regression corpus of legitimate CNH Digital PDFs from multiple DETRAN issuers.
- Stronger ICP-Brasil long-term signature/revocation validation.
- Transactional shared identity/session store.
- Per-tenant issuer credentials and authorization.
- Distributed link/IP/device abuse controls.
- Cross-check document reuse analytics using PDF hashes.
- Audit events and trace IDs.
- Explicit retention/deletion policy.
- KMS/HSM key storage and rotation.

## Phase 4 - Browser capture quality

- Browser face tracking.
- Landmark stability.
- Head pose.
- Face size and centering guidance.
- Occlusion and eye-visibility checks.
- Web Worker/WASM execution boundary.

## Phase 5 - Advanced liveness

- Multiple PAD models or calibrated ensemble.
- Optical-flow consistency.
- Landmark temporal analysis.
- Regional illumination response.
- Replay and repeated-frame detection.
- Injection-oriented browser telemetry.
- Native SDK/device attestation option for higher-risk use cases.

## Phase 6 - Evaluation lab

Build a governed dataset and repeatable harness for:

- genuine captures across devices and lighting conditions;
- printed photos;
- photos displayed on phones and monitors;
- replayed videos;
- moved photos and moved screens;
- partial occlusion;
- masks where operationally appropriate to test;
- deepfake and virtual-camera injection attempts;
- representative ages, skin tones, camera qualities and environments.

Measure at minimum FMR/FNMR for identity matching and APCER/BPCER for PAD. Production thresholds and fusion weights must be calibrated from measured results rather than treated as fixed defaults.

# Browser-first identity roadmap

The target product flow is:

```text
CNH capture
    |
    v
Document validation + portrait extraction
    |
    v
Guided live face capture
    |
    +--> passive PAD
    +--> temporal signals
    +--> randomized illumination challenge
    |
    v
Live face <-> CNH portrait 1:1 match
    |
    v
approved / review / rejected + evidence
```

## Phase 1 - Web Capture SDK 2.0

Status: foundation implemented.

- Local brightness, contrast and sharpness preflight.
- Guided capture status.
- Server-provided illumination settle timing.
- Capture metadata and client quality telemetry.
- Server-side challenge normalization.
- Atomic single-use sessions.
- Passive PAD fail-closed by default.
- Architecture ready for a WASM capture runtime.

Next additions:

- Browser face tracking.
- Landmark stability.
- Head pose.
- Face size and centering guidance.
- Occlusion and eye-visibility checks.
- Web Worker/WASM execution boundary.

## Phase 2 - CNH capture

- Front/back capture UX.
- Document edge detection.
- Perspective correction.
- Glare, blur and crop quality checks.
- OCR and field normalization.
- Portrait extraction.
- Evidence package for document processing.

## Phase 3 - CNH portrait vs live face

- Separate document-portrait embedding pipeline.
- Model/version metadata on every template.
- 1:1 face comparison.
- Configurable review band.
- Mismatch diagnostics without exposing sensitive internals to the end user.

## Phase 4 - Advanced liveness

- Multiple PAD models or calibrated ensemble.
- Optical-flow consistency.
- Landmark temporal analysis.
- Regional illumination response.
- Replay and repeated-frame detection.
- Injection-oriented browser telemetry.

## Phase 5 - Evaluation lab

Build a governed dataset and repeatable harness for:

- Genuine captures across devices and lighting conditions.
- Printed photos.
- Photos displayed on phones and monitors.
- Replayed videos.
- Moved photos and moved screens.
- Partial occlusion.
- Masks where legally and operationally appropriate to test.
- Deepfake and virtual-camera injection attempts.
- Different ages, skin tones, camera qualities and environmental conditions representative of deployment.

Measure at minimum FMR/FNMR for identity matching and APCER/BPCER for PAD. Thresholds and liveness weights must be calibrated from measured results rather than treated as production constants.

## Phase 6 - Production controls

- Application/tenant authentication.
- Per-tenant, per-subject and network abuse controls.
- Audit events and trace IDs.
- Key rotation and KMS/HSM storage.
- Retention policies and deletion workflows.
- Observability and evaluation drift monitoring.
- Native SDK/device attestation option for higher-risk customers.

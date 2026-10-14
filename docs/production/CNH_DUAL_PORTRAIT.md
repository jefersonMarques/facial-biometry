# CNH Dual Portrait Consistency — Shadow Mode

Status: **⏳ implemented in code; automated and real-device validation pending**.

## Purpose

Compare the portrait extracted from the cryptographically intact signed CNH PDF with
the portrait embedded in the validated VIO QR payload. This is an additional
**document consistency observation**, not independent identity proof and not a
replacement for a valid PDF/VIO signature, CPF matching, liveness, PAD, or live
face matching.

The VIO portrait and PDF portrait can originate from the same administrative
source. Their agreement must not be described as two independent identity checks.

## Runtime

- Environment flag: `FACEPROOF_CNH_DUAL_PORTRAIT_SHADOW=true`.
- Default: `false`; no secondary VIO decode or model inference when disabled.
- Scope: CNH document upload only, after the existing PDF and VIO signature
  verification and normal reference-photo selection.
- If the selected reference is already the signed VIO portrait, the secondary
  analysis is `not_applicable` and is not performed.
- When the signed PDF portrait is the selected reference, the service attempts to
  decode the VIO portrait. A missing photo or BPG decoder does not reject the CNH.
- The Go handler runs an additional `ExtractReference` using the existing
  Secure Core C++ authority, then computes cosine similarity between SFace
  embeddings using the same model.
- No new score threshold is applied. The existing PDF reference and
  document-to-live biometrics remain unchanged.
- The shadow result is recorded with the encrypted document check and emitted
  as a privacy-safe analytics event when analytics is configured.
- In production/non-demo, VIO image bytes remain transient and are not persisted.
  In the internal authenticated demo console only, when shadow mode is enabled,
  the exact VIO portrait is stored as a bounded MIME-checked base64 image in
  `DemoArtifacts` within the existing AES-GCM encrypted identity check, like
  the PDF and captured demo portraits. The demo-only image is returned solely
  by the authenticated admin check detail endpoint for inspection/download.
  Never return it in issuer/public APIs, analytics events, check lists or reports.
  The demo image follows the current encrypted demo-artifact retention policy,
  which requires a separate production-grade retention/cleanup review.
  Secondary embeddings remain temporary and are cleared at completion,
  processed expiry or failed-document reset.

## Shadow statuses

| Status | Meaning |
|---|---|
| `compared` | Both references were processed and a valid cosine score was recorded |
| `not_applicable` | The selected primary reference already came from VIO |
| `portrait_unavailable` | No usable embedded VIO portrait was available |
| `decoder_unavailable` | The required image decoder was not available |
| `analysis_failed` | Decoding, model compatibility, extraction or cosine comparison failed |

An incompatible image format is currently reported as `analysis_failed`; the
raw VIO image format must not be assumed to be PNG/JPEG. A failed shadow analysis
must never convert an otherwise valid CNH or biometric result into a rejection.

## Event schema

Event name: `cnh_dual_portrait_shadow`.

Fields: `pdf_photo_source`, `vio_photo_present`, `vio_photo_validated`,
`dual_portrait_score` (nullable, raw cosine similarity),
`quality_a`, `quality_b` (nullable), `consistency_status`,
`decode_duration_ms`, `analysis_duration_ms`, `embedding_model`,
and `error_code` (fixed machine-readable code).

The shadow record never stores the VIO image, its embedding, CPF or name.
The raw cosine similarity is **not** the probability that an identity is genuine.

## Three-pair facial observation

With `FACEPROOF_CNH_DUAL_PORTRAIT_SHADOW=true`, the system collects these
scores, without using any of them as additional approval or rejection gates:

| Pair | Combined cosine | Robust multi-variant cosine |
|---|---|---|
| Signed PDF portrait vs signed VIO portrait | yes | yes |
| Signed VIO portrait vs live near-phase frames | yes | yes |
| Signed PDF portrait vs live near-phase frames | yes | yes (matches the existing authoritative live verification method) |

A new `cnh_tri_portrait_shadow` event is emitted **after successful completion
persistence**. The event records the three pairs as `pdf_vio`, `vio_live`,
`pdf_live`, with each pair's raw `combinedCosine` and `robustCosine`,
status, variant counts, number of compared frames and any technical error code.

The VIO photo often has substantially lower resolution and compression quality
than the PDF. Do not reuse the document-vs-live threshold `0.363` for either
VIO pair. Record VIO and PDF quality, dimensions, face size, sharpness and
brightness as **diagnostics, not pass/fail gates**. A lower VIO cosine does
not, by itself, prove tampering.

To enable the VIO-to-live comparison, the secondary SFace embedding is stored
temporarily within the existing **AES-GCM encrypted identity check** while
awaiting completion. The same temporary reference is removed atomically at
normal completion, document-attempt reset or recorded expiry; the VIO image
bytes are never persisted as an additional reference. Incomplete checks not
yet subjected to expiry processing retain their encrypted reference in the
existing storage until the application's retention/cleanup process handles
them. A scheduled expiry/retention sweeper is a production hardening task.
No raw secondary embedding is sent in the analytics event, demo or issuer API.

The final decision still uses only the existing document validation,
Secure Core liveness/PAD and **PDF-to-live robust similarity**. Scores are not
calibrated probabilities.

## Validation before activation

1. Run `go test ./...` in `services/api` and inspect the GitHub Actions CI.
2. Check both VIO image decoding and cryptographic signed-data coverage with
   legitimate documents of different versions and issuers.
3. Test genuine PDF × VIO pairs with different resolutions and compression,
   as well as controlled mismatched portraits.
4. Cover absent images, unknown formats, BPG unavailable, invalid embeddings,
   failed native extraction and invalid or tampered documents.
5. Confirm that activation does not change issued sessions, document rejection,
   matching thresholds, PAD or final decisions.
6. Measure decode + model inference latency and memory use on the target Linux
   runtime; assess impact before enabling for real clients.
7. Only after sufficient labeled samples, design separate consistency thresholds
   and a reviewed rollout plan. Shadow results alone must not drive a hard gate.

Do not mark this component ✅ until integration, runtime and representative
document tests pass. No external production deployment is authorized by this
document.

## Optional BPG decoder on Linux

If the signed VIO QR payload contains a BPG portrait, the shadow analysis
requires the external `bpgdec` executable. Without it the event will report
`decoder_unavailable` and the original CNH-to-live comparison will continue.

For controlled WSL/Linux development, source the decoder from the original
upstream project (<https://bellard.org/bpg/>, version 0.9.8). Build `bpgdec`
from reviewed, pinned source rather than downloading an arbitrary binary.
The upstream BPG distribution is old and must be treated as a native parser
of untrusted document bytes.

After placing a reviewed executable at a controlled absolute path, verify it
is executable and configure the FaceProof API process:

```bash
export FACEPROOF_CNH_DUAL_PORTRAIT_SHADOW=true
export FACEPROOF_BPGDEC_PATH="/absolute/path/to/bpgdec"
./scripts/run-dev.sh --analytics
```

Before enabling for production: review the decoder's origin, build artifacts
and known vulnerabilities; isolate the process with limited privileges and
resource limits. The current application limits the decoder runtime to 5 s
and the decoded PNG to 4 MiB; these limits are not a substitute for OS sandboxing.

## Related analytics diagnosis (October 2026)

A real WSL test confirmed `decoder_unavailable` while CNH validation and live
biometrics succeeded. During the same test, PostgreSQL analytics reported
parameter-inference failures in authoritative reconciliation and completion.
The analytics SQL now explicitly casts floating-point parameters, and CI runs
a PostgreSQL-backed regression test. Repeat the WSL run after pulling these
changes to confirm the deployed external database is healthy.

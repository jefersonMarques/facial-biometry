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
- The VIO image bytes and secondary embeddings are transient and must never
  be placed in the analytics event or the persisted check.

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

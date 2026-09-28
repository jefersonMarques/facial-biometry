# FaceProof

Browser-first, self-hosted identity verification and facial biometric MVP.

The primary flow now accepts an original Brazilian CNH Digital PDF, authenticates the signed PDF and VIO payload, extracts a biometric reference portrait from the cryptographically intact signed PDF, with the signed VIO portrait as fallback, performs browser-based live capture with liveness, and compares the live face against that portrait.

## CNH Digital Identity Check

```text
Issuer creates check
    ↓
expected CPF + minimum signed-PDF date
    ↓
opaque public link
    ↓
original CNH Digital PDF
    ↓
PDF signature + metadata consistency
    ↓
VIO signature + CPF + official portrait
    ↓
browser liveness capture
    ↓
CNH portrait ↔ live face
    ↓
approved / review / rejected
```

The minimum date is enforced against the PDF signing timestamp, not editable PDF metadata. The VIO payload is independently verified and is the source for CPF and the biometric reference portrait.

See `docs/IDENTITY_CHECK.md` for the protocol and endpoints.

## Included

- issuer-authenticated identity-check creation and result lookup;
- opaque public links with token in the URL fragment;
- PDF signature/full-document integrity inspection;
- CNH Digital metadata consistency checks;
- VIO QR decoding v1-v6 and signature verification;
- expected CPF matching;
- signed-PDF portrait extraction for biometrics, with VIO/BPG portrait fallback;
- one-time reference embedding without permanent CNH enrollment;
- guided browser capture with brightness/contrast/sharpness preflight;
- randomized server-generated illumination challenge;
- single-use biometric sessions and server-side challenge reconstruction;
- YuNet detection, SFace embeddings and MiniFASNet V2 passive PAD;
- passive PAD fail-closed mode by default;
- encrypted identity-check/template storage;
- legacy biometric enrollment/verification demo.

## Limitations

This remains a research/MVP baseline. Browser capture cannot provide hardware-backed camera provenance, and the default biometric thresholds are not production-calibrated.

The strict CNH PDF profile must be regression-tested against a representative collection of legitimate CNH Digital PDFs from multiple DETRAN issuers before it is treated as a nationwide production policy.

The MVP also does not provide complete long-term certificate revocation/timestamp validation, distributed abuse controls or horizontally scaled identity/session storage.

## Requirements

- Go 1.23+
- Python 3.11+
- a modern browser with camera support;
- Poppler tools: `pdfsig`, `pdfinfo` and `pdftoppm`;
- `bpgdec` is optional fallback support for VIO portraits encoded in BPG.

Node.js is only required when editing the TypeScript SDK. The compiled JavaScript bundle is committed.

## Run

Linux/macOS:

```bash
./scripts/run-dev.sh
```

Windows:

```powershell
.\scripts\run-dev.ps1
```

Services:

```text
Web:              http://localhost:5173
Identity page:    http://localhost:5173/verify.html
Biometric API:    http://localhost:8080
Biometric engine: http://localhost:8090
```

To create a check, call:

```http
POST /v1/identity/checks
Authorization: Bearer <FACEPROOF_IDENTITY_ISSUER_KEY>
Content-Type: application/json

{
  "cpf": "<expected-cpf>",
  "minimumDocumentDate": "2026-09-25",
  "expiresInMinutes": 60
}
```

The response contains the `checkId` and public verification URL. Query the result later with:

```http
GET /v1/identity/checks/<checkId>
Authorization: Bearer <FACEPROOF_IDENTITY_ISSUER_KEY>
```

## Configuration

Start from `.env.example`. Important identity settings:

- `FACEPROOF_IDENTITY_ISSUER_KEY`
- `FACEPROOF_IDENTITY_VERIFY_URL`
- `FACEPROOF_IDENTITY_LINK_TTL_MINUTES`
- `FACEPROOF_IDENTITY_MAX_PDF_MB`
- `FACEPROOF_PDFSIG_PATH`
- `FACEPROOF_PDFINFO_PATH`
- `FACEPROOF_BPGDEC_PATH`

Important biometric settings:

- `FACEPROOF_SESSION_SECRET`
- `FACEPROOF_TEMPLATE_KEY`
- `FACEPROOF_MATCH_THRESHOLD`
- `FACEPROOF_LIVENESS_THRESHOLD`
- `FACEPROOF_REQUIRE_PASSIVE_PAD`
- `FACEPROOF_ENGINE_URL`

## Documentation

- `docs/IDENTITY_CHECK.md`
- `docs/ARCHITECTURE.md`
- `docs/SECURITY.md`
- `docs/BROWSER_FIRST_ROADMAP.md`
- `THIRD_PARTY_NOTICES.md`
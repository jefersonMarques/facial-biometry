# FaceProof

Browser-first facial biometric enrollment, liveness and verification platform.

The FaceProof core is biometric capture, liveness, enrollment and 1:1 re-verification with the server as the final authority. Document identity flows are optional product modules.

The current repository also includes a Brazilian CNH Digital Identity module that authenticates the signed PDF and VIO payload, extracts an official biometric reference portrait and compares it against a live capture.

See `docs/BROWSER_FIRST_ROADMAP.md` for the SDK, licensing, Control Plane and Go + HTMX Console direction.

## Optional CNH Digital Identity Check

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
Start biometrics
    ↓
smaller oval → automatic first capture
    ↓
larger oval → automatic second capture
    ↓
passive PAD + far/near evidence
    ↓
multi-frame live embedding
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
- continuous camera guidance with exactly two automatic capture stages;
- smaller far oval followed by a larger near oval;
- YuNet positioning feedback before and during each capture;
- short multi-frame burst behind each visible capture;
- low-quality recapture without consuming the biometric-attempt budget;
- passive PAD plus far/near temporal and scale evidence;
- multi-frame SFace live embedding;
- randomized illumination challenge remains available for the legacy enrollment/verification demo, not the Identity Check flow;
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

- Linux x86_64;
- Go 1.23+;
- C++17 toolchain, CMake and Ninja for building the Secure Core;
- Python is optional and reserved for validation/R&D tooling;
- a modern browser with camera support;
- Poppler tools: `pdfinfo` and `pdftoppm`; PDF signature cryptography is also verified natively in Go;
- `bpgdec` is optional fallback support for VIO portraits encoded in BPG.

Node.js is only required when editing the TypeScript SDK. The compiled JavaScript bundle is committed.

## Run

Build the Linux Secure Core once:

```bash
bash scripts/build-biometric-core.sh
```

Start the production-candidate runtime:

```bash
./scripts/run-dev.sh --analytics
```

The native Secure Core is the default runtime. It does not start the Python biometric engine.

Python is available only as an explicit development/R&D fallback:

```bash
./scripts/run-dev.sh --python-engine --analytics
```

Services in native mode:

```text
Web:              http://localhost:5173
Identity page:    http://localhost:5173/verify.html
Biometric API:    http://localhost:8180
Analytics panel:  http://localhost:5174
Secure Core:      libfaceproof_core.so (in-process)
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
- `FACEPROOF_SECURE_CORE_LIBRARY`
- `FACEPROOF_YUNET_MODEL`
- `FACEPROOF_SFACE_MODEL`
- `FACEPROOF_MINIFASNET_MODEL`
- `FACEPROOF_ENGINE_URL` (Python R&D fallback only)

## Documentation

- `docs/IDENTITY_CHECK.md`
- `docs/ARCHITECTURE.md`
- `docs/SECURITY.md`
- `docs/BROWSER_FIRST_ROADMAP.md`
- `THIRD_PARTY_NOTICES.md`

## Cloudflare quick tunnel

The development server on port `5173` is a same-origin gateway:

- static web files are served directly;
- `/v1/*` is reverse-proxied to the FaceProof API on `127.0.0.1:8180`.

This means one tunnel is enough for browser + API:

```powershell
cloudflared tunnel --url http://localhost:5173
```

After Cloudflare prints the public `https://....trycloudflare.com` URL, create a check with the Linux helper:

```bash
./scripts/create-identity-check.sh \
  "CPF_DA_CNH" \
  "2026-01-01" \
  60 \
  "" \
  genuine_live \
  approved \
  "https://YOUR-SUBDOMAIN.trycloudflare.com"
```

Do not tunnel only a plain static file server. The public origin must reach the FaceProof gateway so that `/v1/*` returns the API JSON responses.


### Robust face matching

Identity Check now treats the two biometric phases differently:

- the far capture contributes mainly to PAD, temporal motion and far-to-near scale evidence;
- the near capture is recorded at higher resolution and is the primary source for identity matching;
- the three highest-quality near frames are embedded independently with SFace;
- the signed CNH portrait produces a small reference ensemble using mild luminance normalization;
- each live frame is compared against the reference ensemble and the final similarity is the mean of those quality-selected frame scores.

The UI shows the raw SFace score and configured threshold rather than presenting cosine similarity as a probability.

For the MVP result screen, the browser also shows:
- basic signed CNH data: name, masked CPF, birth date, category, validity and issuing state when available;
- the portrait extracted from the authenticated CNH;
- the best-quality near capture selected by the biometric engine.

The raw CNH portrait used for this comparison is returned only in the immediate document-validation response and is kept in browser memory for the current flow; it is not added to the persistent identity-check record.

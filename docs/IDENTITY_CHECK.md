# CNH Digital identity check

The identity-check flow performs a one-time identity proof using the original CNH Digital PDF as the reference document.

## Flow

```text
Issuer
  |
  +--> expected CPF
  +--> minimum signed-PDF date
  |
  v
opaque verification link
  |
  v
CNH Digital PDF upload
  |
  +--> PDF signature / full-document integrity
  +--> official signer checks
  +--> metadata consistency
  +--> QR VIO decode and signature
  +--> CNH template / SENATRAN
  +--> expected CPF
  +--> signed-PDF freshness
  +--> official portrait extraction
  |
  v
reference SFace embedding
  |
  | raw PDF and portrait discarded
  v
user clicks Start biometrics
  |
  v
camera starts and stays on
  |
  +--> phase 1: guide face into the smaller oval
  +--> automatic first capture (short multi-frame burst)
  +--> oval expands
  +--> phase 2: guide face into the larger oval
  +--> automatic second capture (short multi-frame burst)
  |
  v
passive PAD + far/near temporal/scale evidence
  |
  v
multi-frame SFace embedding <-> CNH embedding
  |
  v
approved / review / rejected
```

## Create a check

`FACEPROOF_IDENTITY_ISSUER_KEY` protects the issuer endpoint.

```http
POST /v1/identity/checks
Authorization: Bearer <issuer-key>
Content-Type: application/json

{
  "cpf": "<expected-cpf>",
  "minimumDocumentDate": "2026-09-25",
  "expiresInMinutes": 60
}
```

The response contains a non-secret `checkId` and a public verification URL. The public token is placed in the URL fragment (`#identity=...`), so it is not sent as part of the HTTP request for the page. The verification page moves it into `sessionStorage`, removes the fragment from the visible URL, and sends it to the API only in `X-FaceProof-Identity-Token`.

## Issuer result

The issuer can query the final state without reusing the public link token:

```http
GET /v1/identity/checks/chk_...
Authorization: Bearer <issuer-key>
```

The response contains status, document evidence and final biometric scores when available. It does not contain the public token or reference embedding.

## Public verification endpoints

```text
GET  /v1/identity/check
POST /v1/identity/document
POST /v1/identity/guide
POST /v1/identity/session
POST /v1/identity/complete
```

All require `X-FaceProof-Identity-Token`.

The `/v1/identity/guide` endpoint is a low-resolution, non-persistent positioning preflight. It uses YuNet to return normalized face position, scale, roll and face-quality signals. It does not create or consume a biometric attempt.

The browser keeps the same camera stream open for the entire biometric flow. After the user clicks **Start biometrics**, YuNet guides the face into a smaller oval. Once framing is stable, FaceProof makes the first automatic capture as a short multi-frame burst. The oval then expands, the user moves closer, and a second guided multi-frame capture is made automatically. There is no third visible capture/challenge step.

The two capture bursts are analyzed together for passive PAD, temporal motion, far-to-near scale change, quality, face presence and a multi-frame SFace embedding. Poor-quality completion returns a recapture-required response while keeping the identity check in `biometry_pending`; this does not consume one of the final biometric attempts.

## PDF authenticity policy

The current strict profile requires:

- a cryptographically valid detached PDF signature;
- native Go PDF signature validation (required); `pdfsig` is optional corroboration to report the total document as signed;
- a DETRAN or SENATRAN signer identity;
- ICP-Brasil evidence in the signer certificate subject/issuer;
- SHA-256, SHA-384 or SHA-512;
- an accepted detached PKCS#7/CAdES signature type;
- no signing time in the future;
- title `CNH Digital`;
- creator and producer `CDT`;
- one page;
- no JavaScript;
- no encryption;
- no `pdfinfo` suspect flag;
- an AcroForm signature form;
- metadata file size matching the uploaded bytes;
- creation/modification metadata consistent with the signed timestamp.

The minimum document date is compared against the PDF signing timestamp, not against editable `CreationDate` or `ModDate` metadata.

A certificate that is expired at verification time is not by itself treated as proof of PDF tampering when the PDF signature remains cryptographically valid and the official signer profile matches. This MVP does not claim complete historical OCSP/CRL or qualified timestamp validation.

## VIO authenticity policy

After PDF integrity passes:

- render the PDF with `pdftoppm`;
- locate the QR Code;
- decode VIO v1-v6;
- require a recognized CNH template issued by SENATRAN;
- verify the VIO signature using the VIO certificate/public-key catalog;
- require the VIO CPF to equal the expected CPF;
- prefer the portrait extracted from the cryptographically intact signed PDF;
- retain the signed VIO portrait as a fallback;
- decode BPG to PNG only when that VIO fallback is needed.

For identity checks, the visual portrait may be used as the biometric reference only after the PDF signature is cryptographically valid and the signed revision covers the complete file. The signed VIO payload remains authoritative for CPF, document type and its own authenticity.

## Privacy and storage

The application does not persist the uploaded PDF or portrait.

The identity-check state is AES-GCM encrypted and contains the expected CPF, minimum date, document evidence and a reference embedding only while the check is waiting for biometrics. After the final decision, the reference embedding is removed.

The public token itself is not persisted; its SHA-256 hash is used as the storage locator and AEAD associated data.

## Runtime dependencies

CNH Digital validation requires:

- native Go PDF signature validation (required); `pdfsig` is optional corroboration for PDF signature inspection;
- `pdfinfo` for PDF structure and metadata;
- `pdftoppm` for page rendering and QR scanning;
- `bpgdec` is optional fallback support for VIO portraits encoded as BPG.

The identity repository is encrypted local-file storage for the MVP. Move it to a transactional shared store before multi-instance production.
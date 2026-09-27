# FaceProof

Browser-first, self-hosted facial biometric MVP with guided multi-frame capture, passive liveness signals, encrypted biometric templates and 1:1 face verification.

The target product is an identity-verification flow that captures a Brazilian CNH, extracts its portrait, performs browser-based live face capture and compares the live identity against the document portrait with liveness evidence.

## What is included

- Web capture SDK written in TypeScript.
- Browser preflight for brightness, contrast and sharpness.
- Approximately 2.4 second guided multi-frame capture.
- Server-generated randomized illumination challenge with enforced high/low transitions.
- Server-side frame timing validation and challenge reconstruction.
- Atomic single-use biometric sessions to block concurrent replay.
- Go API with signed session tokens and encrypted biometric template storage.
- Python/OpenCV biometric engine.
- YuNet face detection (downloaded from the original source).
- SFace face embedding and 1:1 cosine matching (downloaded from the original source).
- MiniFASNet V2 passive anti-spoofing (downloaded from the original source).
- Temporal motion and regional illumination-response signals.
- Passive PAD fail-closed mode enabled by default.
- Enrollment and verification demo flows.
- No paid API and no cloud dependency.

## Important limitation

This is a research/MVP baseline, not a production-certified biometric product. The passive liveness decision combines an open model with experimental temporal signals. It has not been independently tested against ISO/IEC 30107-3 attack instruments, deepfake injection, virtual cameras, masks or a representative production population.

The browser is explicitly treated as an untrusted client. JavaScript, WASM, timestamps and browser camera inputs can be manipulated. The API remains authoritative for session state, challenge definition, challenge normalization and the final decision, but browser-only capture cannot provide hardware-backed camera provenance.

Do not use the default thresholds as final security thresholds. Build an evaluation dataset, measure APCER/BPCER for liveness and FMR/FNMR for identity matching, then calibrate by use case.

## Requirements

- Go 1.23+
- Python 3.11+
- Python packages in `services/engine/requirements.txt`
- A modern browser with camera support

Node.js is only required when changing the TypeScript SDK. A precompiled JavaScript bundle is checked in.

## Quick start

### Linux/macOS

```bash
./scripts/run-dev.sh
```

### Windows PowerShell

```powershell
./scripts/run-dev.ps1
```

Then open:

```text
http://localhost:5173
```

The startup script downloads the three open model artifacts with pinned SHA-256 checksums before starting the services.

The services use:

```text
Web demo:          http://localhost:5173
Biometric API:     http://localhost:8080
Biometric engine:  http://localhost:8090
```

## Manual start

Engine:

```bash
cd services/engine
python -m pip install -r requirements.txt
python engine_server.py
```

API:

```bash
cd services/api
go run ./cmd/server
```

Web:

```bash
cd apps/web-sdk
python -m http.server 5173
```

## Demo flow

1. Enter a `subjectId`.
2. Choose **Enroll**.
3. The browser checks basic illumination and image sharpness before capture.
4. Keep the face centered while the application performs the short randomized-light capture.
5. The API validates capture timing, reconstructs the expected challenge and consumes the session once.
6. The engine extracts a biometric template only after liveness and quality checks.
7. Choose **Verify** with the same `subjectId`.
8. A new live capture is compared with the encrypted stored template.

## Configuration

Copy `.env.example` values into your environment as needed.

The most important values are:

- `FACEPROOF_SESSION_SECRET`: HMAC signing key.
- `FACEPROOF_TEMPLATE_KEY`: Base64-encoded 32-byte AES key.
- `FACEPROOF_MATCH_THRESHOLD`: Initial SFace cosine threshold.
- `FACEPROOF_LIVENESS_THRESHOLD`: Overall liveness threshold.
- `FACEPROOF_REQUIRE_PASSIVE_PAD`: Reject approval when passive PAD is unavailable. Defaults to `true`.
- `FACEPROOF_ENGINE_URL`: Python engine endpoint.

Generate a template encryption key:

```bash
python -c "import os,base64; print(base64.b64encode(os.urandom(32)).decode())"
```

## Browser-first roadmap

The next product layer is CNH document capture, document portrait extraction and live-face-to-document matching. See `docs/BROWSER_FIRST_ROADMAP.md`.

## Architecture and security

See `docs/ARCHITECTURE.md` and `docs/SECURITY.md`.

## Third-party models

See `THIRD_PARTY_NOTICES.md`. Model binaries are not redistributed inside this repository; `models/download_models.py` retrieves them from their original sources and verifies their SHA-256 checksums.

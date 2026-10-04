# Third-party notices

This repository references third-party model artifacts. Model binaries are downloaded from the original sources by `models/download_models.py` and are not relicensed by FaceProof.

## YuNet

File: `models/yunet/face_detection_yunet_2023mar.onnx`

Source: OpenCV Zoo / YuNet.

License: MIT.

The downloader also retrieves the original license text to `models/yunet/LICENSE`.

## SFace

File: `models/sface/face_recognition_sface_2021dec.onnx`

Source: OpenCV Zoo / SFace.

License: Apache License 2.0.

The downloader also retrieves the original license text to `models/sface/LICENSE`.

## MiniFASNet V2

File: `models/minifasnet/MiniFASNetV2.onnx`

Source: yakhyo/face-anti-spoofing, based on Silent-Face-Anti-Spoofing.

License declared by the source repository: Apache License 2.0.

The downloader also retrieves the source repository license text to `models/minifasnet/LICENSE`.

## OpenCV

The runtime code uses OpenCV APIs. Install OpenCV according to the package's own license and distribution terms.


## gozxing

Go dependency: `github.com/makiuchi-d/gozxing`.

Used to read QR Codes in the CNH Digital VIO pipeline.

License: MIT. The upstream project also contains portions derived from ZXing under Apache License 2.0; consult the upstream license notices for the exact terms.

## Poppler command-line utilities

The identity flow invokes `pdfsig`, `pdfinfo` and `pdftoppm` as external processes. Poppler is not vendored in this repository and must be installed/distributed according to its own license and platform packaging terms.

## BPG decoder

The VIO portrait converter can invoke an external `bpgdec` executable. It is not vendored in this repository.


## fullsailor/pkcs7

Go dependency: `github.com/fullsailor/pkcs7`.

Used by the Windows/native PDF signature fallback to parse BER/DER PKCS#7/CMS signatures found in official CNH Digital PDFs.

License: MIT.


## MediaPipe Tasks Vision / Face Landmarker

Browser dependency: `@mediapipe/tasks-vision` version `1.0.1`.

Runtime use: local facial landmarks for browser-side capture guidance and experimental geometry.

License: Apache License 2.0.

The JavaScript/WASM runtime is installed from the pinned NPM package and copied into the generated same-origin runtime directory only after exact size and SHA-256 verification. The Face Landmarker `float16/1` model is downloaded from the versioned Google MediaPipe model URL and verified against the pinned SHA-256 in `tools/mediapipe-runtime-lock.json`.

The generated MediaPipe runtime directory is not committed to this repository. Runtime network access is restricted by the FaceProof Content Security Policy; MediaPipe input frames remain local browser inputs and the SDK's external metrics endpoint is not allowed by the FaceProof gateway CSP.

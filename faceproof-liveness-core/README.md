# FaceProof Liveness Core

Native C++ core for browser-side liveness geometry. The core is designed to compile both natively and to WebAssembly with Emscripten.

## Current scope

Version `0.1.0` intentionally implements only the geometry already exercised by the experimental TypeScript Liveness V2:

- far/near face scale ratio;
- scale transition score;
- normalized facial perspective change;
- MediaPipe relative-depth change;
- per-phase stability;
- experimental geometry evidence score.

It does **not** decide whether an identity is approved. Browser and WASM output remain untrusted evidence; the FaceProof server remains authoritative.

## Architecture

```text
MediaPipe Face Landmarker
        |
        | 478 landmarks
        v
FaceProof Liveness Core
        C++17
        |
        +-- native build for deterministic tests
        |
        +-- Emscripten
              |
              v
        WebAssembly + ES module
              |
              v
        Web Worker / Web SDK
```

## Native build

Linux/macOS:

```bash
cmake -S faceproof-liveness-core -B build/liveness-core
cmake --build build/liveness-core --config Release
ctest --test-dir build/liveness-core --output-on-failure
```

Windows with Visual Studio Build Tools:

```powershell
cmake -S faceproof-liveness-core -B build/liveness-core
cmake --build build/liveness-core --config Release
ctest --test-dir build/liveness-core -C Release --output-on-failure
```

## WebAssembly build

Activate an Emscripten SDK environment first, then run:

```bash
emcmake cmake -S faceproof-liveness-core -B build/liveness-wasm \
  -DFACEPROOF_BUILD_TESTS=OFF \
  -DCMAKE_BUILD_TYPE=Release

cmake --build build/liveness-wasm
```

Expected artifacts:

```text
build/liveness-wasm/
├── faceproof-liveness-core.js
└── faceproof-liveness-core.wasm
```

The WASM target uses:

- C++17;
- `-O3`;
- WebAssembly SIMD (`-msimd128`);
- no pthreads in this first version;
- ES module output;
- browser/worker runtime;
- growable memory.

## Public C ABI

The stable core interface is declared in:

```text
include/faceproof_liveness.h
```

Main lifecycle:

```text
fp_create
fp_reset
fp_begin_phase
fp_push_landmarks
fp_get_result
fp_destroy
```

The Emscripten adapter accepts landmarks as a flat `Float64Array` in XYZ order and exposes the result through a fixed C struct.

## Security model

The WASM is not a root of trust.

An attacker controlling the browser can modify JavaScript, WASM, memory, timestamps or camera input. The local core exists for performance, richer evidence, frame selection and attack-cost increase.

Production decisions must continue to be performed by the server using server-controlled session state, biometric models and independently validated evidence.

## Next steps

1. Build and publish the WASM artifact from a pinned Emscripten toolchain.
2. Add a TypeScript Web Worker wrapper.
3. Run TypeScript and C++ implementations side by side in shadow mode.
4. Compare outputs on genuine/photo/video captures.
5. Replace TypeScript geometry only after parity is demonstrated.
6. Add pose, parallax, optical flow and capture-integrity signals incrementally.

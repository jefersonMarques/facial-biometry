#include "faceproof_liveness.h"

#include <emscripten/emscripten.h>

#include <cstdint>
#include <new>
#include <vector>

namespace {

struct FPWasmContext {
    FPContext* core = nullptr;
    std::vector<FPLandmark> landmarks;
    FPLivenessResult result{};
};

FPWasmContext* from_handle(std::uintptr_t handle) {
    return reinterpret_cast<FPWasmContext*>(handle);
}

}  // namespace

extern "C" {

EMSCRIPTEN_KEEPALIVE
std::uintptr_t fp_wasm_create(void) {
    auto* wrapper = new (std::nothrow) FPWasmContext();
    if (!wrapper) {
        return 0;
    }

    wrapper->core = fp_create();
    if (!wrapper->core) {
        delete wrapper;
        return 0;
    }

    return reinterpret_cast<std::uintptr_t>(wrapper);
}

EMSCRIPTEN_KEEPALIVE
void fp_wasm_destroy(std::uintptr_t handle) {
    auto* wrapper = from_handle(handle);
    if (!wrapper) {
        return;
    }

    fp_destroy(wrapper->core);
    delete wrapper;
}

EMSCRIPTEN_KEEPALIVE
int fp_wasm_reset(std::uintptr_t handle) {
    auto* wrapper = from_handle(handle);
    if (!wrapper || !wrapper->core) {
        return FP_ERR_INVALID_ARGUMENT;
    }

    fp_reset(wrapper->core);
    wrapper->landmarks.clear();
    wrapper->result = {};
    return FP_OK;
}

EMSCRIPTEN_KEEPALIVE
int fp_wasm_push_landmarks_xyz(
    std::uintptr_t handle,
    int phase,
    const double* xyz,
    std::uint32_t landmark_count,
    double timestamp_ms
) {
    auto* wrapper = from_handle(handle);
    if (!wrapper || !wrapper->core || !xyz || landmark_count == 0) {
        return FP_ERR_INVALID_ARGUMENT;
    }

    if (phase != FP_PHASE_FAR && phase != FP_PHASE_NEAR) {
        return FP_ERR_INVALID_ARGUMENT;
    }

    wrapper->landmarks.resize(landmark_count);
    for (std::uint32_t index = 0; index < landmark_count; ++index) {
        const auto offset = static_cast<std::size_t>(index) * 3;
        wrapper->landmarks[index] = {
            xyz[offset],
            xyz[offset + 1],
            xyz[offset + 2],
        };
    }

    const int phase_result = fp_begin_phase(wrapper->core, static_cast<FPPhase>(phase));
    if (phase_result != FP_OK) {
        return phase_result;
    }

    return fp_push_landmarks(
        wrapper->core,
        wrapper->landmarks.data(),
        landmark_count,
        timestamp_ms
    );
}

EMSCRIPTEN_KEEPALIVE
std::uintptr_t fp_wasm_get_result(std::uintptr_t handle) {
    auto* wrapper = from_handle(handle);
    if (!wrapper || !wrapper->core) {
        return 0;
    }

    if (fp_get_result(wrapper->core, &wrapper->result) != FP_OK) {
        return 0;
    }

    return reinterpret_cast<std::uintptr_t>(&wrapper->result);
}

EMSCRIPTEN_KEEPALIVE
std::uint32_t fp_wasm_result_size(void) {
    return static_cast<std::uint32_t>(sizeof(FPLivenessResult));
}

}  // extern "C"

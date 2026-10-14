#include "faceproof_liveness.h"

#include <emscripten/emscripten.h>

#include <cstdint>
#include <new>
#include <vector>

namespace {

constexpr std::uint32_t kResultValueCount = 10;
constexpr std::uint32_t kGuideValueCount = 7;

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
int fp_wasm_write_result(
    std::uintptr_t handle,
    double* output,
    std::uint32_t output_count
) {
    auto* wrapper = from_handle(handle);
    if (!wrapper || !wrapper->core || !output || output_count < kResultValueCount) {
        return FP_ERR_INVALID_ARGUMENT;
    }

    if (fp_get_result(wrapper->core, &wrapper->result) != FP_OK) {
        return FP_ERR_INVALID_ARGUMENT;
    }

    output[0] = static_cast<double>(wrapper->result.status);
    output[1] = static_cast<double>(wrapper->result.sample_count);
    output[2] = static_cast<double>(wrapper->result.far_samples);
    output[3] = static_cast<double>(wrapper->result.near_samples);
    output[4] = wrapper->result.scale_ratio;
    output[5] = wrapper->result.transition_score;
    output[6] = wrapper->result.perspective_change;
    output[7] = wrapper->result.depth_change;
    output[8] = wrapper->result.phase_stability;
    output[9] = wrapper->result.evidence_score;

    return FP_OK;
}

EMSCRIPTEN_KEEPALIVE
std::uint32_t fp_wasm_result_value_count(void) {
    return kResultValueCount;
}

EMSCRIPTEN_KEEPALIVE
int fp_wasm_write_guide_xyz(
    const double* xyz,
    std::uint32_t landmark_count,
    double* output,
    std::uint32_t output_count
) {
    if (!xyz || landmark_count == 0 || !output || output_count < kGuideValueCount) {
        return FP_ERR_INVALID_ARGUMENT;
    }

    std::vector<FPLandmark> landmarks(landmark_count);
    for (std::uint32_t index = 0; index < landmark_count; ++index) {
        const auto offset = static_cast<std::size_t>(index) * 3;
        landmarks[index] = {
            xyz[offset],
            xyz[offset + 1],
            xyz[offset + 2],
        };
    }

    FPGuideResult result{};
    if (fp_analyze_guide(landmarks.data(), landmark_count, &result) != FP_OK) {
        output[0] = 0.0;
        for (std::uint32_t index = 1; index < kGuideValueCount; ++index) {
            output[index] = 0.0;
        }
        return FP_OK;
    }

    output[0] = static_cast<double>(result.face_detected);
    output[1] = result.center_x;
    output[2] = result.center_y;
    output[3] = result.width_ratio;
    output[4] = result.height_ratio;
    output[5] = result.roll_degrees;
    output[6] = result.face_size_score;
    return FP_OK;
}

EMSCRIPTEN_KEEPALIVE
std::uint32_t fp_wasm_guide_value_count(void) {
    return kGuideValueCount;
}

}  // extern "C"

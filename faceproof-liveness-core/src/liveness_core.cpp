#include "faceproof_liveness.h"

#include "internal.h"

#include <cmath>
#include <cstddef>
#include <new>

struct FPContext {
    int phase = -1;
    std::vector<faceproof::Observation> observations;
};

extern "C" {

FPContext* fp_create(void) {
    return new (std::nothrow) FPContext();
}

void fp_destroy(FPContext* context) {
    delete context;
}

void fp_reset(FPContext* context) {
    if (!context) {
        return;
    }

    context->phase = -1;
    context->observations.clear();
}

int fp_begin_phase(FPContext* context, FPPhase phase) {
    if (!context || (phase != FP_PHASE_FAR && phase != FP_PHASE_NEAR)) {
        return FP_ERR_INVALID_ARGUMENT;
    }

    context->phase = static_cast<int>(phase);
    return FP_OK;
}

int fp_push_landmarks(
    FPContext* context,
    const FPLandmark* landmarks,
    std::uint32_t landmark_count,
    double timestamp_ms
) {
    if (!context || !landmarks || landmark_count == 0 || !std::isfinite(timestamp_ms)) {
        return FP_ERR_INVALID_ARGUMENT;
    }

    if (context->phase != FP_PHASE_FAR && context->phase != FP_PHASE_NEAR) {
        return FP_ERR_PHASE_NOT_SET;
    }

    faceproof::Observation observation;
    if (!faceproof::extract_observation(
        landmarks,
        landmark_count,
        static_cast<FPPhase>(context->phase),
        timestamp_ms,
        observation
    )) {
        return FP_ERR_INVALID_ARGUMENT;
    }

    context->observations.push_back(observation);
    if (context->observations.size() > faceproof::kMaxObservations) {
        const auto excess = context->observations.size() - faceproof::kMaxObservations;
        context->observations.erase(
            context->observations.begin(),
            context->observations.begin() + static_cast<std::ptrdiff_t>(excess)
        );
    }

    return FP_OK;
}

int fp_get_result(const FPContext* context, FPLivenessResult* result) {
    if (!context || !result) {
        return FP_ERR_INVALID_ARGUMENT;
    }

    *result = faceproof::summarize(context->observations);
    return FP_OK;
}

int fp_analyze_guide(
    const FPLandmark* landmarks,
    std::uint32_t landmark_count,
    FPGuideResult* result
) {
    if (!landmarks || landmark_count == 0 || !result) {
        return FP_ERR_INVALID_ARGUMENT;
    }

    if (!faceproof::analyze_guide(landmarks, landmark_count, *result)) {
        *result = {};
        return FP_ERR_INVALID_ARGUMENT;
    }

    return FP_OK;
}

const char* fp_version(void) {
    return FP_LIVENESS_CORE_VERSION;
}

}  // extern "C"

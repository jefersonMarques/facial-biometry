#pragma once

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define FP_LIVENESS_CORE_VERSION "0.1.0"

typedef enum FPPhase {
    FP_PHASE_FAR = 0,
    FP_PHASE_NEAR = 1
} FPPhase;

typedef enum FPLivenessStatus {
    FP_LIVENESS_INSUFFICIENT = 0,
    FP_LIVENESS_EXPERIMENTAL = 1
} FPLivenessStatus;

typedef enum FPResultCode {
    FP_OK = 0,
    FP_ERR_INVALID_ARGUMENT = -1,
    FP_ERR_PHASE_NOT_SET = -2
} FPResultCode;

typedef struct FPLandmark {
    double x;
    double y;
    double z;
} FPLandmark;

typedef struct FPGuideResult {
    int32_t face_detected;
    double center_x;
    double center_y;
    double width_ratio;
    double height_ratio;
    double roll_degrees;
    double face_size_score;
} FPGuideResult;

typedef struct FPLivenessResult {
    int32_t status;
    uint32_t sample_count;
    uint32_t far_samples;
    uint32_t near_samples;
    double scale_ratio;
    double transition_score;
    double perspective_change;
    double depth_change;
    double phase_stability;
    double evidence_score;
} FPLivenessResult;

typedef struct FPContext FPContext;

FPContext* fp_create(void);
void fp_destroy(FPContext* context);

void fp_reset(FPContext* context);
int fp_begin_phase(FPContext* context, FPPhase phase);

int fp_push_landmarks(
    FPContext* context,
    const FPLandmark* landmarks,
    uint32_t landmark_count,
    double timestamp_ms
);

int fp_get_result(const FPContext* context, FPLivenessResult* result);

int fp_analyze_guide(
    const FPLandmark* landmarks,
    uint32_t landmark_count,
    FPGuideResult* result
);

const char* fp_version(void);

#ifdef __cplusplus
}
#endif

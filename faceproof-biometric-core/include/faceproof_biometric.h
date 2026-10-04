#pragma once

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

#define FP_BIOMETRIC_CORE_VERSION "0.1.0"

double fp_bio_robust_mean(const double* values, size_t count);

double fp_bio_guided_capture_score(
    const double* far_scales,
    size_t far_count,
    const double* near_scales,
    size_t near_count,
    const double* center_scores,
    size_t center_count,
    const double* quality_scores,
    size_t quality_count
);

double fp_bio_identity_liveness_score(
    double passive_score,
    int passive_available,
    double guided_score,
    double temporal_score,
    double quality_score,
    double face_presence
);

const char* fp_biometric_version(void);

#ifdef __cplusplus
}
#endif

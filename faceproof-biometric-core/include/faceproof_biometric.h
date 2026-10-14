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

typedef struct FPBiometricCenter {
    double x;
    double y;
    double scale;
} FPBiometricCenter;

double fp_bio_temporal_motion_score(
    const double* pixel_differences,
    size_t pixel_difference_count,
    const FPBiometricCenter* centers,
    size_t center_count
);

double fp_bio_illumination_correlation(
    const int* challenge_indices,
    const double* observed_brightness,
    size_t observation_count,
    const double* illumination_pattern,
    size_t pattern_count,
    double* correlation
);

const char* fp_biometric_version(void);

#ifdef __cplusplus
}
#endif

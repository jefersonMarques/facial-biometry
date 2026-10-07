#include "faceproof_biometric.h"

#include <iomanip>
#include <iostream>

int main() {
    const double robust_values[] = {0.10, 0.20, 0.30, 0.40, 0.50, 0.90};
    const double far[] = {0.30, 0.31, 0.32, 0.31};
    const double near_values[] = {0.48, 0.49, 0.50, 0.49};
    const double center[] = {0.90, 0.92, 0.88, 0.91, 0.90, 0.89};
    const double quality[] = {0.75, 0.80, 0.78, 0.79, 0.77, 0.76};

    const double robust = fp_bio_robust_mean(robust_values, 6);
    const double guided = fp_bio_guided_capture_score(
        far, 4,
        near_values, 4,
        center, 6,
        quality, 6
    );
    const double liveness_with_pad = fp_bio_identity_liveness_score(
        0.92, 1, guided, 0.65, 0.78, 1.0
    );
    const double liveness_without_pad = fp_bio_identity_liveness_score(
        0.92, 0, guided, 0.65, 0.78, 1.0
    );

    const double pixel_differences[] = {
        0.012, 0.018, 0.025, 0.021, 0.017
    };
    const FPBiometricCenter centers[] = {
        {0.500, 0.460, 0.220},
        {0.505, 0.462, 0.225},
        {0.511, 0.465, 0.233},
        {0.516, 0.467, 0.240},
        {0.521, 0.470, 0.248},
        {0.525, 0.472, 0.255},
    };
    const double temporal = fp_bio_temporal_motion_score(
        pixel_differences,
        5,
        centers,
        6
    );

    const int challenge_indices[] = {
        0, 0, 1, 1, 2, 2, 3, 3
    };
    const double observed_brightness[] = {
        0.30, 0.31,
        0.43, 0.44,
        0.56, 0.55,
        0.68, 0.69,
    };
    const double illumination_pattern[] = {
        0.25, 0.40, 0.55, 0.70
    };
    double correlation = 0.0;
    const double illumination = fp_bio_illumination_correlation(
        challenge_indices,
        observed_brightness,
        8,
        illumination_pattern,
        4,
        &correlation
    );

    std::cout << std::setprecision(17)
              << "{"
              << "\"robustMean\":" << robust << ","
              << "\"guidedScore\":" << guided << ","
              << "\"livenessWithPad\":" << liveness_with_pad << ","
              << "\"livenessWithoutPad\":" << liveness_without_pad << ","
              << "\"temporalScore\":" << temporal << ","
              << "\"illuminationScore\":" << illumination << ","
              << "\"illuminationCorrelation\":" << correlation
              << "}\n";
    return 0;
}

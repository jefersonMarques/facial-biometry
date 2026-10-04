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

    std::cout << std::setprecision(17)
              << "{"
              << "\"robustMean\":" << robust << ","
              << "\"guidedScore\":" << guided << ","
              << "\"livenessWithPad\":" << liveness_with_pad << ","
              << "\"livenessWithoutPad\":" << liveness_without_pad
              << "}\n";
    return 0;
}

#include "faceproof_biometric.h"

#include <cmath>
#include <cstdlib>
#include <iostream>
#include <string>

namespace {

void require(bool condition, const char* message) {
    if (!condition) {
        std::cerr << "FAILED: " << message << "\n";
        std::exit(1);
    }
}

bool near(double left, double right, double tolerance = 1e-12) {
    return std::abs(left - right) <= tolerance;
}

}  // namespace

int main() {
    require(
        std::string(fp_biometric_version()) == FP_BIOMETRIC_CORE_VERSION,
        "version must match"
    );

    const double robust_values[] = {0.10, 0.20, 0.30, 0.40, 0.50, 0.90};
    require(
        near(fp_bio_robust_mean(robust_values, 6), 0.35),
        "robust mean must match NumPy percentile trimming"
    );

    const double far[] = {0.30, 0.31, 0.32, 0.31};
    const double near_values[] = {0.48, 0.49, 0.50, 0.49};
    const double center[] = {0.90, 0.92, 0.88, 0.91, 0.90, 0.89};
    const double quality[] = {0.75, 0.80, 0.78, 0.79, 0.77, 0.76};

    const double guided = fp_bio_guided_capture_score(
        far, 4,
        near_values, 4,
        center, 6,
        quality, 6
    );
    require(guided > 0.80 && guided < 1.0, "guided score must be strong");

    const double with_pad = fp_bio_identity_liveness_score(
        0.92, 1, guided, 0.65, 0.78, 1.0
    );
    const double without_pad = fp_bio_identity_liveness_score(
        0.92, 0, guided, 0.65, 0.78, 1.0
    );

    require(with_pad > without_pad, "passive PAD must strengthen liveness score");
    require(without_pad <= 0.72, "fallback liveness must remain capped");

    std::cout << "FaceProof biometric core scoring tests passed\n";
    return 0;
}

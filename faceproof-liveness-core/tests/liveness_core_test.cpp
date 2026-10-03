#include "faceproof_liveness.h"

#include <cmath>
#include <cstdlib>
#include <iostream>
#include <string>
#include <vector>

namespace {

constexpr std::size_t kLandmarkCount = 478;

void require(bool condition, const char* message) {
    if (!condition) {
        std::cerr << "FAILED: " << message << "\n";
        std::exit(1);
    }
}

bool near(double left, double right, double tolerance = 1e-8) {
    return std::abs(left - right) <= tolerance;
}

std::vector<FPLandmark> make_landmarks(
    double scale,
    bool perspective_change,
    double nose_depth_ratio
) {
    std::vector<FPLandmark> landmarks(kLandmarkCount);

    const double center_x = 0.5;
    const double center_y = 0.5;
    const double nose_width = (perspective_change ? 0.220 : 0.210) * scale;
    const double eye_span = (perspective_change ? 0.620 : 0.610) * scale;
    const double mouth_width = (perspective_change ? 0.414 : 0.410) * scale;
    const double nose_to_chin = (perspective_change ? 0.580 : 0.590) * scale;
    const double nose_to_forehead = (perspective_change ? 0.500 : 0.510) * scale;

    landmarks[234] = {center_x - scale / 2.0, center_y, nose_depth_ratio * scale};
    landmarks[454] = {center_x + scale / 2.0, center_y, nose_depth_ratio * scale};
    landmarks[1] = {center_x, center_y, 0.0};
    landmarks[98] = {center_x - nose_width / 2.0, center_y, 0.0};
    landmarks[327] = {center_x + nose_width / 2.0, center_y, 0.0};
    landmarks[33] = {center_x - eye_span / 2.0, center_y, 0.0};
    landmarks[263] = {center_x + eye_span / 2.0, center_y, 0.0};
    landmarks[61] = {center_x - mouth_width / 2.0, center_y, 0.0};
    landmarks[291] = {center_x + mouth_width / 2.0, center_y, 0.0};
    landmarks[152] = {center_x, center_y + nose_to_chin, 0.0};
    landmarks[10] = {center_x, center_y - nose_to_forehead, 0.0};

    return landmarks;
}

void push_phase(
    FPContext* context,
    FPPhase phase,
    double base_scale,
    bool perspective_change,
    double nose_depth_ratio,
    double timestamp_base
) {
    require(fp_begin_phase(context, phase) == FP_OK, "phase must start");

    for (int index = 0; index < 6; ++index) {
        const auto landmarks = make_landmarks(
            base_scale + static_cast<double>(index) * 0.0005,
            perspective_change,
            nose_depth_ratio
        );

        require(
            fp_push_landmarks(
                context,
                landmarks.data(),
                static_cast<uint32_t>(landmarks.size()),
                timestamp_base + static_cast<double>(index) * 80.0
            ) == FP_OK,
            "landmarks must be accepted"
        );
    }
}

FPLivenessResult run_case(bool perspective_change, double near_scale) {
    FPContext* context = fp_create();
    require(context != nullptr, "context must be created");

    push_phase(context, FP_PHASE_FAR, 0.32, false, 0.180, 0.0);
    push_phase(
        context,
        FP_PHASE_NEAR,
        near_scale,
        perspective_change,
        perspective_change ? 0.195 : 0.180,
        700.0
    );

    FPLivenessResult result{};
    require(fp_get_result(context, &result) == FP_OK, "result must be available");
    fp_destroy(context);
    return result;
}

}  // namespace

int main() {
    require(std::string(fp_version()) == FP_LIVENESS_CORE_VERSION, "version must match");

    {
        FPContext* context = fp_create();
        require(context != nullptr, "context must be created");
        const auto landmarks = make_landmarks(0.32, false, 0.180);
        require(
            fp_push_landmarks(
                context,
                landmarks.data(),
                static_cast<uint32_t>(landmarks.size()),
                0.0
            ) == FP_ERR_PHASE_NOT_SET,
            "phase is required before samples"
        );
        fp_destroy(context);
    }

    {
        FPContext* context = fp_create();
        require(context != nullptr, "context must be created");
        push_phase(context, FP_PHASE_FAR, 0.32, false, 0.180, 0.0);

        FPLivenessResult result{};
        require(fp_get_result(context, &result) == FP_OK, "partial result must be readable");
        require(result.status == FP_LIVENESS_INSUFFICIENT, "one phase is insufficient");
        require(result.far_samples == 6, "far samples must be counted");
        require(result.near_samples == 0, "near samples must be zero");
        fp_destroy(context);
    }

    const auto planar = run_case(false, 0.52);
    require(planar.status == FP_LIVENESS_EXPERIMENTAL, "clear transition must be collected");
    require(planar.scale_ratio > 1.5, "scale ratio must show near transition");
    require(planar.transition_score > 0.75, "transition score must be strong");
    require(near(planar.perspective_change, 0.0), "planar scaling must keep perspective");

    const auto geometry = run_case(true, 0.52);
    require(
        geometry.perspective_change > planar.perspective_change,
        "geometry response must raise perspective change"
    );
    require(
        geometry.depth_change > planar.depth_change,
        "geometry response must raise depth change"
    );
    require(
        geometry.evidence_score > planar.evidence_score,
        "geometry response must raise evidence"
    );

    const auto weak_transition = run_case(true, 0.34);
    require(
        weak_transition.status == FP_LIVENESS_INSUFFICIENT,
        "weak far-to-near transition must remain insufficient"
    );

    std::cout << "FaceProof liveness core tests passed\n";
    return 0;
}

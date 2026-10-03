#include "internal.h"

#include <array>
#include <cmath>

namespace faceproof {
namespace {

constexpr std::uint32_t kLeftCheek = 234;
constexpr std::uint32_t kRightCheek = 454;
constexpr std::uint32_t kNoseTip = 1;
constexpr std::uint32_t kNoseLeft = 98;
constexpr std::uint32_t kNoseRight = 327;
constexpr std::uint32_t kLeftEyeOuter = 33;
constexpr std::uint32_t kRightEyeOuter = 263;
constexpr std::uint32_t kMouthLeft = 61;
constexpr std::uint32_t kMouthRight = 291;
constexpr std::uint32_t kChin = 152;
constexpr std::uint32_t kForehead = 10;

double distance2d(const FPLandmark& left, const FPLandmark& right) {
    return std::hypot(right.x - left.x, right.y - left.y);
}

bool finite_landmark(const FPLandmark& landmark) {
    return std::isfinite(landmark.x) &&
        std::isfinite(landmark.y) &&
        std::isfinite(landmark.z);
}

}  // namespace

bool extract_observation(
    const FPLandmark* landmarks,
    std::uint32_t landmark_count,
    FPPhase phase,
    double timestamp_ms,
    Observation& output
) {
    if (!landmarks || landmark_count <= kRightCheek || !std::isfinite(timestamp_ms)) {
        return false;
    }

    const auto& left_cheek = landmarks[kLeftCheek];
    const auto& right_cheek = landmarks[kRightCheek];
    const auto& nose_tip = landmarks[kNoseTip];
    const auto& nose_left = landmarks[kNoseLeft];
    const auto& nose_right = landmarks[kNoseRight];
    const auto& left_eye_outer = landmarks[kLeftEyeOuter];
    const auto& right_eye_outer = landmarks[kRightEyeOuter];
    const auto& mouth_left = landmarks[kMouthLeft];
    const auto& mouth_right = landmarks[kMouthRight];
    const auto& chin = landmarks[kChin];
    const auto& forehead = landmarks[kForehead];

    const std::array<const FPLandmark*, 11> required = {
        &left_cheek, &right_cheek, &nose_tip, &nose_left, &nose_right,
        &left_eye_outer, &right_eye_outer, &mouth_left, &mouth_right,
        &chin, &forehead,
    };

    for (const auto* landmark : required) {
        if (!finite_landmark(*landmark)) {
            return false;
        }
    }

    const double scale = distance2d(left_cheek, right_cheek);
    if (!std::isfinite(scale) || scale < 0.05) {
        return false;
    }

    const double left_nose_distance = distance2d(nose_tip, left_cheek);
    const double right_nose_distance = distance2d(nose_tip, right_cheek);
    const double cheek_depth = (left_cheek.z + right_cheek.z) / 2.0;

    output.timestamp_ms = timestamp_ms;
    output.phase = phase;
    output.scale = scale;
    output.nose_width_ratio = distance2d(nose_left, nose_right) / scale;
    output.eye_span_ratio = distance2d(left_eye_outer, right_eye_outer) / scale;
    output.mouth_width_ratio = distance2d(mouth_left, mouth_right) / scale;
    output.nose_to_chin_ratio = distance2d(nose_tip, chin) / scale;
    output.nose_to_forehead_ratio = distance2d(nose_tip, forehead) / scale;
    output.nose_depth_ratio = (cheek_depth - nose_tip.z) / scale;
    output.yaw_asymmetry = std::abs(left_nose_distance - right_nose_distance) / scale;

    return true;
}

}  // namespace faceproof

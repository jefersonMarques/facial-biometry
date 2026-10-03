#include "internal.h"

#include <algorithm>
#include <array>
#include <cmath>
#include <limits>

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


namespace faceproof {

bool analyze_guide(
    const FPLandmark* landmarks,
    std::uint32_t landmark_count,
    FPGuideResult& result
) {
    result = {};
    if (!landmarks || landmark_count == 0) {
        return false;
    }

    double min_x = std::numeric_limits<double>::infinity();
    double min_y = std::numeric_limits<double>::infinity();
    double max_x = -std::numeric_limits<double>::infinity();
    double max_y = -std::numeric_limits<double>::infinity();
    std::uint32_t finite_count = 0;

    for (std::uint32_t index = 0; index < landmark_count; ++index) {
        const auto& landmark = landmarks[index];
        if (!finite_landmark(landmark)) {
            continue;
        }

        min_x = std::min(min_x, landmark.x);
        min_y = std::min(min_y, landmark.y);
        max_x = std::max(max_x, landmark.x);
        max_y = std::max(max_y, landmark.y);
        finite_count++;
    }

    if (finite_count < 50 || !std::isfinite(min_x) || !std::isfinite(min_y) ||
        !std::isfinite(max_x) || !std::isfinite(max_y)) {
        return false;
    }

    const double width = std::max(0.0, max_x - min_x);
    const double height = std::max(0.0, max_y - min_y);
    if (width < 0.02 || height < 0.02) {
        return false;
    }

    result.face_detected = 1;
    result.center_x = clamp01(min_x + width / 2.0);
    result.center_y = clamp01(min_y + height / 2.0);
    result.width_ratio = clamp01(width);
    result.height_ratio = clamp01(height);

    if (landmark_count > kRightEyeOuter) {
        const auto& left_eye = landmarks[kLeftEyeOuter];
        const auto& right_eye = landmarks[kRightEyeOuter];
        if (finite_landmark(left_eye) && finite_landmark(right_eye)) {
            constexpr double kRadiansToDegrees = 57.2957795130823208768;
            result.roll_degrees = std::atan2(
                right_eye.y - left_eye.y,
                right_eye.x - left_eye.x
            ) * kRadiansToDegrees;
        }
    }

    const double area_ratio = width * height;
    if (area_ratio < 0.07) {
        result.face_size_score = clamp01(area_ratio / 0.07);
    } else if (area_ratio > 0.62) {
        result.face_size_score = clamp01(1.0 - ((area_ratio - 0.62) / 0.30));
    } else {
        result.face_size_score = 1.0;
    }

    return true;
}

}  // namespace faceproof

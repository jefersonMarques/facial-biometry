#include "internal.h"

#include <cmath>
#include <vector>

namespace faceproof {
namespace {

FPLivenessResult base_result(
    std::uint32_t sample_count,
    std::uint32_t far_samples,
    std::uint32_t near_samples
) {
    FPLivenessResult result{};
    result.status = FP_LIVENESS_INSUFFICIENT;
    result.sample_count = sample_count;
    result.far_samples = far_samples;
    result.near_samples = near_samples;
    return result;
}

}  // namespace

FPLivenessResult summarize(const std::vector<Observation>& observations) {
    std::vector<Observation> far;
    std::vector<Observation> near;
    far.reserve(observations.size());
    near.reserve(observations.size());

    for (const auto& item : observations) {
        if (item.phase == FP_PHASE_FAR) {
            far.push_back(item);
        } else if (item.phase == FP_PHASE_NEAR) {
            near.push_back(item);
        }
    }

    auto result = base_result(
        static_cast<std::uint32_t>(observations.size()),
        static_cast<std::uint32_t>(far.size()),
        static_cast<std::uint32_t>(near.size())
    );

    if (far.size() < kMinPhaseSamples || near.size() < kMinPhaseSamples) {
        return result;
    }

    const auto far_metrics = aggregate_phase(far);
    const auto near_metrics = aggregate_phase(near);
    if (far_metrics.scale <= 0.0) {
        return result;
    }

    result.scale_ratio = near_metrics.scale / far_metrics.scale;
    result.transition_score = clamp01((result.scale_ratio - 1.15) / 0.45);

    result.perspective_change = median({
        relative_change(far_metrics.nose_width_ratio, near_metrics.nose_width_ratio),
        relative_change(far_metrics.eye_span_ratio, near_metrics.eye_span_ratio),
        relative_change(far_metrics.mouth_width_ratio, near_metrics.mouth_width_ratio),
        relative_change(far_metrics.nose_to_chin_ratio, near_metrics.nose_to_chin_ratio),
        relative_change(far_metrics.nose_to_forehead_ratio, near_metrics.nose_to_forehead_ratio),
    });

    result.depth_change = std::abs(
        near_metrics.nose_depth_ratio - far_metrics.nose_depth_ratio
    );

    result.phase_stability = clamp01(
        1.0 - median({
            phase_variation(far),
            phase_variation(near),
        }) / 0.05
    );

    const double perspective_score = clamp01((result.perspective_change - 0.003) / 0.030);
    const double depth_score = clamp01((result.depth_change - 0.002) / 0.025);

    result.evidence_score = clamp01(
        0.45 * result.transition_score +
        0.30 * perspective_score +
        0.15 * depth_score +
        0.10 * result.phase_stability
    );

    result.status = result.scale_ratio >= 1.10
        ? FP_LIVENESS_EXPERIMENTAL
        : FP_LIVENESS_INSUFFICIENT;

    return result;
}

}  // namespace faceproof

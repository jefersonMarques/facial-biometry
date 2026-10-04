#include "faceproof_biometric.h"

#include <algorithm>
#include <cmath>
#include <numeric>
#include <vector>

namespace {

double clamp01(double value) {
    return std::max(0.0, std::min(1.0, value));
}

double mean(const std::vector<double>& values) {
    if (values.empty()) {
        return 0.0;
    }
    return std::accumulate(values.begin(), values.end(), 0.0) /
        static_cast<double>(values.size());
}

double percentile_linear(std::vector<double> values, double quantile) {
    if (values.empty()) {
        return 0.0;
    }
    std::sort(values.begin(), values.end());
    if (values.size() == 1) {
        return values.front();
    }

    const double position =
        static_cast<double>(values.size() - 1) * quantile;
    const auto lower_index = static_cast<std::size_t>(std::floor(position));
    const auto upper_index = static_cast<std::size_t>(std::ceil(position));
    if (lower_index == upper_index) {
        return values[lower_index];
    }

    const double weight = position - static_cast<double>(lower_index);
    return values[lower_index] * (1.0 - weight) +
        values[upper_index] * weight;
}

double median(const double* values, std::size_t count) {
    if (values == nullptr || count == 0) {
        return 0.0;
    }
    std::vector<double> sorted(values, values + count);
    std::sort(sorted.begin(), sorted.end());
    const std::size_t middle = count / 2;
    if (count % 2 == 1) {
        return sorted[middle];
    }
    return (sorted[middle - 1] + sorted[middle]) / 2.0;
}

}  // namespace

extern "C" {

double fp_bio_robust_mean(const double* values, size_t count) {
    if (values == nullptr || count == 0) {
        return 0.0;
    }

    std::vector<double> data(values, values + count);
    if (count < 5) {
        return mean(data);
    }

    const double lower = percentile_linear(data, 0.15);
    const double upper = percentile_linear(data, 0.85);

    std::vector<double> trimmed;
    trimmed.reserve(data.size());
    for (double value : data) {
        if (value >= lower && value <= upper) {
            trimmed.push_back(value);
        }
    }
    return trimmed.empty() ? mean(data) : mean(trimmed);
}

double fp_bio_guided_capture_score(
    const double* far_scales,
    size_t far_count,
    const double* near_scales,
    size_t near_count,
    const double* center_scores,
    size_t center_count,
    const double* quality_scores,
    size_t quality_count
) {
    if (far_scales == nullptr || far_count == 0 ||
        near_scales == nullptr || near_count == 0) {
        return 0.0;
    }

    const double far_median = median(far_scales, far_count);
    const double near_median = median(near_scales, near_count);
    const double scale_delta = near_median - far_median;
    const double transition_score = clamp01((scale_delta - 0.05) / 0.15);
    const double coverage_score = clamp01(
        static_cast<double>(std::min(far_count, near_count)) / 3.0
    );
    const double centering_score =
        center_scores != nullptr && center_count > 0
            ? fp_bio_robust_mean(center_scores, center_count)
            : 0.0;
    const double quality_score =
        quality_scores != nullptr && quality_count > 0
            ? fp_bio_robust_mean(quality_scores, quality_count)
            : 0.0;

    return clamp01(
        0.40 * transition_score +
        0.25 * coverage_score +
        0.20 * centering_score +
        0.15 * quality_score
    );
}

double fp_bio_identity_liveness_score(
    double passive_score,
    int passive_available,
    double guided_score,
    double temporal_score,
    double quality_score,
    double face_presence
) {
    const double quality_gate = clamp01((quality_score - 0.25) / 0.55);
    const double presence_gate = clamp01((face_presence - 0.55) / 0.45);

    double liveness_score = 0.0;
    if (passive_available != 0) {
        liveness_score =
            0.55 * passive_score +
            0.20 * guided_score +
            0.10 * temporal_score +
            0.10 * quality_score +
            0.05 * face_presence;
    } else {
        liveness_score =
            0.36 * guided_score +
            0.24 * temporal_score +
            0.22 * quality_score +
            0.18 * face_presence;
        liveness_score = std::min(liveness_score, 0.72);
    }

    return clamp01(
        liveness_score *
        (0.82 + 0.18 * quality_gate) *
        (0.80 + 0.20 * presence_gate)
    );
}

const char* fp_biometric_version(void) {
    return FP_BIOMETRIC_CORE_VERSION;
}

}  // extern "C"

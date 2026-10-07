#include "faceproof_biometric.h"

#include <algorithm>
#include <cmath>
#include <map>
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

double fp_bio_temporal_motion_score(
    const double* pixel_differences,
    size_t pixel_difference_count,
    const FPBiometricCenter* centers,
    size_t center_count
) {
    if (center_count < 3) {
        return 0.0;
    }

    const double pixel_median =
        pixel_differences != nullptr && pixel_difference_count > 0
            ? median(pixel_differences, pixel_difference_count)
            : 0.0;

    std::vector<double> position_differences;
    if (centers != nullptr && center_count > 1) {
        position_differences.reserve(center_count - 1);
        for (size_t index = 1; index < center_count; ++index) {
            const double dx = centers[index].x - centers[index - 1].x;
            const double dy = centers[index].y - centers[index - 1].y;
            const double ds =
                centers[index].scale - centers[index - 1].scale;
            position_differences.push_back(
                std::sqrt(dx * dx + dy * dy + ds * ds)
            );
        }
    }

    const double position_median =
        position_differences.empty()
            ? 0.0
            : median(
                position_differences.data(),
                position_differences.size()
            );

    const double pixel_score =
        clamp01((pixel_median - 0.006) / 0.055);
    const double position_score =
        clamp01(position_median / 0.045);
    const double too_dynamic_penalty =
        clamp01((pixel_median - 0.16) / 0.20);

    return clamp01(
        (0.65 * pixel_score + 0.35 * position_score) *
        (1.0 - 0.45 * too_dynamic_penalty)
    );
}

double fp_bio_illumination_correlation(
    const int* challenge_indices,
    const double* observed_brightness,
    size_t observation_count,
    const double* illumination_pattern,
    size_t pattern_count,
    double* correlation
) {
    if (correlation != nullptr) {
        *correlation = 0.0;
    }
    if (challenge_indices == nullptr ||
        observed_brightness == nullptr ||
        illumination_pattern == nullptr ||
        observation_count == 0 ||
        pattern_count == 0) {
        return 0.5;
    }

    std::map<int, std::vector<double>> grouped;
    for (size_t index = 0; index < observation_count; ++index) {
        const int challenge_index = challenge_indices[index];
        if (challenge_index >= 0 &&
            static_cast<size_t>(challenge_index) < pattern_count) {
            grouped[challenge_index].push_back(
                observed_brightness[index]
            );
        }
    }

    if (grouped.size() < 4) {
        return 0.5;
    }

    std::vector<double> expected;
    std::vector<double> observed;
    expected.reserve(grouped.size());
    observed.reserve(grouped.size());

    for (const auto& item : grouped) {
        if (item.second.empty()) {
            continue;
        }
        expected.push_back(
            illumination_pattern[static_cast<size_t>(item.first)]
        );
        observed.push_back(
            median(item.second.data(), item.second.size())
        );
    }

    if (expected.size() < 4) {
        return 0.5;
    }

    const double expected_mean = mean(expected);
    const double observed_mean = mean(observed);

    double expected_variance = 0.0;
    double observed_variance = 0.0;
    double covariance = 0.0;

    for (size_t index = 0; index < expected.size(); ++index) {
        const double expected_delta =
            expected[index] - expected_mean;
        const double observed_delta =
            observed[index] - observed_mean;
        expected_variance += expected_delta * expected_delta;
        observed_variance += observed_delta * observed_delta;
        covariance += expected_delta * observed_delta;
    }

    expected_variance /= static_cast<double>(expected.size());
    observed_variance /= static_cast<double>(observed.size());

    const double expected_std = std::sqrt(expected_variance);
    const double observed_std = std::sqrt(observed_variance);
    if (expected_std < 1e-6 || observed_std < 0.004) {
        return 0.5;
    }

    covariance /= static_cast<double>(expected.size());
    const double value =
        covariance / (expected_std * observed_std);
    if (!std::isfinite(value)) {
        return 0.5;
    }

    if (correlation != nullptr) {
        *correlation = value;
    }
    return clamp01((value + 0.15) / 1.15);
}


const char* fp_biometric_version(void) {
    return FP_BIOMETRIC_CORE_VERSION;
}

}  // extern "C"

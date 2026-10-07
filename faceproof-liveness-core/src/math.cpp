#include "internal.h"

#include <algorithm>
#include <array>
#include <cmath>
#include <utility>
#include <vector>

namespace faceproof {

double clamp01(double value) {
    return std::max(0.0, std::min(1.0, value));
}

double median(std::vector<double> values) {
    if (values.empty()) {
        return 0.0;
    }

    const auto middle = values.begin() + static_cast<std::ptrdiff_t>(values.size() / 2);
    std::nth_element(values.begin(), middle, values.end());
    const double right = *middle;

    if (values.size() % 2 == 1) {
        return right;
    }

    const auto left = std::max_element(values.begin(), middle);
    return (*left + right) / 2.0;
}

double relative_change(double from, double to) {
    const double denominator = std::max(std::abs(from), 1e-6);
    return std::abs(to - from) / denominator;
}

namespace {

double coefficient_of_variation(const std::vector<double>& values) {
    const double center = median(values);
    if (std::abs(center) < 1e-8) {
        return 0.0;
    }

    std::vector<double> deviations;
    deviations.reserve(values.size());
    for (const double value : values) {
        deviations.push_back(std::abs(value - center));
    }

    return median(std::move(deviations)) / std::abs(center);
}

}  // namespace

double phase_variation(const std::vector<Observation>& observations) {
    std::array<std::vector<double>, 5> features;
    for (auto& feature : features) {
        feature.reserve(observations.size());
    }

    for (const auto& item : observations) {
        features[0].push_back(item.nose_width_ratio);
        features[1].push_back(item.eye_span_ratio);
        features[2].push_back(item.mouth_width_ratio);
        features[3].push_back(item.nose_to_chin_ratio);
        features[4].push_back(item.nose_to_forehead_ratio);
    }

    std::vector<double> variations;
    variations.reserve(features.size());
    for (const auto& feature : features) {
        variations.push_back(coefficient_of_variation(feature));
    }

    return median(std::move(variations));
}

PhaseAggregate aggregate_phase(const std::vector<Observation>& observations) {
    PhaseAggregate aggregate;

    std::vector<double> scale;
    std::vector<double> nose_width;
    std::vector<double> eye_span;
    std::vector<double> mouth_width;
    std::vector<double> nose_to_chin;
    std::vector<double> nose_to_forehead;
    std::vector<double> nose_depth;

    const auto count = observations.size();
    scale.reserve(count);
    nose_width.reserve(count);
    eye_span.reserve(count);
    mouth_width.reserve(count);
    nose_to_chin.reserve(count);
    nose_to_forehead.reserve(count);
    nose_depth.reserve(count);

    for (const auto& item : observations) {
        scale.push_back(item.scale);
        nose_width.push_back(item.nose_width_ratio);
        eye_span.push_back(item.eye_span_ratio);
        mouth_width.push_back(item.mouth_width_ratio);
        nose_to_chin.push_back(item.nose_to_chin_ratio);
        nose_to_forehead.push_back(item.nose_to_forehead_ratio);
        nose_depth.push_back(item.nose_depth_ratio);
    }

    aggregate.scale = median(std::move(scale));
    aggregate.nose_width_ratio = median(std::move(nose_width));
    aggregate.eye_span_ratio = median(std::move(eye_span));
    aggregate.mouth_width_ratio = median(std::move(mouth_width));
    aggregate.nose_to_chin_ratio = median(std::move(nose_to_chin));
    aggregate.nose_to_forehead_ratio = median(std::move(nose_to_forehead));
    aggregate.nose_depth_ratio = median(std::move(nose_depth));

    return aggregate;
}

}  // namespace faceproof

#pragma once

#include "faceproof_liveness.h"

#include <cstdint>
#include <vector>

namespace faceproof {

constexpr std::uint32_t kMinPhaseSamples = 4;
constexpr std::size_t kMaxObservations = 120;

struct Observation {
    double timestamp_ms = 0.0;
    FPPhase phase = FP_PHASE_FAR;
    double scale = 0.0;
    double nose_width_ratio = 0.0;
    double eye_span_ratio = 0.0;
    double mouth_width_ratio = 0.0;
    double nose_to_chin_ratio = 0.0;
    double nose_to_forehead_ratio = 0.0;
    double nose_depth_ratio = 0.0;
    double yaw_asymmetry = 0.0;
};

struct PhaseAggregate {
    double scale = 0.0;
    double nose_width_ratio = 0.0;
    double eye_span_ratio = 0.0;
    double mouth_width_ratio = 0.0;
    double nose_to_chin_ratio = 0.0;
    double nose_to_forehead_ratio = 0.0;
    double nose_depth_ratio = 0.0;
};

double clamp01(double value);
double median(std::vector<double> values);
double relative_change(double from, double to);
double phase_variation(const std::vector<Observation>& observations);
PhaseAggregate aggregate_phase(const std::vector<Observation>& observations);

bool extract_observation(
    const FPLandmark* landmarks,
    std::uint32_t landmark_count,
    FPPhase phase,
    double timestamp_ms,
    Observation& output
);

FPLivenessResult summarize(const std::vector<Observation>& observations);

}  // namespace faceproof

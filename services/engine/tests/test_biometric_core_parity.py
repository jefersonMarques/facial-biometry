from __future__ import annotations

import json
import os
import subprocess
import unittest

import numpy as np

from biometric_engine.metrics import (
    guided_capture_score,
    identity_liveness_score,
    illumination_correlation,
    robust_mean,
)


class BiometricCoreParityTest(unittest.TestCase):
    def test_cpp_scoring_matches_python(self) -> None:
        executable = os.environ.get("FACEPROOF_BIOMETRIC_MATH_CLI")
        if not executable:
            self.skipTest("FACEPROOF_BIOMETRIC_MATH_CLI is not configured")

        completed = subprocess.run(
            [executable],
            check=True,
            capture_output=True,
            text=True,
        )
        actual = json.loads(completed.stdout)

        robust_values = [0.10, 0.20, 0.30, 0.40, 0.50, 0.90]
        far = [0.30, 0.31, 0.32, 0.31]
        near = [0.48, 0.49, 0.50, 0.49]
        center = [0.90, 0.92, 0.88, 0.91, 0.90, 0.89]
        quality = [0.75, 0.80, 0.78, 0.79, 0.77, 0.76]

        expected_robust = robust_mean(robust_values)
        expected_guided = guided_capture_score(far, near, center, quality)
        expected_with_pad = identity_liveness_score(
            passive_score=0.92,
            passive_available=True,
            guided_score=expected_guided,
            temporal_score=0.65,
            quality_score=0.78,
            face_presence=1.0,
        )
        expected_without_pad = identity_liveness_score(
            passive_score=0.92,
            passive_available=False,
            guided_score=expected_guided,
            temporal_score=0.65,
            quality_score=0.78,
            face_presence=1.0,
        )

        pixel_differences = [0.012, 0.018, 0.025, 0.021, 0.017]
        centers = [
            (0.500, 0.460, 0.220),
            (0.505, 0.462, 0.225),
            (0.511, 0.465, 0.233),
            (0.516, 0.467, 0.240),
            (0.521, 0.470, 0.248),
            (0.525, 0.472, 0.255),
        ]
        position_differences = []
        for left, right in zip(centers, centers[1:]):
            dx = right[0] - left[0]
            dy = right[1] - left[1]
            ds = right[2] - left[2]
            position_differences.append(
                (dx * dx + dy * dy + ds * ds) ** 0.5
            )

        pixel_median = float(np.median(pixel_differences))
        position_median = float(np.median(position_differences))
        pixel_score = max(
            0.0,
            min(1.0, (pixel_median - 0.006) / 0.055),
        )
        position_score = max(
            0.0,
            min(1.0, position_median / 0.045),
        )
        dynamic_penalty = max(
            0.0,
            min(1.0, (pixel_median - 0.16) / 0.20),
        )
        expected_temporal = max(
            0.0,
            min(
                1.0,
                (
                    0.65 * pixel_score
                    + 0.35 * position_score
                )
                * (1.0 - 0.45 * dynamic_penalty),
            ),
        )

        challenge_indices = [0, 0, 1, 1, 2, 2, 3, 3]
        observed_brightness = [
            0.30,
            0.31,
            0.43,
            0.44,
            0.56,
            0.55,
            0.68,
            0.69,
        ]
        illumination_pattern = [0.25, 0.40, 0.55, 0.70]
        expected_illumination, expected_correlation = (
            illumination_correlation(
                challenge_indices,
                observed_brightness,
                illumination_pattern,
            )
        )

        self.assertAlmostEqual(actual["robustMean"], expected_robust, places=14)
        self.assertAlmostEqual(actual["guidedScore"], expected_guided, places=14)
        self.assertAlmostEqual(actual["livenessWithPad"], expected_with_pad, places=14)
        self.assertAlmostEqual(actual["livenessWithoutPad"], expected_without_pad, places=14)
        self.assertAlmostEqual(actual["temporalScore"], expected_temporal, places=14)
        self.assertAlmostEqual(
            actual["illuminationScore"],
            expected_illumination,
            places=14,
        )
        self.assertAlmostEqual(
            actual["illuminationCorrelation"],
            expected_correlation,
            places=14,
        )


if __name__ == "__main__":
    unittest.main()

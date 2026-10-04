from __future__ import annotations

import json
import os
import subprocess
import unittest

from biometric_engine.metrics import (
    guided_capture_score,
    identity_liveness_score,
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

        self.assertAlmostEqual(actual["robustMean"], expected_robust, places=14)
        self.assertAlmostEqual(actual["guidedScore"], expected_guided, places=14)
        self.assertAlmostEqual(actual["livenessWithPad"], expected_with_pad, places=14)
        self.assertAlmostEqual(actual["livenessWithoutPad"], expected_without_pad, places=14)


if __name__ == "__main__":
    unittest.main()

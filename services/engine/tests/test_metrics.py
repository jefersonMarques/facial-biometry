import unittest

import numpy as np

from biometric_engine.metrics import clamp01, illumination_correlation


class MetricsTest(unittest.TestCase):
    def test_clamp01(self) -> None:
        self.assertEqual(clamp01(-1.0), 0.0)
        self.assertEqual(clamp01(2.0), 1.0)
        self.assertEqual(clamp01(0.4), 0.4)

    def test_illumination_correlation_tracks_pattern(self) -> None:
        pattern = [0.2, 0.8, 0.3, 0.9, 0.4, 0.7]
        indices = [0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5]
        observed = [0.22, 0.21, 0.78, 0.79, 0.31, 0.30, 0.88, 0.89, 0.42, 0.41, 0.69, 0.70]
        score, correlation = illumination_correlation(indices, observed, pattern)
        self.assertGreater(score, 0.9)
        self.assertGreater(correlation, 0.9)


if __name__ == "__main__":
    unittest.main()

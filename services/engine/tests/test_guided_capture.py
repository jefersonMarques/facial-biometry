import unittest

from biometric_engine.analyzer import _guided_capture_score


class GuidedCaptureScoreTest(unittest.TestCase):
    def test_rewards_clear_far_to_near_transition(self) -> None:
        score = _guided_capture_score(
            far_scales=[0.31, 0.32, 0.33],
            near_scales=[0.53, 0.55, 0.56],
            center_scores=[0.92, 0.95, 0.90, 0.94, 0.91, 0.93],
            quality_scores=[0.62, 0.66, 0.64, 0.70, 0.72, 0.69],
        )
        self.assertGreater(score, 0.70)

    def test_penalizes_missing_scale_transition(self) -> None:
        score = _guided_capture_score(
            far_scales=[0.38, 0.39, 0.40],
            near_scales=[0.41, 0.42, 0.43],
            center_scores=[0.95] * 6,
            quality_scores=[0.70] * 6,
        )
        self.assertLess(score, 0.65)

    def test_requires_both_phases(self) -> None:
        score = _guided_capture_score(
            far_scales=[0.32, 0.33],
            near_scales=[],
            center_scores=[0.95, 0.95],
            quality_scores=[0.70, 0.70],
        )
        self.assertEqual(score, 0.0)


if __name__ == "__main__":
    unittest.main()

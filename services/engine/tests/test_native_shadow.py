from __future__ import annotations

import unittest

import numpy as np

from biometric_engine.native_shadow import (
    compare_identity_shadow,
    compare_reference_shadow,
)


class NativeShadowComparisonTest(unittest.TestCase):
    def test_reference_reports_ok_for_equivalent_outputs(self) -> None:
        embedding = [1.0, 0.0, 0.0]
        python_result = {
            "embedding": embedding,
            "quality": {
                "sharpness": 0.7,
                "brightness": 0.8,
                "faceSize": 1.0,
                "score": 0.8,
            },
        }
        native_result = {
            "embedding": embedding,
            "quality": {
                "sharpness": 0.7,
                "brightness": 0.8,
                "faceSize": 1.0,
                "score": 0.8,
            },
        }

        result = compare_reference_shadow(
            python_result,
            native_result,
        )

        self.assertIsNotNone(result)
        self.assertEqual(result["status"], "ok")
        self.assertEqual(result["embeddingCosine"], 1.0)
        self.assertEqual(result["embeddingMaxDelta"], 0.0)

    def test_identity_reports_drift_when_native_embedding_changes(self) -> None:
        python_frames = {
            4: {
                "bbox": (10, 20, 100, 120),
                "confidence": 0.95,
                "quality": {
                    "sharpness": 0.7,
                    "brightness": 0.8,
                    "faceSize": 1.0,
                    "score": 0.8,
                },
                "passivePad": 0.9,
                "embedding": [1.0, 0.0, 0.0],
            },
            5: {
                "bbox": (10, 20, 100, 120),
                "confidence": 0.95,
                "quality": {
                    "sharpness": 0.7,
                    "brightness": 0.8,
                    "faceSize": 1.0,
                    "score": 0.8,
                },
                "passivePad": 0.9,
                "embedding": [1.0, 0.0, 0.0],
            },
            6: {
                "bbox": (10, 20, 100, 120),
                "confidence": 0.95,
                "quality": {
                    "sharpness": 0.7,
                    "brightness": 0.8,
                    "faceSize": 1.0,
                    "score": 0.8,
                },
                "passivePad": 0.9,
                "embedding": [1.0, 0.0, 0.0],
            },
        }
        native_frames = {
            index: {
                "face": {
                    "x": 10,
                    "y": 20,
                    "width": 100,
                    "height": 120,
                    "confidence": 0.95,
                },
                "quality": {
                    "sharpness": 0.7,
                    "brightness": 0.8,
                    "faceSize": 1.0,
                    "score": 0.8,
                },
                "realProbability": 0.9,
                "embedding": (
                    [0.0, 1.0, 0.0]
                    if index == 6
                    else [1.0, 0.0, 0.0]
                ),
            }
            for index in (4, 5, 6)
        }

        result = compare_identity_shadow(
            python_frames=python_frames,
            native_frames=native_frames,
            selected_frame_indices=[4, 5, 6],
            python_combined_embedding=np.asarray(
                [1.0, 0.0, 0.0],
                dtype=np.float32,
            ),
        )

        self.assertEqual(result["status"], "drift")
        self.assertLess(result["selectedEmbeddingMinCosine"], 0.99999)


    def test_identity_marks_vision_only_shadow_as_partial_when_python_has_pad(self) -> None:
        python_frames = {
            4: {
                "bbox": (10, 20, 100, 120),
                "confidence": 0.95,
                "quality": {
                    "sharpness": 0.7,
                    "brightness": 0.8,
                    "faceSize": 1.0,
                    "score": 0.8,
                },
                "passivePad": 0.9,
                "embedding": [1.0, 0.0, 0.0],
            }
        }
        native_frames = {
            4: {
                "face": {
                    "x": 10,
                    "y": 20,
                    "width": 100,
                    "height": 120,
                    "confidence": 0.95,
                },
                "quality": {
                    "sharpness": 0.7,
                    "brightness": 0.8,
                    "faceSize": 1.0,
                    "score": 0.8,
                },
                "embedding": [1.0, 0.0, 0.0],
                "padStatus": "not_compared",
            }
        }

        result = compare_identity_shadow(
            python_frames=python_frames,
            native_frames=native_frames,
            selected_frame_indices=[4],
            python_combined_embedding=np.asarray(
                [1.0, 0.0, 0.0],
                dtype=np.float32,
            ),
        )

        self.assertEqual(result["status"], "partial")
        self.assertEqual(result["padComparedFrames"], 0)
        self.assertEqual(result["combinedEmbeddingCosine"], 1.0)

    def test_identity_reports_partial_on_native_error(self) -> None:
        result = compare_identity_shadow(
            python_frames={
                0: {
                    "bbox": (1, 2, 3, 4),
                    "confidence": 0.9,
                    "quality": {"score": 0.8},
                    "passivePad": 0.9,
                }
            },
            native_frames={
                0: {"error": "native runtime unavailable"}
            },
            selected_frame_indices=[],
            python_combined_embedding=np.asarray(
                [1.0, 0.0],
                dtype=np.float32,
            ),
        )

        self.assertEqual(result["status"], "partial")
        self.assertIn("native runtime unavailable", result["errors"][0])


if __name__ == "__main__":
    unittest.main()

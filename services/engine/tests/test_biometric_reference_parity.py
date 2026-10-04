from __future__ import annotations

import json
import math
import os
import subprocess
import unittest

import cv2
import numpy as np

from biometric_engine.analyzer import (
    _normalize_reference_lighting,
    _prepare_reference_image,
)
from biometric_engine.image_utils import crop_face
from biometric_engine.metrics import (
    brightness_score,
    clamp01,
    face_size_score,
    sharpness_score,
)
from biometric_engine.models import SFaceEncoder, YuNetDetector


class BiometricReferenceParityTest(unittest.TestCase):
    def test_cpp_reference_ensemble_matches_python(self) -> None:
        cli = os.environ.get("FACEPROOF_BIOMETRIC_REFERENCE_CLI")
        image_path = os.environ.get("FACEPROOF_BIOMETRIC_PARITY_IMAGE")
        yunet_path = os.environ.get("FACEPROOF_YUNET_MODEL")
        sface_path = os.environ.get("FACEPROOF_SFACE_MODEL")

        missing = [
            name
            for name, value in {
                "FACEPROOF_BIOMETRIC_REFERENCE_CLI": cli,
                "FACEPROOF_BIOMETRIC_PARITY_IMAGE": image_path,
                "FACEPROOF_YUNET_MODEL": yunet_path,
                "FACEPROOF_SFACE_MODEL": sface_path,
            }.items()
            if not value
        ]
        if missing:
            self.skipTest("missing parity environment: " + ", ".join(missing))

        completed = subprocess.run(
            [cli, image_path, yunet_path, sface_path],
            check=False,
            capture_output=True,
            text=True,
        )
        self.assertEqual(
            completed.returncode,
            0,
            msg=(
                f"C++ reference probe failed with {completed.returncode}: "
                f"stdout={completed.stdout!r} stderr={completed.stderr!r}"
            ),
        )
        cpp = json.loads(completed.stdout)

        image = cv2.imread(image_path, cv2.IMREAD_COLOR)
        self.assertIsNotNone(image)
        image = _prepare_reference_image(image)

        detector = YuNetDetector(yunet_path)
        detected = detector.detect_primary(image)
        self.assertIsNotNone(detected)

        face_crop = crop_face(image, detected.bbox)
        python_sharpness = sharpness_score(face_crop)
        python_brightness, _ = brightness_score(face_crop)
        python_face_size = face_size_score(image, detected.bbox)
        python_quality = clamp01(
            0.45 * python_sharpness
            + 0.30 * python_brightness
            + 0.25 * python_face_size
        )

        reference_images = [
            image,
            _normalize_reference_lighting(image, clip_limit=1.35),
            _normalize_reference_lighting(image, clip_limit=1.70),
        ]
        encoder = SFaceEncoder(sface_path)
        python_variants = [
            encoder.encode(reference_image, detected).astype(np.float64)
            for reference_image in reference_images
        ]

        python_combined = np.mean(
            np.vstack(python_variants),
            axis=0,
        ).astype(np.float32)
        python_combined = (
            python_combined / float(np.linalg.norm(python_combined))
        ).astype(np.float64)

        cpp_variants = [
            np.asarray(values, dtype=np.float64)
            for values in cpp["embeddings"]
        ]
        cpp_combined = np.asarray(cpp["embedding"], dtype=np.float64)

        self.assertEqual(len(cpp_variants), 3)
        for python_embedding, cpp_embedding in zip(
            python_variants,
            cpp_variants,
            strict=True,
        ):
            self._assert_embedding_close(python_embedding, cpp_embedding)

        self._assert_embedding_close(python_combined, cpp_combined)

        cpp_face = cpp["face"]
        for python_value, cpp_value in zip(
            detected.bbox,
            (
                int(cpp_face["x"]),
                int(cpp_face["y"]),
                int(cpp_face["width"]),
                int(cpp_face["height"]),
            ),
            strict=True,
        ):
            self.assertLessEqual(abs(python_value - cpp_value), 1)

        self.assertAlmostEqual(
            float(detected.confidence),
            float(cpp_face["confidence"]),
            places=5,
        )

        cpp_quality = cpp["quality"]
        self.assertAlmostEqual(
            python_sharpness,
            float(cpp_quality["sharpness"]),
            places=6,
        )
        self.assertAlmostEqual(
            python_brightness,
            float(cpp_quality["brightness"]),
            places=6,
        )
        self.assertAlmostEqual(
            python_face_size,
            float(cpp_quality["faceSize"]),
            places=6,
        )
        self.assertAlmostEqual(
            python_quality,
            float(cpp_quality["score"]),
            places=6,
        )

    def _assert_embedding_close(
        self,
        python_embedding: np.ndarray,
        cpp_embedding: np.ndarray,
    ) -> None:
        self.assertEqual(python_embedding.shape, cpp_embedding.shape)
        cosine = float(
            np.dot(python_embedding, cpp_embedding)
            / (
                np.linalg.norm(python_embedding)
                * np.linalg.norm(cpp_embedding)
            )
        )
        max_delta = float(
            np.max(np.abs(python_embedding - cpp_embedding))
        )

        self.assertTrue(math.isfinite(cosine))
        self.assertGreaterEqual(cosine, 0.99999)
        self.assertLessEqual(max_delta, 5e-4)


if __name__ == "__main__":
    unittest.main()

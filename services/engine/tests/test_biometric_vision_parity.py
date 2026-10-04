from __future__ import annotations

import json
import math
import os
import subprocess
import unittest

import cv2
import numpy as np

from biometric_engine.image_utils import crop_face
from biometric_engine.metrics import (
    brightness_score,
    clamp01,
    face_size_score,
    sharpness_score,
)
from biometric_engine.models import SFaceEncoder, YuNetDetector


class BiometricVisionParityTest(unittest.TestCase):
    def test_cpp_yunet_quality_and_sface_match_python(self) -> None:
        cli = os.environ.get("FACEPROOF_BIOMETRIC_VISION_CLI")
        image_path = os.environ.get("FACEPROOF_BIOMETRIC_PARITY_IMAGE")
        yunet_path = os.environ.get("FACEPROOF_YUNET_MODEL")
        sface_path = os.environ.get("FACEPROOF_SFACE_MODEL")

        missing = [
            name
            for name, value in {
                "FACEPROOF_BIOMETRIC_VISION_CLI": cli,
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
            check=True,
            capture_output=True,
            text=True,
        )
        cpp = json.loads(completed.stdout)

        image = cv2.imread(image_path, cv2.IMREAD_COLOR)
        self.assertIsNotNone(image)

        detector = YuNetDetector(yunet_path)
        detected = detector.detect_primary(image)
        self.assertIsNotNone(detected)

        python_bbox = detected.bbox
        cpp_face = cpp["face"]
        cpp_bbox = (
            int(cpp_face["x"]),
            int(cpp_face["y"]),
            int(cpp_face["width"]),
            int(cpp_face["height"]),
        )
        for python_value, cpp_value in zip(python_bbox, cpp_bbox, strict=True):
            self.assertLessEqual(abs(python_value - cpp_value), 1)

        self.assertAlmostEqual(
            float(detected.confidence),
            float(cpp_face["confidence"]),
            places=5,
        )

        face_crop = crop_face(image, detected.bbox)
        python_sharpness = sharpness_score(face_crop)
        python_brightness, python_brightness_value = brightness_score(face_crop)
        python_face_size = face_size_score(image, detected.bbox)
        python_quality = clamp01(
            0.45 * python_sharpness
            + 0.30 * python_brightness
            + 0.25 * python_face_size
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
            python_brightness_value,
            float(cpp_quality["brightnessValue"]),
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

        encoder = SFaceEncoder(sface_path)
        python_embedding = encoder.encode(image, detected).astype(np.float64)
        cpp_embedding = np.asarray(cpp["embedding"], dtype=np.float64)

        self.assertEqual(python_embedding.shape, cpp_embedding.shape)
        cosine = float(
            np.dot(python_embedding, cpp_embedding)
            / (np.linalg.norm(python_embedding) * np.linalg.norm(cpp_embedding))
        )
        max_delta = float(np.max(np.abs(python_embedding - cpp_embedding)))

        self.assertTrue(math.isfinite(cosine))
        self.assertGreaterEqual(cosine, 0.99999)
        self.assertLessEqual(max_delta, 5e-4)


if __name__ == "__main__":
    unittest.main()

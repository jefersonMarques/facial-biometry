from __future__ import annotations

import json
import os
from pathlib import Path
import subprocess
import unittest

import cv2

from biometric_engine.models import MiniFASNetV2, YuNetDetector


class BiometricPADParityTest(unittest.TestCase):
    def test_cpp_minifasnet_matches_python(self) -> None:
        cli = os.environ.get("FACEPROOF_BIOMETRIC_PAD_CLI")
        image_path = os.environ.get("FACEPROOF_BIOMETRIC_PARITY_IMAGE")
        yunet_path = os.environ.get("FACEPROOF_YUNET_MODEL")
        sface_path = os.environ.get("FACEPROOF_SFACE_MODEL")
        minifas_path = os.environ.get("FACEPROOF_MINIFAS_MODEL")

        missing = [
            name
            for name, value in {
                "FACEPROOF_BIOMETRIC_PAD_CLI": cli,
                "FACEPROOF_BIOMETRIC_PARITY_IMAGE": image_path,
                "FACEPROOF_YUNET_MODEL": yunet_path,
                "FACEPROOF_SFACE_MODEL": sface_path,
                "FACEPROOF_MINIFAS_MODEL": minifas_path,
            }.items()
            if not value
        ]
        if missing:
            self.skipTest("missing parity environment: " + ", ".join(missing))

        completed = subprocess.run(
            [
                cli,
                "-",
                yunet_path,
                sface_path,
                minifas_path,
            ],
            input=Path(image_path).read_bytes(),
            check=False,
            capture_output=True,
        )
        self.assertEqual(
            completed.returncode,
            0,
            msg=(
                f"C++ PAD probe failed with {completed.returncode}: "
                f"stdout={completed.stdout!r} stderr={completed.stderr!r}"
            ),
        )
        cpp = json.loads(completed.stdout.decode("utf-8"))

        image = cv2.imread(image_path, cv2.IMREAD_COLOR)
        self.assertIsNotNone(image)

        detector = YuNetDetector(yunet_path)
        detected = detector.detect_primary(image)
        self.assertIsNotNone(detected)

        pad = MiniFASNetV2(minifas_path)
        python_probability = pad.predict_real_probability(
            image,
            detected,
        )

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

        self.assertEqual(int(cpp["inputWidth"]), pad._input_width)
        self.assertEqual(int(cpp["inputHeight"]), pad._input_height)
        self.assertGreaterEqual(int(cpp["classCount"]), 2)
        self.assertAlmostEqual(
            float(cpp["realProbability"]),
            float(python_probability),
            places=6,
        )


if __name__ == "__main__":
    unittest.main()

from __future__ import annotations

import unittest

import cv2
import numpy as np

from biometric_engine.image_utils import decode_image_bytes


class BinaryImageDecodeTest(unittest.TestCase):
    def test_decodes_jpeg_bytes_without_base64(self) -> None:
        image = np.full((12, 16, 3), 127, dtype=np.uint8)
        ok, encoded = cv2.imencode(".jpg", image)
        self.assertTrue(ok)

        decoded = decode_image_bytes(encoded.tobytes())

        self.assertEqual(decoded.shape, image.shape)


if __name__ == "__main__":
    unittest.main()

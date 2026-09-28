import unittest

import numpy as np

from biometric_engine.analyzer import _prepare_reference_image


class ReferenceImagePreparationTest(unittest.TestCase):
    def test_upscales_small_vio_portrait_without_changing_aspect_ratio_materially(self) -> None:
        image = np.zeros((61, 45, 3), dtype=np.uint8)

        prepared = _prepare_reference_image(image)

        self.assertEqual(prepared.shape[1], 240)
        self.assertEqual(prepared.shape[0], 325)

    def test_keeps_regular_camera_image_unchanged(self) -> None:
        image = np.zeros((480, 640, 3), dtype=np.uint8)

        prepared = _prepare_reference_image(image)

        self.assertIs(prepared, image)


if __name__ == "__main__":
    unittest.main()

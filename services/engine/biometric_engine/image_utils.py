from __future__ import annotations

import base64

import cv2
import numpy as np


def decode_data_url(value: str) -> np.ndarray:
    encoded = value.split(",", 1)[1] if "," in value else value
    binary = base64.b64decode(encoded, validate=True)
    buffer = np.frombuffer(binary, dtype=np.uint8)
    image = cv2.imdecode(buffer, cv2.IMREAD_COLOR)
    if image is None:
        raise ValueError("invalid image payload")
    return image


def crop_face(image: np.ndarray, bbox: tuple[int, int, int, int], margin: float = 0.08) -> np.ndarray:
    x, y, width, height = bbox
    image_height, image_width = image.shape[:2]
    margin_x = int(width * margin)
    margin_y = int(height * margin)
    x1 = max(0, x - margin_x)
    y1 = max(0, y - margin_y)
    x2 = min(image_width, x + width + margin_x)
    y2 = min(image_height, y + height + margin_y)
    return image[y1:y2, x1:x2]

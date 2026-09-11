from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Optional

import cv2
import numpy as np


@dataclass(frozen=True)
class DetectedFace:
    raw: np.ndarray
    bbox: tuple[int, int, int, int]
    confidence: float


class YuNetDetector:
    def __init__(self, model_path: str, score_threshold: float = 0.82) -> None:
        if not Path(model_path).is_file():
            raise FileNotFoundError(f"YuNet model not found: {model_path}")

        self._detector = cv2.FaceDetectorYN.create(
            model=model_path,
            config="",
            input_size=(320, 320),
            score_threshold=score_threshold,
            nms_threshold=0.3,
            top_k=5000,
        )

    def detect_primary(self, image: np.ndarray) -> Optional[DetectedFace]:
        height, width = image.shape[:2]
        self._detector.setInputSize((width, height))
        _, faces = self._detector.detect(image)
        if faces is None or len(faces) == 0:
            return None

        ranked = sorted(
            faces,
            key=lambda face: float(face[2] * face[3]) * max(float(face[-1]), 0.01),
            reverse=True,
        )
        face = ranked[0].astype(np.float32)
        x, y, width_box, height_box = [int(round(value)) for value in face[:4]]
        x = max(0, x)
        y = max(0, y)
        width_box = max(1, min(width - x, width_box))
        height_box = max(1, min(height - y, height_box))

        return DetectedFace(
            raw=face,
            bbox=(x, y, width_box, height_box),
            confidence=float(face[-1]),
        )


class SFaceEncoder:
    model_name = "opencv-sface-2021dec"

    def __init__(self, model_path: str) -> None:
        if not Path(model_path).is_file():
            raise FileNotFoundError(f"SFace model not found: {model_path}")
        self._recognizer = cv2.FaceRecognizerSF.create(model_path, "")

    def encode(self, image: np.ndarray, face: DetectedFace) -> np.ndarray:
        aligned = self._recognizer.alignCrop(image, face.raw)
        feature = self._recognizer.feature(aligned)
        embedding = feature.reshape(-1).astype(np.float32)
        norm = float(np.linalg.norm(embedding))
        if norm <= 1e-8:
            raise ValueError("SFace returned a zero embedding")
        return embedding / norm


class MiniFASNetV2:
    def __init__(self, model_path: str, scale: float = 2.7) -> None:
        try:
            import onnxruntime as ort
        except ImportError as error:
            raise RuntimeError("onnxruntime is required for MiniFASNet") from error

        if not Path(model_path).is_file():
            raise FileNotFoundError(f"MiniFASNet model not found: {model_path}")

        available = set(ort.get_available_providers())
        providers = [name for name in ["CUDAExecutionProvider", "CPUExecutionProvider"] if name in available]
        if not providers:
            providers = ["CPUExecutionProvider"]

        self._session = ort.InferenceSession(model_path, providers=providers)
        self._scale = scale
        input_config = self._session.get_inputs()[0]
        self._input_name = input_config.name
        self._input_height = int(input_config.shape[2])
        self._input_width = int(input_config.shape[3])
        self._output_name = self._session.get_outputs()[0].name

    def predict_real_probability(self, image: np.ndarray, face: DetectedFace) -> float:
        crop = self._crop_scaled(image, face.bbox)
        resized = cv2.resize(crop, (self._input_width, self._input_height))
        tensor = resized.astype(np.float32).transpose(2, 0, 1)[None, ...]
        logits = self._session.run([self._output_name], {self._input_name: tensor})[0]
        probabilities = self._softmax(logits)
        if probabilities.shape[1] < 2:
            raise ValueError("MiniFASNet output must contain at least two classes")
        return float(probabilities[0, 1])

    def _crop_scaled(self, image: np.ndarray, bbox: tuple[int, int, int, int]) -> np.ndarray:
        source_height, source_width = image.shape[:2]
        x, y, width, height = bbox
        scale = min(
            (source_height - 1) / max(height, 1),
            (source_width - 1) / max(width, 1),
            self._scale,
        )

        new_width = width * scale
        new_height = height * scale
        center_x = x + width / 2
        center_y = y + height / 2

        x1 = max(0, int(center_x - new_width / 2))
        y1 = max(0, int(center_y - new_height / 2))
        x2 = min(source_width - 1, int(center_x + new_width / 2))
        y2 = min(source_height - 1, int(center_y + new_height / 2))
        return image[y1 : y2 + 1, x1 : x2 + 1]

    @staticmethod
    def _softmax(values: np.ndarray) -> np.ndarray:
        shifted = values - np.max(values, axis=1, keepdims=True)
        exponentials = np.exp(shifted)
        return exponentials / exponentials.sum(axis=1, keepdims=True)

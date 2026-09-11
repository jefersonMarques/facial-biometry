from __future__ import annotations

from dataclasses import dataclass
from typing import Any

import cv2
import numpy as np

from .image_utils import crop_face, decode_data_url
from .metrics import (
    brightness_score,
    clamp01,
    face_size_score,
    illumination_correlation,
    robust_mean,
    sharpness_score,
    temporal_motion_score,
)
from .models import MiniFASNetV2, SFaceEncoder, YuNetDetector


@dataclass
class FrameAnalysis:
    image: np.ndarray
    face_raw: Any
    bbox: tuple[int, int, int, int]
    sharpness: float
    brightness_quality: float
    brightness_value: float
    face_size: float
    quality: float
    passive_probability: float | None
    challenge_index: int


class BiometricAnalyzer:
    def __init__(
        self,
        yunet_model_path: str,
        sface_model_path: str,
        minifasnet_model_path: str,
    ) -> None:
        self._detector = YuNetDetector(yunet_model_path)
        self._encoder = SFaceEncoder(sface_model_path)
        self._passive_pad: MiniFASNetV2 | None = None
        self._passive_pad_error: str | None = None

        try:
            self._passive_pad = MiniFASNetV2(minifasnet_model_path)
        except Exception as error:
            self._passive_pad_error = str(error)

    def analyze(self, payload: dict[str, Any]) -> dict[str, Any]:
        frames_payload = payload.get("frames")
        illumination_pattern = payload.get("illuminationPattern")
        if not isinstance(frames_payload, list) or not 8 <= len(frames_payload) <= 32:
            raise ValueError("frames must contain between 8 and 32 items")
        if not isinstance(illumination_pattern, list) or len(illumination_pattern) < 4:
            raise ValueError("illuminationPattern is invalid")

        frame_analyses: list[FrameAnalysis] = []
        face_crops: list[np.ndarray] = []
        normalized_centers: list[tuple[float, float, float]] = []
        diagnostics: list[str] = []

        for frame_payload in frames_payload:
            image = decode_data_url(str(frame_payload.get("imageBase64", "")))
            detected = self._detector.detect_primary(image)
            if detected is None:
                continue

            face_crop = crop_face(image, detected.bbox)
            if face_crop.size == 0:
                continue

            sharpness = sharpness_score(face_crop)
            brightness_quality, brightness_value = brightness_score(face_crop)
            size_score = face_size_score(image, detected.bbox)
            quality = clamp01(0.45 * sharpness + 0.30 * brightness_quality + 0.25 * size_score)

            passive_probability: float | None = None
            if self._passive_pad is not None:
                try:
                    passive_probability = self._passive_pad.predict_real_probability(image, detected)
                except Exception as error:
                    diagnostics.append(f"passive PAD inference failed: {error}")

            image_height, image_width = image.shape[:2]
            x, y, width, height = detected.bbox
            center_x = (x + width / 2) / max(image_width, 1)
            center_y = (y + height / 2) / max(image_height, 1)
            normalized_scale = np.sqrt((width * height) / max(image_width * image_height, 1))

            face_crops.append(face_crop)
            normalized_centers.append((float(center_x), float(center_y), float(normalized_scale)))
            frame_analyses.append(
                FrameAnalysis(
                    image=image,
                    face_raw=detected,
                    bbox=detected.bbox,
                    sharpness=sharpness,
                    brightness_quality=brightness_quality,
                    brightness_value=brightness_value,
                    face_size=size_score,
                    quality=quality,
                    passive_probability=passive_probability,
                    challenge_index=int(frame_payload.get("challengeIndex", -1)),
                )
            )

        if not frame_analyses:
            raise ValueError("no face detected in capture")

        face_presence = len(frame_analyses) / len(frames_payload)
        quality_score = robust_mean([frame.quality for frame in frame_analyses])
        sharpness = robust_mean([frame.sharpness for frame in frame_analyses])
        brightness = robust_mean([frame.brightness_quality for frame in frame_analyses])
        size_score = robust_mean([frame.face_size for frame in frame_analyses])

        temporal_score = temporal_motion_score(face_crops, normalized_centers)
        illumination_score, illumination_correlation_value = illumination_correlation(
            [frame.challenge_index for frame in frame_analyses],
            [frame.brightness_value for frame in frame_analyses],
            [float(value) for value in illumination_pattern],
        )

        passive_values = [
            frame.passive_probability
            for frame in frame_analyses
            if frame.passive_probability is not None
        ]
        if passive_values:
            passive_score = robust_mean([float(value) for value in passive_values])
            passive_status = "available"
        else:
            passive_score = 0.5
            passive_status = "unavailable"
            if self._passive_pad_error:
                diagnostics.append(f"passive PAD unavailable: {self._passive_pad_error}")

        quality_gate = clamp01((quality_score - 0.25) / 0.55)
        presence_gate = clamp01((face_presence - 0.55) / 0.45)

        if passive_status == "available":
            liveness_score = (
                0.48 * passive_score
                + 0.16 * temporal_score
                + 0.15 * illumination_score
                + 0.13 * quality_score
                + 0.08 * face_presence
            )
        else:
            liveness_score = (
                0.30 * temporal_score
                + 0.25 * illumination_score
                + 0.25 * quality_score
                + 0.20 * face_presence
            )
            liveness_score = min(liveness_score, 0.72)

        liveness_score = clamp01(liveness_score * (0.65 + 0.35 * quality_gate) * (0.70 + 0.30 * presence_gate))

        best_index = int(np.argmax([frame.quality for frame in frame_analyses]))
        best = frame_analyses[best_index]
        embedding = self._encoder.encode(best.image, best.face_raw)

        if face_presence < 0.75:
            diagnostics.append("face presence is low")
        if quality_score < 0.55:
            diagnostics.append("capture quality is low")
        if temporal_score < 0.25:
            diagnostics.append("temporal motion signal is weak")
        if illumination_correlation_value < 0.10:
            diagnostics.append("illumination response correlation is weak")

        return {
            "livenessScore": round(float(liveness_score), 6),
            "passivePad": {
                "score": round(float(passive_score), 6),
                "status": passive_status,
            },
            "temporalMotion": {
                "score": round(float(temporal_score), 6),
                "status": "available",
            },
            "illumination": {
                "score": round(float(illumination_score), 6),
                "status": "available",
            },
            "quality": {
                "score": round(float(quality_score), 6),
                "facePresence": round(float(face_presence), 6),
                "sharpness": round(float(sharpness), 6),
                "brightness": round(float(brightness), 6),
                "faceSize": round(float(size_score), 6),
                "detectedFrames": len(frame_analyses),
                "processedFrames": len(frames_payload),
            },
            "embedding": [round(float(value), 8) for value in embedding.tolist()],
            "embeddingModel": self._encoder.model_name,
            "bestFrameIndex": best_index,
            "diagnostics": diagnostics,
        }

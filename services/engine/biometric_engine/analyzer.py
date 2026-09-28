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
    facial_illumination_value,
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
        self._guide_detector = YuNetDetector(yunet_model_path)
        self._encoder = SFaceEncoder(sface_model_path)
        self._passive_pad: MiniFASNetV2 | None = None
        self._passive_pad_error: str | None = None

        try:
            self._passive_pad = MiniFASNetV2(minifasnet_model_path)
        except Exception as error:
            self._passive_pad_error = str(error)

    def extract_reference(self, payload: dict[str, Any]) -> dict[str, Any]:
        image_base64 = payload.get("imageBase64")
        if not isinstance(image_base64, str) or not image_base64.strip():
            raise ValueError("imageBase64 is required")

        image = _prepare_reference_image(decode_data_url(image_base64))
        detected = self._detector.detect_primary(image)
        if detected is None:
            raise ValueError("no face detected in reference image")

        face_crop = crop_face(image, detected.bbox)
        if face_crop.size == 0:
            raise ValueError("reference face crop is empty")

        sharpness = sharpness_score(face_crop)
        brightness, _ = brightness_score(face_crop)
        size_score = face_size_score(image, detected.bbox)
        quality = clamp01(0.45 * sharpness + 0.30 * brightness + 0.25 * size_score)

        reference_images = [
            image,
            _normalize_reference_lighting(image, clip_limit=1.35),
            _normalize_reference_lighting(image, clip_limit=1.70),
        ]
        reference_embeddings = [
            self._encoder.encode(reference_image, detected)
            for reference_image in reference_images
        ]
        embedding = np.mean(np.vstack(reference_embeddings), axis=0).astype(np.float32)
        embedding_norm = float(np.linalg.norm(embedding))
        if embedding_norm <= 1e-8:
            raise ValueError("combined reference embedding is zero")
        embedding = embedding / embedding_norm

        return {
            "embedding": [round(float(value), 8) for value in embedding.tolist()],
            "embeddings": [
                [round(float(value), 8) for value in item.tolist()]
                for item in reference_embeddings
            ],
            "embeddingModel": self._encoder.model_name,
            "quality": {
                "score": round(float(quality), 6),
                "facePresence": 1.0,
                "sharpness": round(float(sharpness), 6),
                "brightness": round(float(brightness), 6),
                "faceSize": round(float(size_score), 6),
                "detectedFrames": 1,
                "processedFrames": 1,
            },
        }


    def guide(self, payload: dict[str, Any]) -> dict[str, Any]:
        image_base64 = payload.get("imageBase64")
        if not isinstance(image_base64, str) or not image_base64.strip():
            raise ValueError("imageBase64 is required")

        image = decode_data_url(image_base64)
        detected = self._guide_detector.detect_primary(image)
        if detected is None:
            return {
                "faceDetected": False,
                "confidence": 0.0,
                "centerX": 0.0,
                "centerY": 0.0,
                "widthRatio": 0.0,
                "heightRatio": 0.0,
                "rollDegrees": 0.0,
                "quality": _empty_quality(),
            }

        face_crop = crop_face(image, detected.bbox)
        if face_crop.size == 0:
            return {
                "faceDetected": False,
                "confidence": 0.0,
                "centerX": 0.0,
                "centerY": 0.0,
                "widthRatio": 0.0,
                "heightRatio": 0.0,
                "rollDegrees": 0.0,
                "quality": _empty_quality(),
            }

        image_height, image_width = image.shape[:2]
        x, y, width, height = detected.bbox
        center_x = (x + width / 2) / max(image_width, 1)
        center_y = (y + height / 2) / max(image_height, 1)
        width_ratio = width / max(image_width, 1)
        height_ratio = height / max(image_height, 1)

        sharpness = sharpness_score(face_crop)
        brightness, _ = brightness_score(face_crop)
        size_score = face_size_score(image, detected.bbox)
        quality = clamp01(0.45 * sharpness + 0.30 * brightness + 0.25 * size_score)

        roll_degrees = 0.0
        raw = detected.raw
        if len(raw) >= 8:
            right_eye_x, right_eye_y = float(raw[4]), float(raw[5])
            left_eye_x, left_eye_y = float(raw[6]), float(raw[7])
            roll_degrees = float(np.degrees(np.arctan2(left_eye_y - right_eye_y, left_eye_x - right_eye_x)))

        return {
            "faceDetected": True,
            "confidence": round(float(detected.confidence), 6),
            "centerX": round(float(center_x), 6),
            "centerY": round(float(center_y), 6),
            "widthRatio": round(float(width_ratio), 6),
            "heightRatio": round(float(height_ratio), 6),
            "rollDegrees": round(float(roll_degrees), 3),
            "quality": {
                "score": round(float(quality), 6),
                "facePresence": 1.0,
                "sharpness": round(float(sharpness), 6),
                "brightness": round(float(brightness), 6),
                "faceSize": round(float(size_score), 6),
                "detectedFrames": 1,
                "processedFrames": 1,
            },
        }

    def analyze_identity(self, payload: dict[str, Any]) -> dict[str, Any]:
        frames_payload = payload.get("guidedFrames")
        if not isinstance(frames_payload, list) or not 6 <= len(frames_payload) <= 12:
            raise ValueError("guidedFrames must contain between 6 and 12 items")

        frame_analyses: list[FrameAnalysis] = []
        face_crops: list[np.ndarray] = []
        normalized_centers: list[tuple[float, float, float]] = []
        embeddings: list[tuple[float, str, int, np.ndarray]] = []
        far_scales: list[float] = []
        near_scales: list[float] = []
        center_scores: list[float] = []
        quality_scores: list[float] = []
        passive_values: list[float] = []
        diagnostics: list[str] = []

        far_count = 0
        near_count = 0

        for frame_index, frame_payload in enumerate(frames_payload):
            if not isinstance(frame_payload, dict):
                continue
            phase = str(frame_payload.get("phase", "")).strip().lower()
            if phase not in {"far", "near"}:
                continue

            image = decode_data_url(str(frame_payload.get("imageBase64", "")))
            detected = self._detector.detect_primary(image)
            if detected is None:
                continue

            face_crop = crop_face(image, detected.bbox)
            if face_crop.size == 0:
                continue

            sharpness = sharpness_score(face_crop)
            brightness_quality, _ = brightness_score(face_crop)
            brightness_value = facial_illumination_value(face_crop)
            size_score = face_size_score(image, detected.bbox)
            quality = clamp01(0.45 * sharpness + 0.30 * brightness_quality + 0.25 * size_score)

            passive_probability: float | None = None
            if self._passive_pad is not None:
                try:
                    passive_probability = self._passive_pad.predict_real_probability(image, detected)
                    passive_values.append(float(passive_probability))
                except Exception as error:
                    diagnostics.append(f"passive PAD inference failed: {error}")

            image_height, image_width = image.shape[:2]
            x, y, width, height = detected.bbox
            center_x = (x + width / 2) / max(image_width, 1)
            center_y = (y + height / 2) / max(image_height, 1)
            normalized_scale = np.sqrt((width * height) / max(image_width * image_height, 1))
            height_ratio = height / max(image_height, 1)

            center_distance = float(np.sqrt((center_x - 0.5) ** 2 + (center_y - 0.46) ** 2))
            center_scores.append(clamp01(1.0 - center_distance / 0.24))
            quality_scores.append(float(quality))

            if phase == "far":
                far_scales.append(float(height_ratio))
                far_count += 1
            else:
                near_scales.append(float(height_ratio))
                near_count += 1

            embedding = self._encoder.encode(image, detected)
            embeddings.append((quality, phase, frame_index, embedding))
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
                    challenge_index=-1,
                )
            )

        if far_count < 3 or near_count < 3:
            raise ValueError("identity capture must contain at least three valid far and near frames")
        if not frame_analyses:
            raise ValueError("no face detected in identity capture")

        face_presence = len(frame_analyses) / len(frames_payload)
        quality_score = robust_mean([frame.quality for frame in frame_analyses])
        sharpness = robust_mean([frame.sharpness for frame in frame_analyses])
        brightness = robust_mean([frame.brightness_quality for frame in frame_analyses])
        size_score = robust_mean([frame.face_size for frame in frame_analyses])
        temporal_score = temporal_motion_score(face_crops, normalized_centers)
        guided_score = _guided_capture_score(far_scales, near_scales, center_scores, quality_scores)

        if passive_values:
            passive_score = robust_mean(passive_values)
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
                0.55 * passive_score
                + 0.20 * guided_score
                + 0.10 * temporal_score
                + 0.10 * quality_score
                + 0.05 * face_presence
            )
        else:
            liveness_score = (
                0.36 * guided_score
                + 0.24 * temporal_score
                + 0.22 * quality_score
                + 0.18 * face_presence
            )
            liveness_score = min(liveness_score, 0.72)

        liveness_score = clamp01(
            liveness_score
            * (0.65 + 0.35 * quality_gate)
            * (0.70 + 0.30 * presence_gate)
        )

        near_embeddings = [item for item in embeddings if item[1] == "near"]
        near_embeddings.sort(key=lambda item: item[0], reverse=True)
        selected_near = near_embeddings[:3]
        if len(selected_near) < 3:
            raise ValueError("not enough high-quality near face embeddings")

        selected_embeddings = [embedding for _, _, _, embedding in selected_near]
        combined_embedding = np.mean(np.vstack(selected_embeddings), axis=0).astype(np.float32)
        embedding_norm = float(np.linalg.norm(combined_embedding))
        if embedding_norm <= 1e-8:
            raise ValueError("combined SFace embedding is zero")
        combined_embedding = combined_embedding / embedding_norm

        if quality_score < 0.50:
            diagnostics.append("capture quality is low")
        if face_presence < 0.80:
            diagnostics.append("face presence is low")
        if guided_score < 0.45:
            diagnostics.append("guided far/near capture signal is weak")
        if temporal_score < 0.20:
            diagnostics.append("temporal motion signal is weak")

        best_frame_index = int(selected_near[0][2])
        face_embeddings = [
            {
                "frameIndex": int(frame_index),
                "phase": phase,
                "quality": round(float(quality), 6),
                "embedding": [round(float(value), 8) for value in embedding.tolist()],
            }
            for quality, phase, frame_index, embedding in selected_near
        ]

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
                "score": 0.5,
                "status": "not_used",
            },
            "guidedCapture": {
                "score": round(float(guided_score), 6),
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
            "embedding": [round(float(value), 8) for value in combined_embedding.tolist()],
            "faceEmbeddings": face_embeddings,
            "embeddingModel": self._encoder.model_name,
            "bestFrameIndex": best_frame_index,
            "diagnostics": diagnostics,
        }

    def analyze(self, payload: dict[str, Any]) -> dict[str, Any]:
        frames_payload = payload.get("frames")
        guided_frames_payload = payload.get("guidedFrames")
        illumination_pattern = payload.get("illuminationPattern")
        if not isinstance(frames_payload, list) or not 8 <= len(frames_payload) <= 32:
            raise ValueError("frames must contain between 8 and 32 items")
        if not isinstance(guided_frames_payload, list) or not 4 <= len(guided_frames_payload) <= 12:
            raise ValueError("guidedFrames must contain between 4 and 12 items")
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
            brightness_quality, _ = brightness_score(face_crop)
            brightness_value = facial_illumination_value(face_crop)
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

        guided_embeddings, guided_score, guided_diagnostics = self._analyze_guided_frames(guided_frames_payload)
        diagnostics.extend(guided_diagnostics)

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
                0.45 * passive_score
                + 0.15 * temporal_score
                + 0.14 * illumination_score
                + 0.12 * quality_score
                + 0.08 * face_presence
                + 0.06 * guided_score
            )
        else:
            liveness_score = (
                0.28 * temporal_score
                + 0.24 * illumination_score
                + 0.23 * quality_score
                + 0.17 * face_presence
                + 0.08 * guided_score
            )
            liveness_score = min(liveness_score, 0.72)

        liveness_score = clamp01(liveness_score * (0.65 + 0.35 * quality_gate) * (0.70 + 0.30 * presence_gate))

        best_index = int(np.argmax([frame.quality for frame in frame_analyses]))
        best = frame_analyses[best_index]

        challenge_candidates = sorted(
            range(len(frame_analyses)),
            key=lambda index: frame_analyses[index].quality,
            reverse=True,
        )[:3]
        embeddings = [
            self._encoder.encode(frame_analyses[index].image, frame_analyses[index].face_raw)
            for index in challenge_candidates
        ]
        embeddings.extend(guided_embeddings)
        embedding = np.mean(np.vstack(embeddings), axis=0).astype(np.float32)
        embedding_norm = float(np.linalg.norm(embedding))
        if embedding_norm <= 1e-8:
            raise ValueError("combined SFace embedding is zero")
        embedding = embedding / embedding_norm

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
            "guidedCapture": {
                "score": round(float(guided_score), 6),
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


    def _analyze_guided_frames(
        self,
        frames_payload: list[dict[str, Any]],
    ) -> tuple[list[np.ndarray], float, list[str]]:
        candidates: list[tuple[float, np.ndarray]] = []
        far_scales: list[float] = []
        near_scales: list[float] = []
        center_scores: list[float] = []
        quality_scores: list[float] = []
        diagnostics: list[str] = []

        for frame_payload in frames_payload:
            if not isinstance(frame_payload, dict):
                continue
            phase = str(frame_payload.get("phase", "")).strip().lower()
            if phase not in {"far", "near"}:
                continue

            image = decode_data_url(str(frame_payload.get("imageBase64", "")))
            detected = self._detector.detect_primary(image)
            if detected is None:
                continue

            face_crop = crop_face(image, detected.bbox)
            if face_crop.size == 0:
                continue

            image_height, image_width = image.shape[:2]
            x, y, width, height = detected.bbox
            center_x = (x + width / 2) / max(image_width, 1)
            center_y = (y + height / 2) / max(image_height, 1)
            height_ratio = height / max(image_height, 1)

            sharpness = sharpness_score(face_crop)
            brightness, _ = brightness_score(face_crop)
            size_score = face_size_score(image, detected.bbox)
            quality = clamp01(0.45 * sharpness + 0.30 * brightness + 0.25 * size_score)

            center_distance = float(np.sqrt((center_x - 0.5) ** 2 + (center_y - 0.46) ** 2))
            center_score = clamp01(1.0 - center_distance / 0.24)
            center_scores.append(center_score)
            quality_scores.append(quality)

            if phase == "far":
                far_scales.append(float(height_ratio))
            else:
                near_scales.append(float(height_ratio))

            if quality >= 0.30:
                candidates.append((quality, self._encoder.encode(image, detected)))

        guided_score = _guided_capture_score(far_scales, near_scales, center_scores, quality_scores)
        if len(far_scales) < 2 or len(near_scales) < 2:
            diagnostics.append("guided capture phase coverage is low")
        elif float(np.median(near_scales)) - float(np.median(far_scales)) < 0.08:
            diagnostics.append("guided near/far scale transition is weak")
        if guided_score < 0.45:
            diagnostics.append("guided capture signal is weak")

        candidates.sort(key=lambda item: item[0], reverse=True)
        embeddings = [embedding for _, embedding in candidates[:6]]
        return embeddings, guided_score, diagnostics




def _empty_quality() -> dict[str, Any]:
    return {
        "score": 0.0,
        "facePresence": 0.0,
        "sharpness": 0.0,
        "brightness": 0.0,
        "faceSize": 0.0,
        "detectedFrames": 0,
        "processedFrames": 1,
    }


def _guided_capture_score(
    far_scales: list[float],
    near_scales: list[float],
    center_scores: list[float],
    quality_scores: list[float],
) -> float:
    if not far_scales or not near_scales:
        return 0.0

    far_median = float(np.median(far_scales))
    near_median = float(np.median(near_scales))
    scale_delta = near_median - far_median
    transition_score = clamp01((scale_delta - 0.05) / 0.15)
    coverage_score = clamp01(min(len(far_scales), len(near_scales)) / 3.0)
    centering_score = robust_mean(center_scores) if center_scores else 0.0
    quality_score = robust_mean(quality_scores) if quality_scores else 0.0

    return clamp01(
        0.40 * transition_score
        + 0.25 * coverage_score
        + 0.20 * centering_score
        + 0.15 * quality_score
    )


def _normalize_reference_lighting(image: np.ndarray, clip_limit: float) -> np.ndarray:
    lab = cv2.cvtColor(image, cv2.COLOR_BGR2LAB)
    luminance, channel_a, channel_b = cv2.split(lab)
    clahe = cv2.createCLAHE(clipLimit=clip_limit, tileGridSize=(8, 8))
    normalized_luminance = clahe.apply(luminance)
    blended_luminance = cv2.addWeighted(luminance, 0.68, normalized_luminance, 0.32, 0)
    normalized = cv2.merge((blended_luminance, channel_a, channel_b))
    return cv2.cvtColor(normalized, cv2.COLOR_LAB2BGR)


def _prepare_reference_image(
    image: np.ndarray,
    minimum_short_side: int = 480,
    maximum_long_side: int = 1280,
) -> np.ndarray:
    height, width = image.shape[:2]
    if height <= 0 or width <= 0:
        raise ValueError("reference image has invalid dimensions")

    short_side = min(height, width)
    long_side = max(height, width)
    if short_side >= minimum_short_side or long_side >= maximum_long_side:
        return image

    scale = minimum_short_side / short_side
    if long_side * scale > maximum_long_side:
        scale = maximum_long_side / long_side

    resized_width = max(1, int(round(width * scale)))
    resized_height = max(1, int(round(height * scale)))
    return cv2.resize(image, (resized_width, resized_height), interpolation=cv2.INTER_CUBIC)

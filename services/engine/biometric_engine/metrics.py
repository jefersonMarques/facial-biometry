from __future__ import annotations

import math
from collections import defaultdict
from typing import Iterable

import cv2
import numpy as np


def clamp01(value: float) -> float:
    return max(0.0, min(1.0, float(value)))


def sharpness_score(image: np.ndarray) -> float:
    gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)
    variance = float(cv2.Laplacian(gray, cv2.CV_64F).var())
    return clamp01((variance - 30.0) / 220.0)


def brightness_score(image: np.ndarray) -> tuple[float, float]:
    gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)
    normalized = float(gray.mean()) / 255.0
    score = 1.0 - min(abs(normalized - 0.52) / 0.52, 1.0)
    return clamp01(score), normalized


def face_size_score(image: np.ndarray, bbox: tuple[int, int, int, int]) -> float:
    _, _, width, height = bbox
    image_height, image_width = image.shape[:2]
    area_ratio = (width * height) / float(max(image_width * image_height, 1))
    if area_ratio < 0.07:
        return clamp01(area_ratio / 0.07)
    if area_ratio > 0.62:
        return clamp01(1.0 - ((area_ratio - 0.62) / 0.30))
    return 1.0


def temporal_motion_score(face_crops: list[np.ndarray], centers: list[tuple[float, float, float]]) -> float:
    if len(face_crops) < 3:
        return 0.0

    pixel_differences: list[float] = []
    previous: np.ndarray | None = None
    for crop in face_crops:
        gray = cv2.cvtColor(crop, cv2.COLOR_BGR2GRAY)
        normalized = cv2.resize(gray, (96, 96)).astype(np.float32) / 255.0
        if previous is not None:
            pixel_differences.append(float(np.mean(np.abs(normalized - previous))))
        previous = normalized

    position_differences: list[float] = []
    for left, right in zip(centers, centers[1:]):
        dx = right[0] - left[0]
        dy = right[1] - left[1]
        ds = right[2] - left[2]
        position_differences.append(math.sqrt(dx * dx + dy * dy + ds * ds))

    pixel_median = float(np.median(pixel_differences)) if pixel_differences else 0.0
    position_median = float(np.median(position_differences)) if position_differences else 0.0

    pixel_score = clamp01((pixel_median - 0.006) / 0.055)
    position_score = clamp01(position_median / 0.045)

    too_dynamic_penalty = clamp01((pixel_median - 0.16) / 0.20)
    return clamp01((0.65 * pixel_score + 0.35 * position_score) * (1.0 - 0.45 * too_dynamic_penalty))


def illumination_correlation(
    challenge_indices: Iterable[int],
    observed_brightness: Iterable[float],
    illumination_pattern: list[float],
) -> tuple[float, float]:
    grouped: dict[int, list[float]] = defaultdict(list)
    for challenge_index, brightness in zip(challenge_indices, observed_brightness):
        if 0 <= challenge_index < len(illumination_pattern):
            grouped[challenge_index].append(brightness)

    valid_indices = sorted(index for index, values in grouped.items() if values)
    if len(valid_indices) < 4:
        return 0.5, 0.0

    expected = np.asarray([illumination_pattern[index] for index in valid_indices], dtype=np.float64)
    observed = np.asarray([np.median(grouped[index]) for index in valid_indices], dtype=np.float64)

    expected_std = float(expected.std())
    observed_std = float(observed.std())
    if expected_std < 1e-6 or observed_std < 0.004:
        return 0.5, 0.0

    correlation = float(np.corrcoef(expected, observed)[0, 1])
    if not np.isfinite(correlation):
        return 0.5, 0.0

    score = clamp01((correlation + 0.15) / 1.15)
    return score, correlation


def robust_mean(values: list[float]) -> float:
    if not values:
        return 0.0
    values_array = np.asarray(values, dtype=np.float64)
    if len(values_array) < 5:
        return float(values_array.mean())
    lower, upper = np.percentile(values_array, [15, 85])
    trimmed = values_array[(values_array >= lower) & (values_array <= upper)]
    return float(trimmed.mean()) if len(trimmed) else float(values_array.mean())

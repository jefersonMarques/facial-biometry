from __future__ import annotations

import json
import math
import os
import subprocess
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import numpy as np


@dataclass(frozen=True)
class NativeShadowConfig:
    enabled: bool
    vision_cli: str
    pad_cli: str
    reference_cli: str
    yunet_model: str
    sface_model: str
    minifas_model: str
    timeout_seconds: float


class NativeBiometricShadow:
    def __init__(self, config: NativeShadowConfig) -> None:
        self._config = config

    @property
    def enabled(self) -> bool:
        return self._config.enabled

    @classmethod
    def from_environment(
        cls,
        *,
        yunet_model: str,
        sface_model: str,
        minifas_model: str,
    ) -> "NativeBiometricShadow":
        enabled = _env_bool("FACEPROOF_NATIVE_SHADOW_ENABLED", False)
        vision_cli = os.getenv("FACEPROOF_BIOMETRIC_VISION_CLI", "").strip()
        pad_cli = os.getenv("FACEPROOF_BIOMETRIC_PAD_CLI", "").strip()
        reference_cli = os.getenv("FACEPROOF_BIOMETRIC_REFERENCE_CLI", "").strip()
        timeout_seconds = float(
            os.getenv("FACEPROOF_NATIVE_SHADOW_TIMEOUT_SECONDS", "5.0")
        )

        return cls(
            NativeShadowConfig(
                enabled=enabled,
                vision_cli=vision_cli,
                pad_cli=pad_cli,
                reference_cli=reference_cli,
                yunet_model=yunet_model,
                sface_model=sface_model,
                minifas_model=minifas_model,
                timeout_seconds=max(0.5, min(timeout_seconds, 30.0)),
            )
        )

    def frame(self, image_bytes: bytes) -> dict[str, Any] | None:
        if not self.enabled:
            return None

        if self._config.pad_cli and Path(self._config.pad_cli).is_file():
            command = [
                self._config.pad_cli,
                "-",
                self._config.yunet_model,
                self._config.sface_model,
                self._config.minifas_model,
            ]
            return self._run_json(command, image_bytes)

        if self._config.vision_cli and Path(self._config.vision_cli).is_file():
            command = [
                self._config.vision_cli,
                "-",
                self._config.yunet_model,
                self._config.sface_model,
            ]
            result = self._run_json(command, image_bytes)
            result["padStatus"] = "not_compared"
            return result

        return {
            "error": (
                "native shadow requires FACEPROOF_BIOMETRIC_PAD_CLI "
                "or FACEPROOF_BIOMETRIC_VISION_CLI"
            )
        }

    def reference(self, image_bytes: bytes) -> dict[str, Any] | None:
        if not self.enabled:
            return None
        if not self._config.reference_cli:
            return {"error": "FACEPROOF_BIOMETRIC_REFERENCE_CLI is not configured"}
        if not Path(self._config.reference_cli).is_file():
            return {"error": "native reference CLI does not exist"}

        command = [
            self._config.reference_cli,
            "-",
            self._config.yunet_model,
            self._config.sface_model,
        ]
        return self._run_json(command, image_bytes)

    def _run_json(
        self,
        command: list[str],
        image_bytes: bytes,
    ) -> dict[str, Any]:
        try:
            completed = subprocess.run(
                command,
                input=image_bytes,
                capture_output=True,
                check=False,
                timeout=self._config.timeout_seconds,
            )
        except subprocess.TimeoutExpired:
            return {"error": "native shadow timed out"}
        except OSError as error:
            return {"error": f"native shadow could not start: {error}"}

        if completed.returncode != 0:
            stderr = completed.stderr.decode("utf-8", errors="replace").strip()
            return {
                "error": (
                    f"native shadow exited with {completed.returncode}"
                    + (f": {stderr}" if stderr else "")
                )
            }

        try:
            decoded = json.loads(completed.stdout.decode("utf-8"))
        except (UnicodeDecodeError, json.JSONDecodeError) as error:
            return {"error": f"native shadow returned invalid JSON: {error}"}

        if not isinstance(decoded, dict):
            return {"error": "native shadow returned a non-object payload"}
        return decoded


def compare_reference_shadow(
    python_result: dict[str, Any],
    native_result: dict[str, Any] | None,
) -> dict[str, Any] | None:
    if native_result is None:
        return None
    if "error" in native_result:
        return {
            "status": "partial",
            "error": str(native_result["error"]),
        }

    python_embedding = _vector(python_result.get("embedding"))
    native_embedding = _vector(native_result.get("embedding"))
    if python_embedding is None or native_embedding is None:
        return {
            "status": "partial",
            "error": "reference embedding is missing",
        }

    cosine, max_delta = _embedding_delta(
        python_embedding,
        native_embedding,
    )

    quality_delta = _quality_delta(
        python_result.get("quality"),
        native_result.get("quality"),
    )

    status = "ok"
    if cosine < 0.99999 or max_delta > 5e-4 or quality_delta > 1e-5:
        status = "drift"

    return {
        "status": status,
        "embeddingCosine": round(cosine, 8),
        "embeddingMaxDelta": round(max_delta, 8),
        "qualityMaxDelta": round(quality_delta, 8),
    }


def compare_identity_shadow(
    *,
    python_frames: dict[int, dict[str, Any]],
    native_frames: dict[int, dict[str, Any]],
    selected_frame_indices: list[int],
    python_combined_embedding: np.ndarray,
) -> dict[str, Any]:
    errors = [
        str(value.get("error"))
        for value in native_frames.values()
        if isinstance(value, dict) and value.get("error")
    ]
    comparable_indices = sorted(
        index
        for index in python_frames
        if index in native_frames and "error" not in native_frames[index]
    )

    bbox_max_delta = 0.0
    confidence_max_delta = 0.0
    quality_max_delta = 0.0
    passive_pad_max_delta = 0.0
    passive_comparisons = 0

    for index in comparable_indices:
        python_frame = python_frames[index]
        native_frame = native_frames[index]

        python_bbox = python_frame.get("bbox")
        native_face = native_frame.get("face")
        if (
            isinstance(python_bbox, tuple)
            and len(python_bbox) == 4
            and isinstance(native_face, dict)
        ):
            native_bbox = (
                int(native_face.get("x", 0)),
                int(native_face.get("y", 0)),
                int(native_face.get("width", 0)),
                int(native_face.get("height", 0)),
            )
            bbox_max_delta = max(
                bbox_max_delta,
                max(
                    abs(float(left) - float(right))
                    for left, right in zip(
                        python_bbox,
                        native_bbox,
                        strict=True,
                    )
                ),
            )
            confidence_max_delta = max(
                confidence_max_delta,
                abs(
                    float(python_frame.get("confidence", 0.0))
                    - float(native_face.get("confidence", 0.0))
                ),
            )

        quality_max_delta = max(
            quality_max_delta,
            _quality_delta(
                python_frame.get("quality"),
                native_frame.get("quality"),
            ),
        )

        python_pad = python_frame.get("passivePad")
        native_pad = native_frame.get("realProbability")
        if python_pad is not None and native_pad is not None:
            passive_pad_max_delta = max(
                passive_pad_max_delta,
                abs(float(python_pad) - float(native_pad)),
            )
            passive_comparisons += 1

    embedding_cosines: list[float] = []
    embedding_max_deltas: list[float] = []
    native_selected: list[np.ndarray] = []

    for index in selected_frame_indices:
        python_frame = python_frames.get(index)
        native_frame = native_frames.get(index)
        if not python_frame or not native_frame or "error" in native_frame:
            continue

        python_embedding = _vector(python_frame.get("embedding"))
        native_embedding = _vector(native_frame.get("embedding"))
        if python_embedding is None or native_embedding is None:
            continue

        cosine, max_delta = _embedding_delta(
            python_embedding,
            native_embedding,
        )
        embedding_cosines.append(cosine)
        embedding_max_deltas.append(max_delta)
        native_selected.append(native_embedding)

    combined_cosine = 0.0
    combined_max_delta = 1.0
    if native_selected and len(native_selected) == len(selected_frame_indices):
        native_combined = np.mean(
            np.vstack(native_selected),
            axis=0,
        ).astype(np.float32)
        norm = float(np.linalg.norm(native_combined))
        if norm > 1e-8:
            native_combined = native_combined / norm
            combined_cosine, combined_max_delta = _embedding_delta(
                python_combined_embedding.astype(np.float64),
                native_combined.astype(np.float64),
            )

    status = "ok"
    if errors or len(comparable_indices) != len(python_frames):
        status = "partial"

    drift = (
        bbox_max_delta > 1.0
        or confidence_max_delta > 1e-5
        or quality_max_delta > 1e-5
        or (passive_comparisons > 0 and passive_pad_max_delta > 1e-5)
        or (
            embedding_cosines
            and min(embedding_cosines) < 0.99999
        )
        or (
            embedding_max_deltas
            and max(embedding_max_deltas) > 5e-4
        )
        or (
            native_selected
            and (
                combined_cosine < 0.99999
                or combined_max_delta > 5e-4
            )
        )
    )
    if drift:
        status = "drift"

    result: dict[str, Any] = {
        "status": status,
        "pythonFrames": len(python_frames),
        "nativeFrames": len(comparable_indices),
        "bboxMaxDeltaPx": round(bbox_max_delta, 6),
        "confidenceMaxDelta": round(confidence_max_delta, 8),
        "qualityMaxDelta": round(quality_max_delta, 8),
        "passivePadMaxDelta": round(passive_pad_max_delta, 8),
        "selectedEmbeddingMinCosine": round(
            min(embedding_cosines) if embedding_cosines else 0.0,
            8,
        ),
        "selectedEmbeddingMaxDelta": round(
            max(embedding_max_deltas) if embedding_max_deltas else 0.0,
            8,
        ),
        "combinedEmbeddingCosine": round(combined_cosine, 8),
        "combinedEmbeddingMaxDelta": round(combined_max_delta, 8),
    }
    if errors:
        result["errors"] = errors[:3]
    return result


def _env_bool(name: str, default: bool) -> bool:
    value = os.getenv(name)
    if value is None:
        return default
    return value.strip().lower() in {"1", "true", "yes", "on"}


def _vector(value: Any) -> np.ndarray | None:
    if not isinstance(value, (list, tuple)):
        return None
    try:
        vector = np.asarray(value, dtype=np.float64).reshape(-1)
    except (TypeError, ValueError):
        return None
    if vector.size == 0 or not np.all(np.isfinite(vector)):
        return None
    return vector


def _embedding_delta(
    left: np.ndarray,
    right: np.ndarray,
) -> tuple[float, float]:
    if left.shape != right.shape or left.size == 0:
        return 0.0, math.inf

    left_norm = float(np.linalg.norm(left))
    right_norm = float(np.linalg.norm(right))
    if left_norm <= 1e-12 or right_norm <= 1e-12:
        return 0.0, math.inf

    cosine = float(np.dot(left, right) / (left_norm * right_norm))
    max_delta = float(np.max(np.abs(left - right)))
    return cosine, max_delta


def _quality_delta(left: Any, right: Any) -> float:
    if not isinstance(left, dict) or not isinstance(right, dict):
        return 0.0

    pairs = [
        ("sharpness", "sharpness"),
        ("brightness", "brightness"),
        ("faceSize", "faceSize"),
        ("score", "score"),
    ]
    deltas = []
    for left_key, right_key in pairs:
        if left_key in left and right_key in right:
            deltas.append(
                abs(float(left[left_key]) - float(right[right_key]))
            )
    return max(deltas) if deltas else 0.0

from __future__ import annotations

import ctypes
import os
import unittest
from pathlib import Path

import numpy as np

from biometric_engine.analyzer import BiometricAnalyzer


EMBEDDING_CAPACITY = 512
MAX_SELECTED_FRAMES = 3


class FrameInput(ctypes.Structure):
    _fields_ = [
        ("data", ctypes.POINTER(ctypes.c_uint8)),
        ("size", ctypes.c_size_t),
        ("phase", ctypes.c_int32),
    ]


class SelectedEmbedding(ctypes.Structure):
    _fields_ = [
        ("frame_index", ctypes.c_int32),
        ("phase", ctypes.c_int32),
        ("quality", ctypes.c_double),
        ("embedding_size", ctypes.c_size_t),
        ("embedding", ctypes.c_float * EMBEDDING_CAPACITY),
    ]


class IdentityResult(ctypes.Structure):
    _fields_ = [
        ("liveness_score", ctypes.c_double),
        ("passive_pad_score", ctypes.c_double),
        ("passive_pad_available", ctypes.c_int32),
        ("temporal_motion_score", ctypes.c_double),
        ("guided_capture_score", ctypes.c_double),
        ("quality_score", ctypes.c_double),
        ("face_presence", ctypes.c_double),
        ("sharpness", ctypes.c_double),
        ("brightness", ctypes.c_double),
        ("face_size", ctypes.c_double),
        ("detected_frames", ctypes.c_int32),
        ("processed_frames", ctypes.c_int32),
        ("best_frame_index", ctypes.c_int32),
        ("embedding_size", ctypes.c_size_t),
        ("embedding", ctypes.c_float * EMBEDDING_CAPACITY),
        ("selected_embedding_count", ctypes.c_size_t),
        ("selected_embeddings", SelectedEmbedding * MAX_SELECTED_FRAMES),
    ]


@unittest.skipUnless(
    os.getenv("FACEPROOF_SECURE_CORE_LIBRARY")
    and os.getenv("FACEPROOF_BIOMETRIC_PARITY_IMAGE"),
    "native Secure Core parity environment is not configured",
)
class SecureCoreIdentityParityTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.library = ctypes.CDLL(os.environ["FACEPROOF_SECURE_CORE_LIBRARY"])
        cls.library.fp_secure_core_create.argtypes = [
            ctypes.c_char_p,
            ctypes.c_char_p,
            ctypes.c_char_p,
        ]
        cls.library.fp_secure_core_create.restype = ctypes.c_void_p
        cls.library.fp_secure_core_destroy.argtypes = [ctypes.c_void_p]
        cls.library.fp_secure_core_destroy.restype = None
        cls.library.fp_secure_core_analyze_identity.argtypes = [
            ctypes.c_void_p,
            ctypes.POINTER(FrameInput),
            ctypes.c_size_t,
            ctypes.POINTER(IdentityResult),
        ]
        cls.library.fp_secure_core_analyze_identity.restype = ctypes.c_int
        cls.library.fp_secure_core_last_error.argtypes = []
        cls.library.fp_secure_core_last_error.restype = ctypes.c_char_p

        cls.yunet = os.environ["FACEPROOF_YUNET_MODEL"]
        cls.sface = os.environ["FACEPROOF_SFACE_MODEL"]
        cls.minifas = os.environ["FACEPROOF_MINIFAS_MODEL"]
        cls.fixture = Path(os.environ["FACEPROOF_BIOMETRIC_PARITY_IMAGE"]).read_bytes()

        cls.core = cls.library.fp_secure_core_create(
            cls.yunet.encode(),
            cls.sface.encode(),
            cls.minifas.encode(),
        )
        if not cls.core:
            error = cls.library.fp_secure_core_last_error()
            raise RuntimeError(error.decode() if error else "Secure Core initialization failed")

        cls.python = BiometricAnalyzer(
            cls.yunet,
            cls.sface,
            cls.minifas,
        )

    @classmethod
    def tearDownClass(cls) -> None:
        if getattr(cls, "core", None):
            cls.library.fp_secure_core_destroy(cls.core)

    def test_identity_pipeline_matches_python_authority(self) -> None:
        phases = ["far", "far", "far", "near", "near", "near"]
        python_result = self.python.analyze_identity(
            {
                "guidedFrames": [
                    {"phase": phase, "imageBytes": self.fixture}
                    for phase in phases
                ]
            }
        )

        buffers = [
            (ctypes.c_uint8 * len(self.fixture)).from_buffer_copy(self.fixture)
            for _ in phases
        ]
        native_frames = (FrameInput * len(phases))()
        for index, phase in enumerate(phases):
            native_frames[index] = FrameInput(
                ctypes.cast(buffers[index], ctypes.POINTER(ctypes.c_uint8)),
                len(self.fixture),
                0 if phase == "far" else 1,
            )

        native_result = IdentityResult()
        code = self.library.fp_secure_core_analyze_identity(
            self.core,
            native_frames,
            len(phases),
            ctypes.byref(native_result),
        )
        if code != 0:
            error = self.library.fp_secure_core_last_error()
            self.fail(
                f"Secure Core identity failed code={code}: "
                f"{error.decode() if error else 'unknown error'}"
            )

        self.assertAlmostEqual(
            native_result.liveness_score,
            python_result["livenessScore"],
            places=5,
        )
        self.assertAlmostEqual(
            native_result.passive_pad_score,
            python_result["passivePad"]["score"],
            places=5,
        )
        self.assertAlmostEqual(
            native_result.temporal_motion_score,
            python_result["temporalMotion"]["score"],
            places=5,
        )
        self.assertAlmostEqual(
            native_result.guided_capture_score,
            python_result["guidedCapture"]["score"],
            places=5,
        )
        self.assertAlmostEqual(
            native_result.quality_score,
            python_result["quality"]["score"],
            places=5,
        )
        self.assertEqual(
            native_result.detected_frames,
            python_result["quality"]["detectedFrames"],
        )
        self.assertEqual(
            native_result.processed_frames,
            python_result["quality"]["processedFrames"],
        )
        self.assertEqual(
            native_result.best_frame_index,
            python_result["bestFrameIndex"],
        )

        dimension = int(native_result.embedding_size)
        self.assertGreater(dimension, 0)
        native_embedding = np.asarray(
            native_result.embedding[:dimension],
            dtype=np.float64,
        )
        python_embedding = np.asarray(
            python_result["embedding"],
            dtype=np.float64,
        )
        self.assertEqual(native_embedding.shape, python_embedding.shape)
        self.assertLessEqual(
            float(np.max(np.abs(native_embedding - python_embedding))),
            5e-5,
        )

        self.assertEqual(
            int(native_result.selected_embedding_count),
            len(python_result["faceEmbeddings"]),
        )
        for index, python_selected in enumerate(python_result["faceEmbeddings"]):
            native_selected = native_result.selected_embeddings[index]
            self.assertEqual(
                int(native_selected.frame_index),
                int(python_selected["frameIndex"]),
            )
            self.assertAlmostEqual(
                float(native_selected.quality),
                float(python_selected["quality"]),
                places=5,
            )

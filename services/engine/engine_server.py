from __future__ import annotations

import json
import os
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any

from biometric_engine import BiometricAnalyzer


BASE_DIRECTORY = Path(__file__).resolve().parent


def env_path(name: str, default_relative_path: str) -> str:
    value = os.getenv(name)
    if value:
        return str(Path(value).resolve())
    return str((BASE_DIRECTORY / default_relative_path).resolve())


ANALYZER = BiometricAnalyzer(
    yunet_model_path=env_path("FACEPROOF_YUNET_MODEL", "../../models/yunet/face_detection_yunet_2023mar.onnx"),
    sface_model_path=env_path("FACEPROOF_SFACE_MODEL", "../../models/sface/face_recognition_sface_2021dec.onnx"),
    minifasnet_model_path=env_path("FACEPROOF_MINIFASNET_MODEL", "../../models/minifasnet/MiniFASNetV2.onnx"),
)
ANALYZER_LOCK = threading.Lock()
GUIDE_LOCK = threading.Lock()


class Handler(BaseHTTPRequestHandler):
    server_version = "FaceProofEngine/0.2"

    def do_GET(self) -> None:
        if self.path == "/health":
            self._write_json(200, {"status": "ok"})
            return
        self._write_json(404, {"error": "route not found"})

    def do_POST(self) -> None:
        try:
            if self.path == "/analyze":
                payload = self._read_json(max_bytes=16 * 1024 * 1024)
                with ANALYZER_LOCK:
                    result = ANALYZER.analyze(payload)
                self._write_json(200, result)
                return

            if self.path == "/reference":
                payload = self._read_json(max_bytes=6 * 1024 * 1024)
                with ANALYZER_LOCK:
                    result = ANALYZER.extract_reference(payload)
                self._write_json(200, result)
                return

            if self.path == "/guide":
                payload = self._read_json(max_bytes=2 * 1024 * 1024)
                with GUIDE_LOCK:
                    result = ANALYZER.guide(payload)
                self._write_json(200, result)
                return

            self._write_json(404, {"error": "route not found"})
        except ValueError as error:
            self._write_json(400, {"error": str(error)})
        except Exception:
            self._write_json(500, {"error": "engine failure"})

    def log_message(self, format_string: str, *args: Any) -> None:
        print(f"[engine] {self.address_string()} - {format_string % args}")

    def _read_json(self, max_bytes: int) -> dict[str, Any]:
        content_length = int(self.headers.get("Content-Length", "0"))
        if content_length <= 0 or content_length > max_bytes:
            raise ValueError("invalid request size")
        body = self.rfile.read(content_length)
        payload = json.loads(body)
        if not isinstance(payload, dict):
            raise ValueError("JSON body must be an object")
        return payload

    def _write_json(self, status: int, payload: dict[str, Any]) -> None:
        body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def main() -> None:
    host = os.getenv("FACEPROOF_ENGINE_ADDR", "127.0.0.1")
    port = int(os.getenv("FACEPROOF_ENGINE_PORT", "8090"))
    server = ThreadingHTTPServer((host, port), Handler)
    print(f"FaceProof engine listening on http://{host}:{port}")
    server.serve_forever()


if __name__ == "__main__":
    main()

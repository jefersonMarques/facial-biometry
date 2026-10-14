from __future__ import annotations

import json
import unittest

from biometric_engine.http_payload import parse_identity_multipart


class IdentityMultipartPayloadTest(unittest.TestCase):
    def test_preserves_binary_jpeg_frames_and_metadata(self) -> None:
        boundary = "faceproof-test-boundary"
        jpeg = b"\xff\xd8\xff\xe0binary-jpeg\xff\xd9"
        manifest = {
            "guidedFrames": [
                {
                    "phase": "far" if index < 3 else "near",
                    "clientQuality": {
                        "brightness": 0.5,
                        "contrast": 0.1,
                        "sharpness": 0.05,
                    },
                }
                for index in range(6)
            ]
        }

        chunks: list[bytes] = []
        chunks.append(
            (
                f"--{boundary}\r\n"
                'Content-Disposition: form-data; name="manifest"\r\n'
                "Content-Type: text/plain; charset=utf-8\r\n"
                "\r\n"
                f"{json.dumps(manifest)}\r\n"
            ).encode("utf-8")
        )

        for index in range(6):
            chunks.append(
                (
                    f"--{boundary}\r\n"
                    f'Content-Disposition: form-data; name="frame"; filename="{index:02d}.jpg"\r\n'
                    "Content-Type: image/jpeg\r\n"
                    "\r\n"
                ).encode("ascii")
                + jpeg
                + b"\r\n"
            )

        chunks.append(f"--{boundary}--\r\n".encode("ascii"))
        body = b"".join(chunks)

        payload = parse_identity_multipart(
            f"multipart/form-data; boundary={boundary}",
            body,
        )

        frames = payload["guidedFrames"]
        self.assertEqual(len(frames), 6)
        self.assertEqual(frames[0]["phase"], "far")
        self.assertEqual(frames[5]["phase"], "near")
        self.assertEqual(frames[0]["imageBytes"], jpeg)
        self.assertNotIn("imageBase64", frames[0])

    def test_rejects_non_jpeg_frame(self) -> None:
        boundary = "faceproof-test-boundary"
        manifest = {
            "guidedFrames": [
                {"phase": "far"},
                {"phase": "far"},
                {"phase": "far"},
                {"phase": "near"},
                {"phase": "near"},
                {"phase": "near"},
            ]
        }

        chunks = [
            (
                f"--{boundary}\r\n"
                'Content-Disposition: form-data; name="manifest"\r\n\r\n'
                f"{json.dumps(manifest)}\r\n"
            ).encode("utf-8")
        ]
        for index in range(6):
            chunks.append(
                (
                    f"--{boundary}\r\n"
                    f'Content-Disposition: form-data; name="frame"; filename="{index:02d}.jpg"\r\n'
                    "Content-Type: image/jpeg\r\n\r\n"
                ).encode("ascii")
                + b"not-a-jpeg"
                + b"\r\n"
            )
        chunks.append(f"--{boundary}--\r\n".encode("ascii"))

        with self.assertRaisesRegex(ValueError, "not a JPEG"):
            parse_identity_multipart(
                f"multipart/form-data; boundary={boundary}",
                b"".join(chunks),
            )


if __name__ == "__main__":
    unittest.main()

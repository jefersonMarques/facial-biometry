from __future__ import annotations

import json
from email import policy
from email.parser import BytesParser
from typing import Any


def parse_identity_multipart(
    content_type: str,
    body: bytes,
    *,
    max_frame_bytes: int = 3 * 1024 * 1024,
    max_manifest_bytes: int = 256 * 1024,
) -> dict[str, Any]:
    message = BytesParser(policy=policy.default).parsebytes(
        (
            f"Content-Type: {content_type}\r\n"
            "MIME-Version: 1.0\r\n"
            "\r\n"
        ).encode("ascii")
        + body
    )
    if not message.is_multipart():
        raise ValueError("invalid multipart identity capture")

    manifest_raw: bytes | None = None
    frame_bytes: list[bytes] = []

    for part in message.iter_parts():
        if part.get_content_disposition() != "form-data":
            continue

        field_name = part.get_param("name", header="content-disposition")
        payload = part.get_payload(decode=True) or b""

        if field_name == "manifest":
            if manifest_raw is not None:
                raise ValueError("identity capture manifest is duplicated")
            if len(payload) > max_manifest_bytes:
                raise ValueError("identity capture manifest is too large")
            manifest_raw = payload
            continue

        if field_name != "frame":
            continue
        if len(frame_bytes) >= 12:
            raise ValueError("identity capture has too many frames")
        if not payload or len(payload) > max_frame_bytes:
            raise ValueError("identity capture frame exceeds size limit")
        if part.get_content_type() != "image/jpeg":
            raise ValueError("identity capture frame must be JPEG")
        if not payload.startswith(b"\xff\xd8\xff"):
            raise ValueError("identity capture frame is not a JPEG")
        frame_bytes.append(payload)

    if manifest_raw is None:
        raise ValueError("identity capture manifest is missing")

    try:
        manifest = json.loads(manifest_raw.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as error:
        raise ValueError("identity capture manifest is invalid") from error

    guided_frames = manifest.get("guidedFrames") if isinstance(manifest, dict) else None
    if not isinstance(guided_frames, list) or not 6 <= len(guided_frames) <= 12:
        raise ValueError("guidedFrames must contain between 6 and 12 items")
    if len(guided_frames) != len(frame_bytes):
        raise ValueError("identity capture frame count does not match manifest")

    normalized_frames: list[dict[str, Any]] = []
    for metadata, image_bytes in zip(guided_frames, frame_bytes, strict=True):
        if not isinstance(metadata, dict):
            raise ValueError("identity capture frame metadata is invalid")
        frame = dict(metadata)
        frame["imageBytes"] = image_bytes
        normalized_frames.append(frame)

    return {"guidedFrames": normalized_frames}

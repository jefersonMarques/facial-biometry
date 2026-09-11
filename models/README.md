# Models

Model binaries are intentionally not embedded in this repository. Download them from their original public sources with checksum verification:

```bash
python models/download_models.py
```

Expected artifacts:

```text
models/yunet/face_detection_yunet_2023mar.onnx
models/sface/face_recognition_sface_2021dec.onnx
models/minifasnet/MiniFASNetV2.onnx
```

The downloader pins SHA-256 checksums for all three ONNX files.

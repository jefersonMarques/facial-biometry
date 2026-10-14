#!/usr/bin/env python3
"""Executa benchmark local com imagens privadas e exporta somente métricas."""

from __future__ import annotations

import argparse
import csv
import json
import statistics
import subprocess
from collections import defaultdict
from pathlib import Path

LABELS = frozenset({"genuine_live", "impostor_live"})
PAIRS = ("pdfVio", "vioLive", "pdfLive")


def summarize(samples: list[dict]) -> dict:
    grouped: dict[tuple[str, str, str], list[float]] = defaultdict(list)
    success: dict[tuple[str, str, str], int] = defaultdict(int)
    for sample in samples:
        for variant in sample["variants"]:
            for pair in PAIRS:
                value = variant[pair].get("robustCosine")
                key = (sample["label"], variant["method"], pair)
                if variant[pair]["status"] == "compared" and isinstance(value, (int, float)):
                    grouped[key].append(float(value))
                    success[key] += 1

    stats = []
    for key in sorted(grouped):
        label, method, pair = key
        values = grouped[key]
        stats.append({
            "label": label,
            "method": method,
            "pair": pair,
            "count": success[key],
            "median": statistics.median(values),
            "minimum": min(values),
            "maximum": max(values),
        })
    return {
        "schemaVersion": 1,
        "purpose": "diagnostic_only_not_calibrated",
        "sampleCount": len(samples),
        "groups": stats,
    }


def run_benchmark(
    manifest: Path,
    binary: Path,
    yunet: Path,
    sface: Path,
    output: Path,
) -> dict:
    samples: list[dict] = []
    with manifest.open(newline="", encoding="utf-8") as stream:
        reader = csv.DictReader(stream)
        required = {"label", "pdf_path", "vio_path", "live_path"}
        if not required.issubset(reader.fieldnames or []):
            raise ValueError("manifest requires label,pdf_path,vio_path,live_path")

        for row_number, row in enumerate(reader, start=1):
            label = (row.get("label") or "").strip()
            if label not in LABELS:
                raise ValueError(f"row {row_number}: invalid label")

            paths = [
                (manifest.parent / (row.get(key) or "")).resolve()
                for key in ("pdf_path", "vio_path", "live_path")
            ]
            if not all(path.is_file() for path in paths):
                raise ValueError(f"row {row_number}: missing source image")

            try:
                process = subprocess.run(
                    [str(binary), *map(str, paths), str(yunet), str(sface)],
                    check=True,
                    capture_output=True,
                    text=True,
                    timeout=45,
                )
                report = json.loads(process.stdout)
            except (subprocess.CalledProcessError, subprocess.TimeoutExpired, json.JSONDecodeError) as error:
                raise RuntimeError(f"row {row_number}: benchmark failed") from error

            if report.get("mode") != "offline_low_resolution_diagnostic":
                raise ValueError(f"row {row_number}: unexpected benchmark format")
            samples.append({
                "sampleIndex": row_number,
                "label": label,
                "originalWidth": report["vioOriginalWidth"],
                "originalHeight": report["vioOriginalHeight"],
                "variants": report["variants"],
            })

    if not samples:
        raise ValueError("empty benchmark manifest")

    output.parent.mkdir(parents=True, exist_ok=True)
    # O arquivo de relatório contém métricas, não imagens, nomes ou embeddings.
    with output.open("w", encoding="utf-8") as stream:
        for record in samples:
            stream.write(json.dumps(record, allow_nan=False, separators=(",", ":")) + "\n")
    summary = summarize(samples)
    summary_path = output.with_suffix(".summary.json")
    summary_path.write_text(
        json.dumps(summary, allow_nan=False, indent=2) + "\n",
        encoding="utf-8",
    )
    return summary


def main() -> None:
    parser = argparse.ArgumentParser(description="Offline FaceProof low-resolution benchmark")
    parser.add_argument("--manifest", type=Path, required=True)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--yunet", type=Path, required=True)
    parser.add_argument("--sface", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    summary = run_benchmark(
        args.manifest.resolve(),
        args.binary.resolve(),
        args.yunet.resolve(),
        args.sface.resolve(),
        args.output.resolve(),
    )
    print(f"Benchmark completed: {summary['sampleCount']} local samples; no image files exported")


if __name__ == "__main__":
    main()

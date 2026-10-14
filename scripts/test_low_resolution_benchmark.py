"""Valida os agregados do benchmark sem fotografias ou embeddings."""

import importlib.util
import unittest
from pathlib import Path

path = Path(__file__).with_name("run-low-resolution-benchmark.py")
spec = importlib.util.spec_from_file_location("low_resolution_benchmark", path)
benchmark = importlib.util.module_from_spec(spec)
spec.loader.exec_module(benchmark)


def variant(method, pdf_vio, vio_live, pdf_live):
    return {
        "method": method,
        "pdfVio": {"status": "compared", "robustCosine": pdf_vio},
        "vioLive": {"status": "compared", "robustCosine": vio_live},
        "pdfLive": {"status": "compared", "robustCosine": pdf_live},
    }


class BenchmarkSummaryTests(unittest.TestCase):
    def test_genuine_and_impostor_groups_are_separate(self):
        records = [
            {
                "label": "genuine_live",
                "variants": [
                    variant("production_cubic", .39, .38, .61),
                    variant("linear", .42, .41, .61),
                ],
            },
            {
                "label": "impostor_live",
                "variants": [
                    variant("production_cubic", .40, .21, .19),
                    variant("linear", .43, .22, .19),
                ],
            },
        ]
        summary = benchmark.summarize(records)
        self.assertEqual(summary["sampleCount"], 2)
        groups = {
            (entry["label"], entry["method"], entry["pair"]): entry
            for entry in summary["groups"]
        }
        self.assertEqual(
            groups[("genuine_live", "production_cubic", "vioLive")]["median"],
            .38,
        )
        self.assertEqual(
            groups[("impostor_live", "production_cubic", "vioLive")]["median"],
            .21,
        )
        self.assertNotIn("decision", summary)
        self.assertNotIn("threshold", summary)

    def test_failed_comparisons_do_not_generate_synthetic_scores(self):
        records = [{
            "label": "genuine_live",
            "variants": [{
                "method": "nearest",
                "pdfVio": {"status": "unavailable"},
                "vioLive": {"status": "unavailable"},
                "pdfLive": {"status": "compared", "robustCosine": .61},
            }],
        }]
        summary = benchmark.summarize(records)
        self.assertEqual(len(summary["groups"]), 1)
        self.assertEqual(summary["groups"][0]["pair"], "pdfLive")


if __name__ == "__main__":
    unittest.main()

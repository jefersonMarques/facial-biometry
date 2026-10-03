import assert from "node:assert/strict";
import test from "node:test";
import { summarizeGeometryEvidence } from "../dist/liveness-v2.js";

function observations({ perspectiveChange = false, scaleNear = 0.52 } = {}) {
    const rows = [];
    for (let index = 0; index < 6; index++) {
        rows.push({
            timestampMs: index * 80,
            phase: "far",
            scale: 0.32 + index * 0.0005,
            noseWidthRatio: 0.210,
            eyeSpanRatio: 0.610,
            mouthWidthRatio: 0.410,
            noseToChinRatio: 0.590,
            noseToForeheadRatio: 0.510,
            noseDepthRatio: 0.180,
            yawAsymmetry: 0.010,
        });
    }
    for (let index = 0; index < 6; index++) {
        rows.push({
            timestampMs: 700 + index * 80,
            phase: "near",
            scale: scaleNear + index * 0.0005,
            noseWidthRatio: perspectiveChange ? 0.220 : 0.210,
            eyeSpanRatio: perspectiveChange ? 0.620 : 0.610,
            mouthWidthRatio: perspectiveChange ? 0.414 : 0.410,
            noseToChinRatio: perspectiveChange ? 0.580 : 0.590,
            noseToForeheadRatio: perspectiveChange ? 0.500 : 0.510,
            noseDepthRatio: perspectiveChange ? 0.195 : 0.180,
            yawAsymmetry: 0.011,
        });
    }
    return rows;
}

test("requires enough far and near observations", () => {
    const result = summarizeGeometryEvidence(observations().slice(0, 7));
    assert.equal(result.status, "insufficient");
});

test("rewards a clear far-to-near transition", () => {
    const result = summarizeGeometryEvidence(observations());
    assert.equal(result.status, "experimental");
    assert.ok(result.scaleRatio > 1.5);
    assert.ok(result.transitionScore > 0.75);
});

test("geometry response raises experimental evidence above planar scaling", () => {
    const planar = summarizeGeometryEvidence(observations());
    const geometry = summarizeGeometryEvidence(observations({ perspectiveChange: true }));
    assert.ok(geometry.perspectiveChange > planar.perspectiveChange);
    assert.ok(geometry.depthChange > planar.depthChange);
    assert.ok(geometry.evidenceScore > planar.evidenceScore);
});

test("does not treat a weak distance transition as sufficient", () => {
    const result = summarizeGeometryEvidence(observations({ perspectiveChange: true, scaleNear: 0.34 }));
    assert.equal(result.status, "insufficient");
});

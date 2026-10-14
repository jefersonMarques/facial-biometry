import test from "node:test";
import assert from "node:assert/strict";

import { LocalCaptureGate } from "../dist/local-capture-gate.js";

function sample(timestampMs, {
    centerX = 0.50,
    centerY = 0.50,
    heightRatio = 0.30,
    rollDegrees = 0,
} = {}) {
    return {
        timestampMs,
        faceDetected: true,
        centerX,
        centerY,
        widthRatio: 0.22,
        heightRatio,
        rollDegrees,
        faceSizeScore: 1,
    };
}

test("accepts a short stable local window", () => {
    const gate = new LocalCaptureGate();
    gate.push(sample(0));
    gate.push(sample(180, { centerX: 0.505, centerY: 0.498, heightRatio: 0.302, rollDegrees: 0.4 }));
    gate.push(sample(360, { centerX: 0.501, centerY: 0.503, heightRatio: 0.299, rollDegrees: -0.2 }));
    gate.push(sample(540, { centerX: 0.503, centerY: 0.501, heightRatio: 0.301, rollDegrees: 0.2 }));

    const result = gate.evaluate("far", 0, 540);
    assert.equal(result.stable, true);
    assert.equal(result.nearScaleReady, true);
});

test("rejects continuous movement inside the local window", () => {
    const gate = new LocalCaptureGate();
    gate.push(sample(0, { centerX: 0.46, centerY: 0.47 }));
    gate.push(sample(180, { centerX: 0.55, centerY: 0.54, rollDegrees: 5 }));
    gate.push(sample(360, { centerX: 0.43, centerY: 0.45, rollDegrees: -4 }));
    gate.push(sample(540, { centerX: 0.57, centerY: 0.56, rollDegrees: 6 }));

    const result = gate.evaluate("far", 0, 540);
    assert.equal(result.stable, false);
    assert.ok(result.centerXRange > 0.045);
});

test("requires the near phase to be meaningfully larger than far", () => {
    const gate = new LocalCaptureGate();
    gate.push(sample(0, { heightRatio: 0.300 }));
    gate.push(sample(180, { heightRatio: 0.302 }));
    gate.push(sample(360, { heightRatio: 0.299 }));
    gate.push(sample(540, { heightRatio: 0.301 }));
    const far = gate.lockFarReference(540);
    assert.ok(far && far > 0.29 && far < 0.31);

    gate.push(sample(900, { heightRatio: 0.320 }));
    gate.push(sample(1080, { heightRatio: 0.321 }));
    gate.push(sample(1260, { heightRatio: 0.322 }));
    let result = gate.evaluate("near", 900, 1260);
    assert.equal(result.nearScaleReady, false);
    assert.ok((result.nearScaleRatio ?? 0) < 1.10);

    gate.push(sample(1440, { heightRatio: 0.350 }));
    result = gate.evaluate("near", 900, 1440);
    assert.equal(result.nearScaleReady, true);
    assert.ok((result.nearScaleRatio ?? 0) >= 1.10);
});

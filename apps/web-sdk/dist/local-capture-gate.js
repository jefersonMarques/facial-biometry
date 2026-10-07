const HISTORY_MS = 5_000;
const STABILITY_WINDOW_MS = 750;
const MIN_SAMPLES = 3;
const MIN_SPAN_MS = 320;
const MAX_CENTER_X_RANGE = 0.045;
const MAX_CENTER_Y_RANGE = 0.055;
const MAX_HEIGHT_RELATIVE_RANGE = 0.12;
const MAX_ROLL_RANGE = 7.0;
const MIN_NEAR_SCALE_RATIO = 1.10;
export class LocalCaptureGate {
    history = [];
    farReferenceHeightRatio = null;
    reset() {
        this.history.length = 0;
        this.farReferenceHeightRatio = null;
    }
    push(sample) {
        this.history.push(sample);
        const cutoff = sample.timestampMs - HISTORY_MS;
        while (this.history.length > 0 &&
            (this.history[0]?.timestampMs ?? sample.timestampMs) < cutoff) {
            this.history.shift();
        }
    }
    lockFarReference(nowMs = performance.now()) {
        const samples = this.recentSamples(nowMs - STABILITY_WINDOW_MS, nowMs);
        const heights = samples
            .map((sample) => sample.heightRatio)
            .filter((value) => Number.isFinite(value) && value > 0);
        this.farReferenceHeightRatio = heights.length > 0
            ? median(heights)
            : null;
        return this.farReferenceHeightRatio;
    }
    evaluate(phase, minTimestampMs = 0, nowMs = performance.now()) {
        const cutoff = Math.max(minTimestampMs, nowMs - STABILITY_WINDOW_MS);
        const samples = this.recentSamples(cutoff, nowMs);
        const centerXRange = range(samples.map((sample) => sample.centerX));
        const centerYRange = range(samples.map((sample) => sample.centerY));
        const heights = samples.map((sample) => sample.heightRatio);
        const medianHeight = median(heights);
        const heightRelativeRange = medianHeight > 1e-6
            ? range(heights) / medianHeight
            : Number.POSITIVE_INFINITY;
        const rollRange = range(samples.map((sample) => sample.rollDegrees));
        const spanMs = samples.length > 1
            ? (samples[samples.length - 1]?.timestampMs ?? nowMs) -
                (samples[0]?.timestampMs ?? nowMs)
            : 0;
        const stable = samples.length >= MIN_SAMPLES &&
            spanMs >= MIN_SPAN_MS &&
            centerXRange <= MAX_CENTER_X_RANGE &&
            centerYRange <= MAX_CENTER_Y_RANGE &&
            heightRelativeRange <= MAX_HEIGHT_RELATIVE_RANGE &&
            rollRange <= MAX_ROLL_RANGE;
        const current = samples[samples.length - 1];
        const nearScaleRatio = phase === "near" &&
            current &&
            this.farReferenceHeightRatio &&
            this.farReferenceHeightRatio > 0
            ? current.heightRatio / this.farReferenceHeightRatio
            : null;
        return {
            stable,
            sampleCount: samples.length,
            spanMs,
            centerXRange,
            centerYRange,
            heightRelativeRange,
            rollRange,
            nearScaleRatio,
            nearScaleReady: phase !== "near" ||
                nearScaleRatio === null ||
                nearScaleRatio >= MIN_NEAR_SCALE_RATIO,
        };
    }
    recentSamples(minTimestampMs, nowMs) {
        return this.history.filter((sample) => sample.timestampMs >= minTimestampMs &&
            sample.timestampMs <= nowMs);
    }
}
function range(values) {
    const finite = values.filter(Number.isFinite);
    if (finite.length === 0) {
        return 0;
    }
    return Math.max(...finite) - Math.min(...finite);
}
function median(values) {
    const finite = values.filter(Number.isFinite).sort((left, right) => left - right);
    if (finite.length === 0) {
        return 0;
    }
    const middle = Math.floor(finite.length / 2);
    if (finite.length % 2 === 1) {
        return finite[middle] ?? 0;
    }
    return ((finite[middle - 1] ?? 0) + (finite[middle] ?? 0)) / 2;
}

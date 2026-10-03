import { GeometryWasmShadow, } from "./liveness-core-shadow.js";
const DEFAULT_MODULE_URL = "https://cdn.jsdelivr.net/npm/@mediapipe/tasks-vision@1.0.1/+esm";
const DEFAULT_WASM_ROOT = "https://cdn.jsdelivr.net/npm/@mediapipe/tasks-vision@1.0.1/wasm";
const DEFAULT_MODEL_URL = "https://storage.googleapis.com/mediapipe-models/face_landmarker/face_landmarker/float16/1/face_landmarker.task";
const DEFAULT_SAMPLE_INTERVAL_MS = 180;
const DEFAULT_MAX_OBSERVATIONS = 120;
const ANALYSIS_WIDTH = 384;
const MIN_PHASE_SAMPLES = 4;
export class ExperimentalGeometryLiveness {
    landmarker = null;
    initialization = null;
    video = null;
    running = false;
    animationFrame = 0;
    lastInferenceAt = 0;
    phase = null;
    observations = [];
    analysisCanvas;
    analysisContext;
    wasmShadow = null;
    onUpdate = null;
    options;
    constructor(options = {}) {
        this.analysisCanvas = document.createElement("canvas");
        const context = this.analysisCanvas.getContext("2d", { alpha: false });
        if (!context) {
            throw new Error("Canvas 2D is not supported");
        }
        this.analysisContext = context;
        this.options = {
            moduleUrl: options.moduleUrl ?? DEFAULT_MODULE_URL,
            wasmRoot: options.wasmRoot ?? DEFAULT_WASM_ROOT,
            modelUrl: options.modelUrl ?? DEFAULT_MODEL_URL,
            sampleIntervalMs: options.sampleIntervalMs ?? DEFAULT_SAMPLE_INTERVAL_MS,
            maxObservations: options.maxObservations ?? DEFAULT_MAX_OBSERVATIONS,
        };
    }
    async initialize() {
        if (this.landmarker) {
            return;
        }
        if (!this.initialization) {
            this.initialization = this.initializeInternal();
        }
        await this.initialization;
    }
    async start(video, onUpdate) {
        await this.initialize();
        this.video = video;
        this.onUpdate = onUpdate ?? null;
        if (this.running) {
            return;
        }
        this.running = true;
        this.animationFrame = window.requestAnimationFrame((timestamp) => this.tick(timestamp));
    }
    stop() {
        this.running = false;
        if (this.animationFrame !== 0) {
            window.cancelAnimationFrame(this.animationFrame);
            this.animationFrame = 0;
        }
        this.video = null;
        this.lastInferenceAt = 0;
    }
    dispose() {
        this.stop();
        this.landmarker?.close();
        this.landmarker = null;
        this.wasmShadow?.dispose();
        this.wasmShadow = null;
        this.initialization = null;
    }
    reset() {
        this.observations.length = 0;
        this.lastInferenceAt = 0;
        this.wasmShadow?.reset();
    }
    setPhase(phase) {
        this.phase = phase;
    }
    summarize() {
        return summarizeGeometryEvidence(this.observations);
    }
    getWasmShadowDiagnostics() {
        return this.wasmShadow?.snapshot() ?? {
            state: "idle",
            summary: null,
            guide: null,
            errorMessage: "",
        };
    }
    async initializeInternal() {
        const moduleUrl = this.options.moduleUrl;
        const visionModule = await import(moduleUrl);
        const fileset = await visionModule.FilesetResolver.forVisionTasks(this.options.wasmRoot);
        const create = async (delegate) => visionModule.FaceLandmarker.createFromOptions(fileset, {
            baseOptions: {
                modelAssetPath: this.options.modelUrl,
                delegate,
            },
            runningMode: "VIDEO",
            numFaces: 1,
            minFaceDetectionConfidence: 0.65,
            minFacePresenceConfidence: 0.65,
            minTrackingConfidence: 0.60,
            outputFaceBlendshapes: false,
            outputFacialTransformationMatrixes: false,
        });
        try {
            this.landmarker = await create("GPU");
        }
        catch {
            this.landmarker = await create("CPU");
        }
        const wasmShadow = new GeometryWasmShadow((diagnostics) => {
            if (diagnostics.guide) {
                dispatchLocalGuide(diagnostics.guide);
            }
            this.onUpdate?.(this.summarize());
        });
        this.wasmShadow = wasmShadow;
        void wasmShadow.initialize().catch(() => {
            // Shadow mode: WASM failure must never affect the active verification flow.
        });
    }
    tick(timestampMs) {
        if (!this.running) {
            return;
        }
        try {
            const video = this.video;
            if (video &&
                this.landmarker &&
                this.phase &&
                video.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA &&
                timestampMs - this.lastInferenceAt >= this.options.sampleIntervalMs) {
                this.lastInferenceAt = timestampMs;
                const analysisFrame = this.prepareAnalysisFrame(video);
                if (!analysisFrame) {
                    return;
                }
                const result = this.landmarker.detectForVideo(analysisFrame, timestampMs);
                const landmarks = result.faceLandmarks?.[0];
                if (landmarks) {
                    this.wasmShadow?.push(this.phase, timestampMs, landmarks);
                    const observation = extractGeometryObservation(landmarks, this.phase, timestampMs);
                    if (observation) {
                        this.observations.push(observation);
                        if (this.observations.length > this.options.maxObservations) {
                            this.observations.splice(0, this.observations.length - this.options.maxObservations);
                        }
                        this.onUpdate?.(this.summarize());
                    }
                }
            }
        }
        finally {
            if (this.running) {
                this.animationFrame = window.requestAnimationFrame((nextTimestamp) => this.tick(nextTimestamp));
            }
        }
    }
    prepareAnalysisFrame(video) {
        const sourceWidth = video.videoWidth;
        const sourceHeight = video.videoHeight;
        if (sourceWidth <= 0 || sourceHeight <= 0) {
            return null;
        }
        const width = Math.min(ANALYSIS_WIDTH, sourceWidth);
        const height = Math.max(1, Math.round((width / sourceWidth) * sourceHeight));
        if (this.analysisCanvas.width !== width || this.analysisCanvas.height !== height) {
            this.analysisCanvas.width = width;
            this.analysisCanvas.height = height;
        }
        this.analysisContext.drawImage(video, 0, 0, width, height);
        return this.analysisCanvas;
    }
}
export function extractGeometryObservation(landmarks, phase, timestampMs) {
    const leftCheek = landmarkAt(landmarks, 234);
    const rightCheek = landmarkAt(landmarks, 454);
    const noseTip = landmarkAt(landmarks, 1);
    const noseLeft = landmarkAt(landmarks, 98);
    const noseRight = landmarkAt(landmarks, 327);
    const leftEyeOuter = landmarkAt(landmarks, 33);
    const rightEyeOuter = landmarkAt(landmarks, 263);
    const mouthLeft = landmarkAt(landmarks, 61);
    const mouthRight = landmarkAt(landmarks, 291);
    const chin = landmarkAt(landmarks, 152);
    const forehead = landmarkAt(landmarks, 10);
    if (!leftCheek || !rightCheek || !noseTip || !noseLeft || !noseRight ||
        !leftEyeOuter || !rightEyeOuter || !mouthLeft || !mouthRight || !chin || !forehead) {
        return null;
    }
    const scale = distance2D(leftCheek, rightCheek);
    if (!Number.isFinite(scale) || scale < 0.05) {
        return null;
    }
    const leftNoseDistance = distance2D(noseTip, leftCheek);
    const rightNoseDistance = distance2D(noseTip, rightCheek);
    const cheekDepth = (leftCheek.z + rightCheek.z) / 2;
    return {
        timestampMs,
        phase,
        scale,
        noseWidthRatio: distance2D(noseLeft, noseRight) / scale,
        eyeSpanRatio: distance2D(leftEyeOuter, rightEyeOuter) / scale,
        mouthWidthRatio: distance2D(mouthLeft, mouthRight) / scale,
        noseToChinRatio: distance2D(noseTip, chin) / scale,
        noseToForeheadRatio: distance2D(noseTip, forehead) / scale,
        noseDepthRatio: (cheekDepth - noseTip.z) / scale,
        yawAsymmetry: Math.abs(leftNoseDistance - rightNoseDistance) / scale,
    };
}
export function summarizeGeometryEvidence(observations) {
    const far = observations.filter((item) => item.phase === "far");
    const near = observations.filter((item) => item.phase === "near");
    const empty = baseSummary(observations.length, far.length, near.length);
    if (far.length < MIN_PHASE_SAMPLES || near.length < MIN_PHASE_SAMPLES) {
        return empty;
    }
    const farMetrics = aggregatePhase(far);
    const nearMetrics = aggregatePhase(near);
    if (farMetrics.scale <= 0) {
        return empty;
    }
    const scaleRatio = nearMetrics.scale / farMetrics.scale;
    const transitionScore = clamp01((scaleRatio - 1.15) / 0.45);
    const perspectiveChange = median([
        relativeChange(farMetrics.noseWidthRatio, nearMetrics.noseWidthRatio),
        relativeChange(farMetrics.eyeSpanRatio, nearMetrics.eyeSpanRatio),
        relativeChange(farMetrics.mouthWidthRatio, nearMetrics.mouthWidthRatio),
        relativeChange(farMetrics.noseToChinRatio, nearMetrics.noseToChinRatio),
        relativeChange(farMetrics.noseToForeheadRatio, nearMetrics.noseToForeheadRatio),
    ]);
    const depthChange = Math.abs(nearMetrics.noseDepthRatio - farMetrics.noseDepthRatio);
    const phaseStability = clamp01(1 - median([
        phaseVariation(far),
        phaseVariation(near),
    ]) / 0.05);
    const perspectiveScore = clamp01((perspectiveChange - 0.003) / 0.030);
    const depthScore = clamp01((depthChange - 0.002) / 0.025);
    const evidenceScore = clamp01(0.45 * transitionScore +
        0.30 * perspectiveScore +
        0.15 * depthScore +
        0.10 * phaseStability);
    return {
        status: scaleRatio >= 1.10 ? "experimental" : "insufficient",
        sampleCount: observations.length,
        farSamples: far.length,
        nearSamples: near.length,
        scaleRatio,
        transitionScore,
        perspectiveChange,
        depthChange,
        phaseStability,
        evidenceScore,
    };
}
function baseSummary(sampleCount, farSamples, nearSamples) {
    return {
        status: "insufficient",
        sampleCount,
        farSamples,
        nearSamples,
        scaleRatio: 0,
        transitionScore: 0,
        perspectiveChange: 0,
        depthChange: 0,
        phaseStability: 0,
        evidenceScore: 0,
    };
}
function aggregatePhase(observations) {
    return {
        timestampMs: median(observations.map((item) => item.timestampMs)),
        phase: observations[0]?.phase ?? "far",
        scale: median(observations.map((item) => item.scale)),
        noseWidthRatio: median(observations.map((item) => item.noseWidthRatio)),
        eyeSpanRatio: median(observations.map((item) => item.eyeSpanRatio)),
        mouthWidthRatio: median(observations.map((item) => item.mouthWidthRatio)),
        noseToChinRatio: median(observations.map((item) => item.noseToChinRatio)),
        noseToForeheadRatio: median(observations.map((item) => item.noseToForeheadRatio)),
        noseDepthRatio: median(observations.map((item) => item.noseDepthRatio)),
        yawAsymmetry: median(observations.map((item) => item.yawAsymmetry)),
    };
}
function phaseVariation(observations) {
    const features = [
        observations.map((item) => item.noseWidthRatio),
        observations.map((item) => item.eyeSpanRatio),
        observations.map((item) => item.mouthWidthRatio),
        observations.map((item) => item.noseToChinRatio),
        observations.map((item) => item.noseToForeheadRatio),
    ];
    return median(features.map(coefficientOfVariation));
}
function coefficientOfVariation(values) {
    const center = median(values);
    if (Math.abs(center) < 1e-8) {
        return 0;
    }
    const deviations = values.map((value) => Math.abs(value - center));
    return median(deviations) / Math.abs(center);
}
function relativeChange(from, to) {
    const denominator = Math.max(Math.abs(from), 1e-6);
    return Math.abs(to - from) / denominator;
}
function landmarkAt(landmarks, index) {
    const landmark = landmarks[index];
    if (!landmark) {
        return null;
    }
    if (![landmark.x, landmark.y, landmark.z].every(Number.isFinite)) {
        return null;
    }
    return landmark;
}
function distance2D(left, right) {
    return Math.hypot(right.x - left.x, right.y - left.y);
}
function median(values) {
    if (values.length === 0) {
        return 0;
    }
    const sorted = [...values].sort((left, right) => left - right);
    const middle = Math.floor(sorted.length / 2);
    const right = sorted[middle] ?? 0;
    if (sorted.length % 2 === 1) {
        return right;
    }
    const left = sorted[middle - 1] ?? right;
    return (left + right) / 2;
}
function clamp01(value) {
    return Math.max(0, Math.min(1, value));
}
function dispatchLocalGuide(guide) {
    window.dispatchEvent(new CustomEvent("faceproof:local-guide", {
        detail: {
            timestampMs: performance.now(),
            guide,
        },
    }));
}

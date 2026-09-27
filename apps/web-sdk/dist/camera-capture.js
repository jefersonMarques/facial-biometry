import { FrameQualityAnalyzer } from "./frame-quality.js";
const SDK_VERSION = "0.2.0";
const READINESS_TIMEOUT_MS = 4_000;
const READINESS_SAMPLE_MS = 220;
const REQUIRED_GOOD_SAMPLES = 2;
export class CameraCapture {
    video;
    stream = null;
    canvas;
    context;
    qualityAnalyzer = new FrameQualityAnalyzer();
    constructor(video) {
        this.video = video;
        this.canvas = document.createElement("canvas");
        const context = this.canvas.getContext("2d", { alpha: false, willReadFrequently: true });
        if (!context) {
            throw new Error("Canvas 2D is not supported");
        }
        this.context = context;
    }
    async start() {
        if (this.stream) {
            return;
        }
        this.stream = await navigator.mediaDevices.getUserMedia({
            audio: false,
            video: {
                facingMode: "user",
                width: { ideal: 1280 },
                height: { ideal: 720 },
                frameRate: { ideal: 30, max: 30 },
            },
        });
        this.video.srcObject = this.stream;
        await this.video.play();
    }
    stop() {
        this.stream?.getTracks().forEach((track) => track.stop());
        this.stream = null;
        this.video.srcObject = null;
    }
    async capture(session, hooks) {
        this.assertReady();
        await this.waitForReadiness(hooks);
        const frames = [];
        const startedAtUnixMs = Date.now();
        const startTime = performance.now();
        const deadline = startTime + session.captureDurationMs;
        hooks.onStatus("Mantenha o rosto centralizado e olhe para a câmera");
        while (performance.now() < deadline) {
            const currentTime = performance.now();
            const elapsed = currentTime - startTime;
            const progress = Math.min(elapsed / session.captureDurationMs, 1);
            const challengeIndex = Math.min(Math.floor(progress * session.illuminationPattern.length), session.illuminationPattern.length - 1);
            const light = session.illuminationPattern[challengeIndex] ?? 0.5;
            hooks.onLightChange(light);
            hooks.onProgress(progress);
            await this.delay(session.illuminationSettleMs);
            const capturedImage = this.captureCurrentFrame();
            hooks.onQualityChange(capturedImage.quality);
            frames.push({
                imageBase64: capturedImage.imageBase64,
                timestampMs: Math.round(performance.now() - startTime),
                challengeIndex,
                clientLight: light,
                clientQuality: {
                    brightness: capturedImage.quality.brightness,
                    contrast: capturedImage.quality.contrast,
                    sharpness: capturedImage.quality.sharpness,
                },
            });
            const spent = performance.now() - currentTime;
            await this.delay(Math.max(0, session.sampleIntervalMs - spent));
        }
        hooks.onLightChange(0);
        hooks.onProgress(1);
        hooks.onStatus("Analisando captura...");
        return {
            frames,
            metadata: {
                sdkVersion: SDK_VERSION,
                startedAtUnixMs,
                endedAtUnixMs: Date.now(),
                frameWidth: this.canvas.width,
                frameHeight: this.canvas.height,
                userAgent: navigator.userAgent,
                platform: navigator.platform || "unknown",
            },
        };
    }
    async waitForReadiness(hooks) {
        const deadline = performance.now() + READINESS_TIMEOUT_MS;
        let goodSamples = 0;
        let lastIssue = "Não foi possível validar a qualidade da câmera.";
        hooks.onStatus("Verificando iluminação e nitidez...");
        while (performance.now() < deadline) {
            const quality = this.analyzeCurrentFrame();
            hooks.onQualityChange(quality);
            if (quality.acceptable) {
                goodSamples++;
                if (goodSamples >= REQUIRED_GOOD_SAMPLES) {
                    return;
                }
            }
            else {
                goodSamples = 0;
                lastIssue = quality.issue ?? lastIssue;
                hooks.onStatus(lastIssue);
            }
            await this.delay(READINESS_SAMPLE_MS);
        }
        throw new Error(lastIssue);
    }
    analyzeCurrentFrame() {
        this.drawCurrentFrame();
        const imageData = this.context.getImageData(0, 0, this.canvas.width, this.canvas.height);
        return this.qualityAnalyzer.analyze(imageData);
    }
    captureCurrentFrame() {
        this.drawCurrentFrame();
        const imageData = this.context.getImageData(0, 0, this.canvas.width, this.canvas.height);
        return {
            imageBase64: this.canvas.toDataURL("image/jpeg", 0.80),
            quality: this.qualityAnalyzer.analyze(imageData),
        };
    }
    drawCurrentFrame() {
        this.assertReady();
        const sourceWidth = this.video.videoWidth;
        const sourceHeight = this.video.videoHeight;
        if (sourceWidth === 0 || sourceHeight === 0) {
            throw new Error("Camera returned an empty frame");
        }
        const outputWidth = Math.min(640, sourceWidth);
        const outputHeight = Math.round((outputWidth / sourceWidth) * sourceHeight);
        if (this.canvas.width !== outputWidth || this.canvas.height !== outputHeight) {
            this.canvas.width = outputWidth;
            this.canvas.height = outputHeight;
        }
        this.context.drawImage(this.video, 0, 0, outputWidth, outputHeight);
    }
    assertReady() {
        if (!this.stream || this.video.readyState < HTMLMediaElement.HAVE_CURRENT_DATA) {
            throw new Error("Camera is not ready");
        }
    }
    async delay(milliseconds) {
        await new Promise((resolve) => window.setTimeout(resolve, milliseconds));
    }
}

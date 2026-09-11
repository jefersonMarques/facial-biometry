export class CameraCapture {
    video;
    stream = null;
    canvas;
    context;
    constructor(video) {
        this.video = video;
        this.canvas = document.createElement("canvas");
        const context = this.canvas.getContext("2d", { alpha: false });
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
        if (!this.stream || this.video.readyState < HTMLMediaElement.HAVE_CURRENT_DATA) {
            throw new Error("Camera is not ready");
        }
        const frames = [];
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
            await this.delay(65);
            frames.push({
                imageBase64: this.captureFrame(),
                timestampMs: Math.round(performance.now() - startTime),
                challengeIndex,
                clientLight: light,
            });
            const spent = performance.now() - currentTime;
            await this.delay(Math.max(0, session.sampleIntervalMs - spent));
        }
        hooks.onLightChange(0);
        hooks.onProgress(1);
        hooks.onStatus("Analisando captura...");
        return frames;
    }
    captureFrame() {
        const sourceWidth = this.video.videoWidth;
        const sourceHeight = this.video.videoHeight;
        if (sourceWidth === 0 || sourceHeight === 0) {
            throw new Error("Camera returned an empty frame");
        }
        const outputWidth = Math.min(640, sourceWidth);
        const outputHeight = Math.round((outputWidth / sourceWidth) * sourceHeight);
        this.canvas.width = outputWidth;
        this.canvas.height = outputHeight;
        this.context.drawImage(this.video, 0, 0, outputWidth, outputHeight);
        return this.canvas.toDataURL("image/jpeg", 0.78);
    }
    async delay(milliseconds) {
        await new Promise((resolve) => window.setTimeout(resolve, milliseconds));
    }
}
//# sourceMappingURL=camera-capture.js.map
export class GeometryWasmShadow {
    worker = null;
    state = "idle";
    summary = null;
    errorMessage = "";
    initialization = null;
    resolveInitialization = null;
    rejectInitialization = null;
    initialize() {
        if (this.state === "ready") {
            return Promise.resolve();
        }
        if (this.initialization) {
            return this.initialization;
        }
        this.state = "loading";
        this.errorMessage = "";
        this.initialization = new Promise((resolve, reject) => {
            this.resolveInitialization = resolve;
            this.rejectInitialization = reject;
            const worker = new Worker(new URL("./liveness-core-worker.js", import.meta.url), { type: "module", name: "faceproof-liveness-core" });
            this.worker = worker;
            worker.addEventListener("message", (event) => {
                this.onMessage(event.data);
            });
            worker.addEventListener("error", (event) => {
                this.fail(event.message || "WASM worker failed");
            });
            worker.postMessage({ type: "init" });
        });
        return this.initialization;
    }
    reset() {
        this.summary = null;
        this.worker?.postMessage({ type: "reset" });
    }
    push(phase, timestampMs, landmarks) {
        if (this.state !== "ready" || !this.worker) {
            return;
        }
        const xyz = new Float64Array(landmarks.length * 3);
        for (let index = 0; index < landmarks.length; index += 1) {
            const landmark = landmarks[index];
            if (!landmark) {
                return;
            }
            const offset = index * 3;
            xyz[offset] = landmark.x;
            xyz[offset + 1] = landmark.y;
            xyz[offset + 2] = landmark.z;
        }
        this.worker.postMessage({
            type: "sample",
            phase,
            timestampMs,
            landmarks: xyz,
        }, [xyz.buffer]);
    }
    snapshot() {
        return {
            state: this.state,
            summary: this.summary,
            errorMessage: this.errorMessage,
        };
    }
    dispose() {
        if (this.worker) {
            this.worker.postMessage({ type: "dispose" });
            this.worker.terminate();
            this.worker = null;
        }
        this.initialization = null;
        this.resolveInitialization = null;
        this.rejectInitialization = null;
        this.state = "idle";
        this.summary = null;
        this.errorMessage = "";
    }
    onMessage(message) {
        if (message.type === "ready") {
            this.state = "ready";
            this.resolveInitialization?.();
            this.resolveInitialization = null;
            this.rejectInitialization = null;
            return;
        }
        if (message.type === "summary") {
            this.summary = message.summary;
            return;
        }
        this.fail(message.message);
    }
    fail(message) {
        const error = new Error(message);
        this.state = "error";
        this.errorMessage = message;
        this.rejectInitialization?.(error);
        this.resolveInitialization = null;
        this.rejectInitialization = null;
        this.initialization = null;
    }
}

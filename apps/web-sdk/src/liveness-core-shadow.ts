import type {
    GeometryCapturePhase,
    GeometryLivenessSummary,
} from "./liveness-v2.js";

export type WasmShadowState = "idle" | "loading" | "ready" | "error";

export interface LocalFaceGuideMetrics {
    faceDetected: boolean;
    centerX: number;
    centerY: number;
    widthRatio: number;
    heightRatio: number;
    rollDegrees: number;
    faceSizeScore: number;
}

export interface WasmShadowDiagnostics {
    state: WasmShadowState;
    summary: GeometryLivenessSummary | null;
    guide: LocalFaceGuideMetrics | null;
    errorMessage: string;
}

type WorkerResponse =
    | { type: "ready" }
    | {
        type: "summary";
        summary: GeometryLivenessSummary;
        guide: LocalFaceGuideMetrics;
    }
    | { type: "error"; message: string };

export class GeometryWasmShadow {
    private worker: Worker | null = null;
    private state: WasmShadowState = "idle";
    private summary: GeometryLivenessSummary | null = null;
    private guide: LocalFaceGuideMetrics | null = null;
    private errorMessage = "";
    private initialization: Promise<void> | null = null;
    private resolveInitialization: (() => void) | null = null;
    private rejectInitialization: ((error: Error) => void) | null = null;

    public constructor(
        private readonly onUpdate?: (diagnostics: WasmShadowDiagnostics) => void,
    ) {}

    public initialize(): Promise<void> {
        if (this.state === "ready") {
            return Promise.resolve();
        }
        if (this.initialization) {
            return this.initialization;
        }

        this.state = "loading";
        this.errorMessage = "";

        this.initialization = new Promise<void>((resolve, reject) => {
            this.resolveInitialization = resolve;
            this.rejectInitialization = reject;

            const worker = new Worker(
                new URL("./liveness-core-worker.js", import.meta.url),
                { type: "module", name: "faceproof-liveness-core" },
            );
            this.worker = worker;
            worker.addEventListener("message", (event: MessageEvent<WorkerResponse>) => {
                this.onMessage(event.data);
            });
            worker.addEventListener("error", (event: ErrorEvent) => {
                this.fail(event.message || "WASM worker failed");
            });
            worker.postMessage({ type: "init" });
        });

        return this.initialization;
    }

    public reset(): void {
        this.summary = null;
        this.guide = null;
        this.worker?.postMessage({ type: "reset" });
        this.emitUpdate();
    }

    public push(
        phase: GeometryCapturePhase,
        timestampMs: number,
        landmarks: Array<{ x: number; y: number; z: number }>,
    ): void {
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

        this.worker.postMessage(
            {
                type: "sample",
                phase,
                timestampMs,
                landmarks: xyz,
            },
            [xyz.buffer],
        );
    }

    public snapshot(): WasmShadowDiagnostics {
        return {
            state: this.state,
            summary: this.summary,
            guide: this.guide,
            errorMessage: this.errorMessage,
        };
    }

    public dispose(): void {
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
        this.guide = null;
        this.errorMessage = "";
    }

    private onMessage(message: WorkerResponse): void {
        if (message.type === "ready") {
            this.state = "ready";
            this.resolveInitialization?.();
            this.resolveInitialization = null;
            this.rejectInitialization = null;
            this.emitUpdate();
            return;
        }

        if (message.type === "summary") {
            this.summary = message.summary;
            this.guide = message.guide;
            this.emitUpdate();
            return;
        }

        this.fail(message.message);
    }

    private fail(message: string): void {
        const error = new Error(message);
        this.state = "error";
        this.errorMessage = message;
        this.rejectInitialization?.(error);
        this.resolveInitialization = null;
        this.rejectInitialization = null;
        this.initialization = null;
        this.emitUpdate();
    }

    private emitUpdate(): void {
        this.onUpdate?.(this.snapshot());
    }
}

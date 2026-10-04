import type {
    CaptureProtocolMetadata,
    GuidedCapturePhase,
} from "./types.js";

export const CAPTURE_LIFECYCLE_EVENT = "faceproof:capture-lifecycle";
export const CAPTURE_PROTOCOL_VERSION = "1";

export type CaptureLifecycleState =
    | "idle"
    | GuidedCapturePhase
    | "submitting"
    | "complete"
    | "cancelled";

export interface CaptureLifecycleEventDetail {
    protocolVersion: typeof CAPTURE_PROTOCOL_VERSION;
    runId: string;
    sequence: number;
    state: CaptureLifecycleState;
    timestampMs: number;
    unixMs: number;
}

export class CaptureLifecycle {
    private runId = "";
    private sequence = 0;
    private startedAtUnixMs = 0;
    private farStartedAtUnixMs = 0;
    private nearStartedAtUnixMs = 0;
    private submittingAtUnixMs = 0;
    private state: CaptureLifecycleState = "idle";

    public begin(): void {
        this.runId = crypto.randomUUID();
        this.sequence = 0;
        this.startedAtUnixMs = Date.now();
        this.farStartedAtUnixMs = 0;
        this.nearStartedAtUnixMs = 0;
        this.submittingAtUnixMs = 0;
        this.state = "idle";
        this.emit();
    }

    public transition(state: Exclude<CaptureLifecycleState, "idle">): void {
        if (!this.runId) {
            throw new Error("Capture lifecycle has not started");
        }
        if (!isAllowedTransition(this.state, state)) {
            throw new Error(`Invalid capture lifecycle transition: ${this.state} -> ${state}`);
        }

        this.state = state;
        this.sequence += 1;
        const now = Date.now();

        if (state === "far") {
            this.farStartedAtUnixMs = now;
        } else if (state === "near") {
            this.nearStartedAtUnixMs = now;
        } else if (state === "submitting") {
            this.submittingAtUnixMs = now;
        }

        this.emit();
    }

    public cancel(): void {
        if (!this.runId || this.state === "complete" || this.state === "cancelled") {
            return;
        }
        this.transition("cancelled");
    }

    public protocolMetadata(): CaptureProtocolMetadata {
        if (
            !this.runId ||
            !this.startedAtUnixMs ||
            !this.farStartedAtUnixMs ||
            !this.nearStartedAtUnixMs ||
            !this.submittingAtUnixMs
        ) {
            throw new Error("Capture lifecycle is incomplete");
        }

        return {
            version: CAPTURE_PROTOCOL_VERSION,
            runId: this.runId,
            startedAtUnixMs: this.startedAtUnixMs,
            farStartedAtUnixMs: this.farStartedAtUnixMs,
            nearStartedAtUnixMs: this.nearStartedAtUnixMs,
            submittingAtUnixMs: this.submittingAtUnixMs,
        };
    }

    private emit(): void {
        const detail: CaptureLifecycleEventDetail = {
            protocolVersion: CAPTURE_PROTOCOL_VERSION,
            runId: this.runId,
            sequence: this.sequence,
            state: this.state,
            timestampMs: performance.now(),
            unixMs: Date.now(),
        };
        window.dispatchEvent(new CustomEvent(CAPTURE_LIFECYCLE_EVENT, { detail }));
    }
}

function isAllowedTransition(
    current: CaptureLifecycleState,
    next: Exclude<CaptureLifecycleState, "idle">,
): boolean {
    switch (current) {
    case "idle":
        return next === "far" || next === "cancelled";
    case "far":
        return next === "near" || next === "cancelled";
    case "near":
        return next === "submitting" || next === "cancelled";
    case "submitting":
        return next === "complete" || next === "cancelled";
    case "complete":
    case "cancelled":
        return false;
    }
}

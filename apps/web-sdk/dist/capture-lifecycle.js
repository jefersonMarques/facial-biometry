export const CAPTURE_LIFECYCLE_EVENT = "faceproof:capture-lifecycle";
export const CAPTURE_PROTOCOL_VERSION = "1";
export class CaptureLifecycle {
    runId = "";
    sequence = 0;
    startedAtUnixMs = 0;
    farStartedAtUnixMs = 0;
    nearStartedAtUnixMs = 0;
    submittingAtUnixMs = 0;
    state = "idle";
    begin(runId, protocolVersion) {
        if (protocolVersion !== CAPTURE_PROTOCOL_VERSION) {
            throw new Error(`Unsupported capture protocol version: ${protocolVersion}`);
        }
        if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(runId)) {
            throw new Error("Invalid server-issued capture run ID");
        }
        this.runId = runId;
        this.sequence = 0;
        this.startedAtUnixMs = Date.now();
        this.farStartedAtUnixMs = 0;
        this.nearStartedAtUnixMs = 0;
        this.submittingAtUnixMs = 0;
        this.state = "idle";
        this.emit();
    }
    transition(state) {
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
        }
        else if (state === "near") {
            this.nearStartedAtUnixMs = now;
        }
        else if (state === "submitting") {
            this.submittingAtUnixMs = now;
        }
        this.emit();
    }
    cancel() {
        if (!this.runId || this.state === "complete" || this.state === "cancelled") {
            return;
        }
        this.transition("cancelled");
    }
    protocolMetadata() {
        if (!this.runId ||
            !this.startedAtUnixMs ||
            !this.farStartedAtUnixMs ||
            !this.nearStartedAtUnixMs ||
            !this.submittingAtUnixMs) {
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
    emit() {
        const detail = {
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
function isAllowedTransition(current, next) {
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

import test from "node:test";
import assert from "node:assert/strict";

if (!globalThis.window) {
    globalThis.window = new EventTarget();
}
if (!globalThis.CustomEvent) {
    globalThis.CustomEvent = class CustomEvent extends Event {
        constructor(type, init = {}) {
            super(type);
            this.detail = init.detail;
        }
    };
}

const {
    CAPTURE_LIFECYCLE_EVENT,
    CaptureLifecycle,
} = await import("../dist/capture-lifecycle.js");

test("emits ordered lifecycle and protocol metadata", () => {
    const lifecycle = new CaptureLifecycle();
    const states = [];
    const handler = (event) => states.push(event.detail.state);
    window.addEventListener(CAPTURE_LIFECYCLE_EVENT, handler);

    lifecycle.begin("123e4567-e89b-42d3-a456-426614174000", "1");
    lifecycle.transition("far");
    lifecycle.transition("near");
    lifecycle.transition("submitting");

    const metadata = lifecycle.protocolMetadata();
    assert.equal(metadata.version, "1");
    assert.match(metadata.runId, /^[0-9a-f-]{36}$/);
    assert.ok(metadata.startedAtUnixMs <= metadata.farStartedAtUnixMs);
    assert.ok(metadata.farStartedAtUnixMs <= metadata.nearStartedAtUnixMs);
    assert.ok(metadata.nearStartedAtUnixMs <= metadata.submittingAtUnixMs);

    lifecycle.transition("complete");
    assert.deepEqual(states, ["idle", "far", "near", "submitting", "complete"]);

    window.removeEventListener(CAPTURE_LIFECYCLE_EVENT, handler);
});

test("rejects invalid phase transitions", () => {
    const lifecycle = new CaptureLifecycle();
    lifecycle.begin("123e4567-e89b-42d3-a456-426614174000", "1");

    assert.throws(
        () => lifecycle.transition("near"),
        /Invalid capture lifecycle transition/,
    );
});

test("rejects a browser-created or malformed run ID", () => {
    const lifecycle = new CaptureLifecycle();

    assert.throws(
        () => lifecycle.begin("browser-made-id", "1"),
        /Invalid server-issued capture run ID/,
    );
});

test("rejects an unsupported server capture protocol version", () => {
    const lifecycle = new CaptureLifecycle();

    assert.throws(
        () => lifecycle.begin("123e4567-e89b-42d3-a456-426614174000", "2"),
        /Unsupported capture protocol version/,
    );
});

const workerScope = self;
let moduleInstance = null;
let contextHandle = 0;
let resultPointer = 0;
let resultValueCount = 0;
workerScope.addEventListener("message", (event) => {
    void handleMessage(event.data);
});
async function handleMessage(message) {
    try {
        if (message.type === "init") {
            await initialize();
            workerScope.postMessage({ type: "ready" });
            return;
        }
        if (message.type === "dispose") {
            dispose();
            workerScope.close();
            return;
        }
        if (!moduleInstance || !contextHandle) {
            throw new Error("FaceProof liveness WASM is not initialized");
        }
        if (message.type === "reset") {
            ensureOk(moduleInstance._fp_wasm_reset(contextHandle), "reset");
            return;
        }
        pushSample(message);
        workerScope.postMessage({
            type: "summary",
            summary: readSummary(),
        });
    }
    catch (error) {
        workerScope.postMessage({
            type: "error",
            message: errorMessage(error),
        });
    }
}
async function initialize() {
    if (moduleInstance && contextHandle) {
        return;
    }
    const moduleUrl = new URL("../wasm/liveness-core/faceproof-liveness-core.js", import.meta.url).href;
    const imported = await import(moduleUrl);
    const factory = imported.default;
    if (typeof factory !== "function") {
        throw new Error("Invalid FaceProof liveness WASM module");
    }
    moduleInstance = await factory();
    contextHandle = moduleInstance._fp_wasm_create();
    if (!contextHandle) {
        throw new Error("Unable to create FaceProof liveness WASM context");
    }
    resultValueCount = moduleInstance._fp_wasm_result_value_count();
    if (resultValueCount !== 10) {
        throw new Error(`Unexpected FaceProof liveness result size: ${resultValueCount}`);
    }
    resultPointer = moduleInstance._malloc(resultValueCount * Float64Array.BYTES_PER_ELEMENT);
    if (!resultPointer) {
        throw new Error("Unable to allocate FaceProof liveness result buffer");
    }
}
function pushSample(message) {
    if (!moduleInstance || !contextHandle) {
        throw new Error("FaceProof liveness WASM is not initialized");
    }
    const landmarks = message.landmarks;
    if (landmarks.length === 0 || landmarks.length % 3 !== 0) {
        throw new Error("Invalid landmark buffer");
    }
    const bytes = landmarks.byteLength;
    const pointer = moduleInstance._malloc(bytes);
    if (!pointer) {
        throw new Error("Unable to allocate landmark buffer");
    }
    try {
        moduleInstance.HEAPF64.set(landmarks, pointer / Float64Array.BYTES_PER_ELEMENT);
        const phase = message.phase === "far" ? 0 : 1;
        ensureOk(moduleInstance._fp_wasm_push_landmarks_xyz(contextHandle, phase, pointer, landmarks.length / 3, message.timestampMs), "push landmarks");
    }
    finally {
        moduleInstance._free(pointer);
    }
}
function readSummary() {
    if (!moduleInstance || !contextHandle || !resultPointer) {
        throw new Error("FaceProof liveness WASM is not initialized");
    }
    ensureOk(moduleInstance._fp_wasm_write_result(contextHandle, resultPointer, resultValueCount), "read result");
    const start = resultPointer / Float64Array.BYTES_PER_ELEMENT;
    const values = moduleInstance.HEAPF64.subarray(start, start + resultValueCount);
    return {
        status: values[0] === 1 ? "experimental" : "insufficient",
        sampleCount: Math.trunc(values[1] ?? 0),
        farSamples: Math.trunc(values[2] ?? 0),
        nearSamples: Math.trunc(values[3] ?? 0),
        scaleRatio: values[4] ?? 0,
        transitionScore: values[5] ?? 0,
        perspectiveChange: values[6] ?? 0,
        depthChange: values[7] ?? 0,
        phaseStability: values[8] ?? 0,
        evidenceScore: values[9] ?? 0,
    };
}
function dispose() {
    if (!moduleInstance) {
        return;
    }
    if (resultPointer) {
        moduleInstance._free(resultPointer);
        resultPointer = 0;
    }
    if (contextHandle) {
        moduleInstance._fp_wasm_destroy(contextHandle);
        contextHandle = 0;
    }
    moduleInstance = null;
    resultValueCount = 0;
}
function ensureOk(code, operation) {
    if (code !== 0) {
        throw new Error(`FaceProof liveness WASM ${operation} failed with code ${code}`);
    }
}
function errorMessage(error) {
    return error instanceof Error ? error.message : String(error);
}

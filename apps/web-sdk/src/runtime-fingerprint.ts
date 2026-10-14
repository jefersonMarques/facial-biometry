import type { RuntimeFingerprint } from "./types.js";

const SDK_VERSION = "0.3.0";
const MEDIAPIPE_MANIFEST_URL = "/vendor/mediapipe/1.0.1/faceproof-runtime-manifest.json";
const LIVENESS_MANIFEST_URL = "/wasm/liveness-core/faceproof-liveness-core.manifest.json";

interface RuntimeManifestFile {
    path: string;
    bytes: number;
    sha256: string;
}

interface MediaPipeRuntimeManifest {
    schemaVersion: number;
    package: {
        version: string;
    };
    files: RuntimeManifestFile[];
}

interface LivenessRuntimeManifest {
    schemaVersion: number;
    version: string;
    files: RuntimeManifestFile[];
}

let cachedFingerprint: Promise<RuntimeFingerprint> | null = null;

export function getRuntimeFingerprint(): Promise<RuntimeFingerprint> {
    if (!cachedFingerprint) {
        cachedFingerprint = loadRuntimeFingerprint();
    }
    return cachedFingerprint;
}

async function loadRuntimeFingerprint(): Promise<RuntimeFingerprint> {
    const [mediaPipe, liveness] = await Promise.all([
        fetchJSON<MediaPipeRuntimeManifest>(MEDIAPIPE_MANIFEST_URL),
        fetchJSON<LivenessRuntimeManifest>(LIVENESS_MANIFEST_URL),
    ]);

    const vision = requiredFile(mediaPipe.files, "vision_bundle.mjs");
    const faceModel = requiredFile(mediaPipe.files, "models/face_landmarker.task");
    const livenessWasm = requiredFile(
        liveness.files,
        "faceproof-liveness-core.wasm",
    );

    const canonical = [
        SDK_VERSION,
        liveness.version,
        normalizeSHA256(livenessWasm.sha256),
        mediaPipe.package.version,
        normalizeSHA256(vision.sha256),
        normalizeSHA256(faceModel.sha256),
    ].join("|");

    return {
        sdkVersion: SDK_VERSION,
        livenessCoreVersion: liveness.version,
        livenessCoreWasmSha256: normalizeSHA256(livenessWasm.sha256),
        mediaPipeVersion: mediaPipe.package.version,
        mediaPipeVisionSha256: normalizeSHA256(vision.sha256),
        faceLandmarkerSha256: normalizeSHA256(faceModel.sha256),
        runtimeId: await sha256Text(canonical),
    };
}

async function fetchJSON<T>(url: string): Promise<T> {
    const response = await fetch(url, {
        method: "GET",
        cache: "no-store",
        credentials: "omit",
        referrerPolicy: "no-referrer",
    });
    if (!response.ok) {
        throw new Error(`Runtime manifest unavailable: ${url} (HTTP ${response.status})`);
    }
    return await response.json() as T;
}

function requiredFile(
    files: RuntimeManifestFile[],
    path: string,
): RuntimeManifestFile {
    const file = files.find((item) => item.path === path);
    if (!file) {
        throw new Error(`Runtime manifest is missing ${path}`);
    }
    if (!Number.isInteger(file.bytes) || file.bytes <= 0) {
        throw new Error(`Runtime manifest has invalid size for ${path}`);
    }
    return {
        ...file,
        sha256: normalizeSHA256(file.sha256),
    };
}

function normalizeSHA256(value: string): string {
    const normalized = value.trim().toLowerCase();
    if (!/^[0-9a-f]{64}$/.test(normalized)) {
        throw new Error("Runtime manifest contains an invalid SHA-256");
    }
    return normalized;
}

async function sha256Text(value: string): Promise<string> {
    const digest = await crypto.subtle.digest(
        "SHA-256",
        new TextEncoder().encode(value),
    );
    return Array.from(new Uint8Array(digest))
        .map((byte) => byte.toString(16).padStart(2, "0"))
        .join("");
}

export class BiometricClient {
    baseUrl;
    constructor(baseUrl) {
        this.baseUrl = baseUrl;
    }
    async createSession(subjectId, kind) {
        const resource = kind === "enrollment" ? "enrollments" : "verifications";
        return this.request(`/v1/biometric/${resource}`, {
            method: "POST",
            body: JSON.stringify({ subjectId }),
        });
    }
    async completeSession(session, capture) {
        if (session.kind === "identity") {
            throw new Error("Identity sessions must use completeIdentityCheck");
        }
        const resource = session.kind === "enrollment" ? "enrollments" : "verifications";
        return this.request(`/v1/biometric/${resource}/${session.sessionId}/complete`, {
            method: "POST",
            body: JSON.stringify({
                sessionToken: session.sessionToken,
                frames: capture.frames,
                metadata: capture.metadata,
            }),
        });
    }
    async getIdentityCheck(token) {
        return this.identityRequest("/v1/identity/check", token, { method: "GET" });
    }
    async uploadIdentityDocument(token, file, fileName, sha256) {
        const form = new FormData();
        form.append("file", file, fileName);
        const headers = new Headers();
        if (sha256) {
            headers.set("X-FaceProof-Upload-SHA256", sha256);
        }
        return this.identityRequest("/v1/identity/document", token, {
            method: "POST",
            headers,
            body: form,
        });
    }
    async guideIdentityFace(token, imageBase64) {
        return this.identityRequest("/v1/identity/guide", token, {
            method: "POST",
            body: JSON.stringify({ imageBase64 }),
        });
    }
    async createIdentitySession(token) {
        return this.identityRequest("/v1/identity/session", token, {
            method: "POST",
        });
    }
    async completeIdentityCheck(token, session, guidedFrames, runtime) {
        if (session.kind !== "identity") {
            throw new Error("Invalid identity capture session");
        }
        const form = new FormData();
        form.append("sessionId", session.sessionId);
        form.append("sessionToken", session.sessionToken);
        form.append("manifest", JSON.stringify({
            runtime,
            guidedFrames: guidedFrames.map((frame) => ({
                phase: frame.phase,
                clientQuality: frame.clientQuality,
            })),
        }));
        guidedFrames.forEach((frame, index) => {
            form.append("frame", frame.imageBlob, `${String(index).padStart(2, "0")}-${frame.phase}.jpg`);
        });
        return this.identityRequest("/v1/identity/complete", token, {
            method: "POST",
            body: form,
        });
    }
    async identityRequest(path, token, init) {
        const headers = new Headers(init.headers);
        headers.set("X-FaceProof-Identity-Token", token);
        return this.request(path, { ...init, headers });
    }
    async request(path, init) {
        const headers = new Headers(init.headers);
        if (init.body && !(init.body instanceof FormData) && !headers.has("Content-Type")) {
            headers.set("Content-Type", "application/json");
        }
        const response = await fetch(`${this.baseUrl}${path}`, {
            ...init,
            headers,
            cache: "no-store",
            credentials: "omit",
            referrerPolicy: "no-referrer",
        });
        const contentType = response.headers.get("content-type")?.toLowerCase() ?? "";
        if (!contentType.includes("application/json")) {
            const text = await response.text();
            const preview = text.trim().slice(0, 80);
            throw new Error(`API returned non-JSON response (HTTP ${response.status})${preview ? `: ${preview}` : ""}`);
        }
        const payload = await response.json();
        if (!response.ok) {
            const message = isErrorPayload(payload) ? payload.error : `HTTP ${response.status}`;
            throw new Error(message);
        }
        return payload;
    }
}
function isErrorPayload(value) {
    return typeof value === "object" && value !== null && "error" in value && typeof value.error === "string";
}

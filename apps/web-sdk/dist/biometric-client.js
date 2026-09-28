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
    async uploadIdentityDocument(token, file) {
        const form = new FormData();
        form.append("file", file, file.name);
        return this.identityRequest("/v1/identity/document", token, {
            method: "POST",
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
    async completeIdentityCheck(token, session, capture) {
        if (session.kind !== "identity") {
            throw new Error("Invalid identity capture session");
        }
        return this.identityRequest("/v1/identity/complete", token, {
            method: "POST",
            body: JSON.stringify({
                sessionId: session.sessionId,
                sessionToken: session.sessionToken,
                frames: capture.frames,
                guidedFrames: capture.guidedFrames ?? [],
                metadata: capture.metadata,
            }),
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

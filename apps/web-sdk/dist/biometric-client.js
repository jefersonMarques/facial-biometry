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
    async completeSession(session, frames) {
        const resource = session.kind === "enrollment" ? "enrollments" : "verifications";
        return this.request(`/v1/biometric/${resource}/${session.sessionId}/complete`, {
            method: "POST",
            body: JSON.stringify({
                sessionToken: session.sessionToken,
                frames,
            }),
        });
    }
    async request(path, init) {
        const response = await fetch(`${this.baseUrl}${path}`, {
            ...init,
            headers: {
                "Content-Type": "application/json",
                ...init.headers,
            },
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
//# sourceMappingURL=biometric-client.js.map
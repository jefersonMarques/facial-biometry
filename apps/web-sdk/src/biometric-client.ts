import type {
    BiometricSessionKind,
    CapturePackage,
    CompletionResponse,
    IdentityCheckStatus,
    IdentityCompletionResponse,
    IdentityDocumentResponse,
    IdentityGuideResult,
    SessionResponse,
} from "./types.js";

export class BiometricClient {
    public constructor(private readonly baseUrl: string) {}

    public async createSession(subjectId: string, kind: BiometricSessionKind): Promise<SessionResponse> {
        const resource = kind === "enrollment" ? "enrollments" : "verifications";
        return this.request<SessionResponse>(`/v1/biometric/${resource}`, {
            method: "POST",
            body: JSON.stringify({ subjectId }),
        });
    }

    public async completeSession(session: SessionResponse, capture: CapturePackage): Promise<CompletionResponse> {
        if (session.kind === "identity") {
            throw new Error("Identity sessions must use completeIdentityCheck");
        }
        const resource = session.kind === "enrollment" ? "enrollments" : "verifications";
        return this.request<CompletionResponse>(`/v1/biometric/${resource}/${session.sessionId}/complete`, {
            method: "POST",
            body: JSON.stringify({
                sessionToken: session.sessionToken,
                frames: capture.frames,
                guidedFrames: capture.guidedFrames ?? [],
                metadata: capture.metadata,
            }),
        });
    }

    public async getIdentityCheck(token: string): Promise<IdentityCheckStatus> {
        return this.identityRequest<IdentityCheckStatus>("/v1/identity/check", token, { method: "GET" });
    }

    public async uploadIdentityDocument(token: string, file: File): Promise<IdentityDocumentResponse> {
        const form = new FormData();
        form.append("file", file, file.name);
        return this.identityRequest<IdentityDocumentResponse>("/v1/identity/document", token, {
            method: "POST",
            body: form,
        });
    }

    public async guideIdentityFace(token: string, imageBase64: string): Promise<IdentityGuideResult> {
        return this.identityRequest<IdentityGuideResult>("/v1/identity/guide", token, {
            method: "POST",
            body: JSON.stringify({ imageBase64 }),
        });
    }

    public async createIdentitySession(token: string): Promise<SessionResponse> {
        return this.identityRequest<SessionResponse>("/v1/identity/session", token, {
            method: "POST",
        });
    }

    public async completeIdentityCheck(
        token: string,
        session: SessionResponse,
        capture: CapturePackage,
    ): Promise<IdentityCompletionResponse> {
        if (session.kind !== "identity") {
            throw new Error("Invalid identity capture session");
        }
        return this.identityRequest<IdentityCompletionResponse>("/v1/identity/complete", token, {
            method: "POST",
            body: JSON.stringify({
                sessionId: session.sessionId,
                sessionToken: session.sessionToken,
                frames: capture.frames,
                metadata: capture.metadata,
            }),
        });
    }

    private async identityRequest<T>(path: string, token: string, init: RequestInit): Promise<T> {
        const headers = new Headers(init.headers);
        headers.set("X-FaceProof-Identity-Token", token);
        return this.request<T>(path, { ...init, headers });
    }

    private async request<T>(path: string, init: RequestInit): Promise<T> {
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

        const payload: unknown = await response.json();
        if (!response.ok) {
            const message = isErrorPayload(payload) ? payload.error : `HTTP ${response.status}`;
            throw new Error(message);
        }
        return payload as T;
    }
}

function isErrorPayload(value: unknown): value is { error: string } {
    return typeof value === "object" && value !== null && "error" in value && typeof (value as { error?: unknown }).error === "string";
}

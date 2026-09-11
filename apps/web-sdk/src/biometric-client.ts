import type { CapturedFrame, CompletionResponse, SessionKind, SessionResponse } from "./types.js";

export class BiometricClient {
    public constructor(private readonly baseUrl: string) {}

    public async createSession(subjectId: string, kind: SessionKind): Promise<SessionResponse> {
        const resource = kind === "enrollment" ? "enrollments" : "verifications";
        return this.request<SessionResponse>(`/v1/biometric/${resource}`, {
            method: "POST",
            body: JSON.stringify({ subjectId }),
        });
    }

    public async completeSession(session: SessionResponse, frames: CapturedFrame[]): Promise<CompletionResponse> {
        const resource = session.kind === "enrollment" ? "enrollments" : "verifications";
        return this.request<CompletionResponse>(`/v1/biometric/${resource}/${session.sessionId}/complete`, {
            method: "POST",
            body: JSON.stringify({
                sessionToken: session.sessionToken,
                frames,
            }),
        });
    }

    private async request<T>(path: string, init: RequestInit): Promise<T> {
        const response = await fetch(`${this.baseUrl}${path}`, {
            ...init,
            headers: {
                "Content-Type": "application/json",
                ...init.headers,
            },
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

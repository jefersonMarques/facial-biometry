export type SessionKind = "enrollment" | "verification";
export type Decision = "approved" | "review" | "rejected";

export interface SessionResponse {
    sessionId: string;
    sessionToken: string;
    kind: SessionKind;
    expiresAt: string;
    captureDurationMs: number;
    sampleIntervalMs: number;
    illuminationSettleMs: number;
    illuminationPattern: number[];
}

export interface ClientFrameQuality {
    brightness: number;
    contrast: number;
    sharpness: number;
}

export interface CapturedFrame {
    imageBase64: string;
    timestampMs: number;
    challengeIndex: number;
    clientLight: number;
    clientQuality: ClientFrameQuality;
}

export interface CaptureMetadata {
    sdkVersion: string;
    startedAtUnixMs: number;
    endedAtUnixMs: number;
    frameWidth: number;
    frameHeight: number;
    userAgent: string;
    platform: string;
}

export interface CapturePackage {
    frames: CapturedFrame[];
    metadata: CaptureMetadata;
}

export interface FrameQualityAssessment extends ClientFrameQuality {
    acceptable: boolean;
    issue: string | null;
}

export interface SignalResult {
    score: number;
    status: string;
}

export interface QualityResult {
    score: number;
    facePresence: number;
    sharpness: number;
    brightness: number;
    faceSize: number;
    detectedFrames: number;
    processedFrames: number;
}

export interface CompletionResponse {
    sessionId: string;
    subjectId: string;
    kind: SessionKind;
    decision: Decision;
    livenessScore: number;
    similarity?: number;
    matchThreshold?: number;
    templateStored?: boolean;
    templateProvisional?: boolean;
    signals: {
        passivePad: SignalResult;
        temporalMotion: SignalResult;
        illumination: SignalResult;
    };
    quality: QualityResult;
    diagnostics: string[];
}

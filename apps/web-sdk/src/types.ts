export type SessionKind = "enrollment" | "verification" | "identity";
export type BiometricSessionKind = Exclude<SessionKind, "identity">;
export type Decision = "approved" | "review" | "rejected";
export type IdentityFlowType =
    | "cnh"
    | "face_enrollment"
    | "face_verification"
    | "photo_verification";
export type IdentityStatus =
    | "pending_document"
    | "processing_document"
    | "biometry_pending"
    | "approved"
    | "review"
    | "rejected"
    | "expired";

export interface SessionResponse {
    sessionId: string;
    sessionToken: string;
    kind: SessionKind;
    captureRunId: string;
    captureProtocolVersion: "1";
    expiresAt: string;
    captureDurationMs: number;
    sampleIntervalMs: number;
    illuminationSettleMs: number;
    illuminationPattern: number[];
}

export interface CaptureProtocolMetadata {
    version: "1";
    runId: string;
    startedAtUnixMs: number;
    farStartedAtUnixMs: number;
    nearStartedAtUnixMs: number;
    submittingAtUnixMs: number;
}

export interface RuntimeFingerprint {
    sdkVersion: string;
    livenessCoreVersion: string;
    livenessCoreWasmSha256: string;
    mediaPipeVersion: string;
    mediaPipeVisionSha256: string;
    faceLandmarkerSha256: string;
    runtimeId: string;
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

export type GuidedCapturePhase = "far" | "near";

export interface GuidedCapturedFrame {
    imageBlob: Blob;
    phase: GuidedCapturePhase;
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
    guidedFrames?: GuidedCapturedFrame[];
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

export interface IdentityGuideResult {
    faceDetected: boolean;
    confidence: number;
    centerX: number;
    centerY: number;
    widthRatio: number;
    heightRatio: number;
    rollDegrees: number;
    quality: QualityResult;
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

export interface GeometryTelemetry {
    runId: string;
    status: "insufficient" | "experimental" | string;
    sampleCount: number;
    farSamples: number;
    nearSamples: number;
    scaleRatio: number;
    transitionScore: number;
    perspectiveChange: number;
    depthChange: number;
    phaseStability: number;
    evidenceScore: number;
    wasmStatus: string;
    wasmMaxDelta?: number;
}

export interface NativeShadowComparison {
    status: "ok" | "partial" | "drift" | string;
    pythonFrames: number;
    nativeFrames: number;
    bboxMaxDeltaPx: number;
    confidenceMaxDelta: number;
    qualityMaxDelta: number;
    padComparedFrames: number;
    passivePadMaxDelta: number;
    selectedEmbeddingMinCosine: number;
    selectedEmbeddingMaxDelta: number;
    combinedEmbeddingCosine: number;
    combinedEmbeddingMaxDelta: number;
    errors?: string[];
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
        guidedCapture?: SignalResult;
    };
    quality: QualityResult;
    diagnostics: string[];
    nativeShadow?: NativeShadowComparison;
}

export interface IdentityCheckStatus {
    id: string;
    status: IdentityStatus;
    expiresAt: string;
    documentAccepted: boolean;
    canStartBiometry: boolean;
    decision?: Decision;
    flowType: IdentityFlowType;
    subjectId?: string;
    displayName?: string;
}

export interface IdentityDocumentDetails {
    signatureValid: boolean;
    vioSignatureValid: boolean;
    cpfMatch: boolean;
    freshnessValid: boolean;
    name?: string;
    cpf?: string;
    birthDate?: string;
    category?: string;
    expiryDate?: string;
    issuingUf?: string;
    referencePhotoDataUrl?: string;
}

export interface IdentityDocumentResponse {
    status: IdentityStatus;
    document: IdentityDocumentDetails;
}

export interface IdentityCompletionResponse {
    id: string;
    status: IdentityStatus;
    decision: Decision;
    flowType: IdentityFlowType;
    displayName?: string;
    templateStored?: boolean;
    livenessScore: number;
    similarity: number;
    frameSimilarities: number[];
    bestFrameIndex: number;
    matchThreshold: number;
    signals: {
        passivePad: SignalResult;
        temporalMotion: SignalResult;
        illumination: SignalResult;
        guidedCapture: SignalResult;
    };
    quality: QualityResult;
    diagnostics: string[];
    nativeShadow?: NativeShadowComparison;
    document: IdentityDocumentDetails;
}

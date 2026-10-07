import { resolveApiBaseUrl } from "./api-base-url.js";
import { BiometricClient } from "./biometric-client.js";
import { CameraCapture } from "./camera-capture.js";
import { CaptureLifecycle } from "./capture-lifecycle.js";
import {
    LocalCaptureGate,
    type LocalCaptureGateResult,
} from "./local-capture-gate.js";
import type { LocalFaceGuideMetrics } from "./liveness-core-shadow.js";
import { getRuntimeFingerprint } from "./runtime-fingerprint.js";
import type {
    GuidedCapturedFrame,
    GuidedCapturePhase,
    IdentityCheckStatus,
    IdentityCompletionResponse,
    IdentityDocumentDetails,
    NativeShadowComparison,
} from "./types.js";

const TOKEN_STORAGE_KEY = "faceproof.identity.token";
const DOCUMENT_PREVIEW_STORAGE_KEY = "faceproof.identity.document-preview";
const GUIDE_SAMPLE_MS = 260;
const GUIDE_READY_SAMPLES = 2;
const PHASE_CAPTURE_FRAMES = 4;
const PHASE_CAPTURE_SAMPLE_MS = 140;
const GUIDE_MAX_NETWORK_FAILURES = 5;
const AUTO_CAMERA_DELAY_MS = 250;
const QUALITY_FALLBACK_SAMPLES = 5;
const FIRST_CAPTURE_MAX_GUIDE_SAMPLES = 8;
const NEAR_HOLD_SECONDS = 3;
const LOCAL_GUIDE_MAX_AGE_MS = 650;
const LOCAL_GUIDE_FALLBACK_AFTER_MS = 1_500;
const CONFIRMED_VIEW_VISIBLE_MS = 3_000;

type GuideTone = "red" | "yellow" | "green";

type GuideSource = "local" | "server";

interface GuideGeometry {
    source: GuideSource;
    faceDetected: boolean;
    centerX: number;
    centerY: number;
    widthRatio: number;
    heightRatio: number;
    rollDegrees: number;
}

interface GuideAssessment {
    ready: boolean;
    captureReady: boolean;
    message: string;
    state: string;
    tone: GuideTone;
    proximityPercent: number;
}

interface PhaseGuideConfig {
    idealMin: number;
    idealMax: number;
    captureMin: number;
    captureMax: number;
    target: number;
}

const SERVER_PHASE_GUIDE: Record<GuidedCapturePhase, PhaseGuideConfig> = {
    far: {
        idealMin: 0.31,
        idealMax: 0.46,
        captureMin: 0.27,
        captureMax: 0.50,
        target: 0.385,
    },
    near: {
        idealMin: 0.51,
        idealMax: 0.69,
        captureMin: 0.47,
        captureMax: 0.73,
        target: 0.60,
    },
};

const LOCAL_PHASE_GUIDE: Record<GuidedCapturePhase, PhaseGuideConfig> = {
    far: {
        idealMin: 0.25,
        idealMax: 0.41,
        captureMin: 0.21,
        captureMax: 0.45,
        target: 0.33,
    },
    near: {
        idealMin: 0.42,
        idealMax: 0.61,
        captureMin: 0.38,
        captureMax: 0.66,
        target: 0.515,
    },
};

const client = new BiometricClient(resolveApiBaseUrl());
const documentPanel = requiredElement<HTMLElement>("documentPanel");
const biometryPanel = requiredElement<HTMLElement>("biometryPanel");
const finalPanel = requiredElement<HTMLElement>("finalPanel");
const fileInput = requiredElement<HTMLInputElement>("cnhFile");
const uploadButton = requiredElement<HTMLButtonElement>("uploadButton");
const fileDrop = fileInput.closest(".file-drop") as HTMLLabelElement;
const fileTitle = requiredElement<HTMLElement>("cnhFileTitle");
const fileName = requiredElement<HTMLElement>("cnhFileName");
const startButton = requiredElement<HTMLButtonElement>("startBiometryButton");
const documentStatus = requiredElement<HTMLDivElement>("documentStatus");
const biometricStatus = requiredElement<HTMLDivElement>("biometricStatus");
const resultPanel = requiredElement<HTMLDivElement>("identityResult");
const video = requiredElement<HTMLVideoElement>("camera");
const progressBar = requiredElement<HTMLDivElement>("progressBar");
const cameraState = requiredElement<HTMLDivElement>("cameraState");
const expiresText = requiredElement<HTMLSpanElement>("expiresText");
const faceGuide = requiredElement<HTMLDivElement>("faceGuide");
const guidePhaseText = requiredElement<HTMLSpanElement>("guidePhaseText");
const captureFlash = requiredElement<HTMLDivElement>("captureFlash");
const processingOverlay = requiredElement<HTMLDivElement>("processingOverlay");

const camera = new CameraCapture(video);
const localCaptureGate = new LocalCaptureGate();
const captureLifecycle = new CaptureLifecycle();
let identityToken = "";
let busy = false;
let autoBiometryScheduled = false;
let manualReadyResolver: (() => void) | null = null;
let currentBiometryPhase: GuidedCapturePhase = "far";
let documentDetails: IdentityDocumentDetails | null = null;
let latestLocalGuide: LocalFaceGuideMetrics | null = null;
let bestCaptureObjectURL = "";
let viewConfirmed = false;
let accumulatedVisibleMS = 0;
let visibleStartedAt: number | null = null;
let viewConfirmationTimer: number | null = null;

const handleLocalGuide = (event: Event): void => {
    const detail = (event as CustomEvent<{ guide?: LocalFaceGuideMetrics }>).detail;
    if (detail?.guide) {
        latestLocalGuide = detail.guide;
        localCaptureGate.push(detail.guide);
    }
};

window.addEventListener("faceproof:local-guide", handleLocalGuide);

void initialize();

uploadButton.addEventListener("click", () => void uploadDocument());
fileInput.addEventListener("change", () => {
    const file = fileInput.files?.[0];
    if (!file) {
        resetSelectedFileUI();
        return;
    }

    fileDrop.classList.add("file-selected");
    fileTitle.textContent = "CNH Digital selecionada";
    fileName.textContent = file.name;
    documentStatus.textContent = "Arquivo selecionado. Iniciando validação...";
    window.setTimeout(() => void uploadDocument(), 100);
});
startButton.addEventListener("click", () => {
    if (manualReadyResolver) {
        const resolve = manualReadyResolver;
        manualReadyResolver = null;
        startButton.hidden = true;
        startButton.disabled = true;
        resolve();
        return;
    }

    void runBiometry();
});
window.addEventListener("beforeunload", () => {
    window.removeEventListener("faceproof:local-guide", handleLocalGuide);
    document.removeEventListener("visibilitychange", handleViewVisibilityChange);
    if (viewConfirmationTimer !== null) {
        window.clearTimeout(viewConfirmationTimer);
        viewConfirmationTimer = null;
    }
    if (bestCaptureObjectURL) {
        URL.revokeObjectURL(bestCaptureObjectURL);
    }
    camera.stop();
});

async function initialize(): Promise<void> {
    identityToken = consumeIdentityToken();
    documentDetails = restoreDocumentPreview();
    if (!identityToken) {
        showFatal("Link de verificação inválido ou incompleto.");
        return;
    }

    try {
        const status = await client.getIdentityCheck(identityToken);
        renderStatus(status);
        startConfirmedViewTracking();
    } catch (error) {
        showFatal(errorMessage(error));
    }
}

function startConfirmedViewTracking(): void {
    if (viewConfirmed) {
        return;
    }

    document.addEventListener("visibilitychange", handleViewVisibilityChange);
    if (document.visibilityState === "visible") {
        visibleStartedAt = performance.now();
        scheduleViewConfirmation();
    }
}

function handleViewVisibilityChange(): void {
    if (viewConfirmed) {
        return;
    }

    if (document.visibilityState === "visible") {
        visibleStartedAt = performance.now();
        scheduleViewConfirmation();
        return;
    }

    accumulateVisibleTime();
    if (viewConfirmationTimer !== null) {
        window.clearTimeout(viewConfirmationTimer);
        viewConfirmationTimer = null;
    }
}

function accumulateVisibleTime(): void {
    if (visibleStartedAt === null) {
        return;
    }
    accumulatedVisibleMS += Math.max(0, performance.now() - visibleStartedAt);
    visibleStartedAt = null;
}

function scheduleViewConfirmation(): void {
    if (viewConfirmed || document.visibilityState !== "visible") {
        return;
    }

    if (viewConfirmationTimer !== null) {
        window.clearTimeout(viewConfirmationTimer);
    }

    const elapsedCurrent = visibleStartedAt === null
        ? 0
        : Math.max(0, performance.now() - visibleStartedAt);
    const remaining = Math.max(
        0,
        CONFIRMED_VIEW_VISIBLE_MS - accumulatedVisibleMS - elapsedCurrent,
    );

    viewConfirmationTimer = window.setTimeout(() => {
        void confirmVisibleView();
    }, remaining);
}

async function confirmVisibleView(): Promise<void> {
    viewConfirmationTimer = null;
    if (viewConfirmed || document.visibilityState !== "visible") {
        return;
    }

    accumulateVisibleTime();
    if (accumulatedVisibleMS < CONFIRMED_VIEW_VISIBLE_MS) {
        visibleStartedAt = performance.now();
        scheduleViewConfirmation();
        return;
    }

    try {
        await client.confirmIdentityView(
            identityToken,
            Math.round(accumulatedVisibleMS),
        );
        viewConfirmed = true;
        document.removeEventListener("visibilitychange", handleViewVisibilityChange);
    } catch {
        visibleStartedAt = performance.now();
        window.setTimeout(() => {
            if (!viewConfirmed && document.visibilityState === "visible") {
                scheduleViewConfirmation();
            }
        }, 1_000);
    }
}

async function uploadDocument(): Promise<void> {
    if (busy) {
        return;
    }

    const file = fileInput.files?.[0];
    if (!file) {
        documentStatus.textContent = "Selecione o PDF da sua CNH Digital.";
        return;
    }
    const hasPDFExtension = file.name.toLowerCase().endsWith(".pdf");
    const hasPDFMime = file.type.toLowerCase() === "application/pdf";
    if (!hasPDFExtension && !hasPDFMime) {
        documentStatus.textContent = "Envie o arquivo PDF original da CNH Digital.";
        return;
    }

    busy = true;
    uploadButton.disabled = true;
    fileInput.disabled = true;
    documentStatus.textContent = "Validando assinatura digital, QR Code e documento...";

    try {
        const prepared = await preparePDFUpload(file);
        documentStatus.textContent = `Validando CNH Digital (${formatBytes(prepared.size)})...`;
        const documentResponse = await client.uploadIdentityDocument(
            identityToken,
            prepared.blob,
            prepared.fileName,
            prepared.sha256,
        );
        documentDetails = documentResponse.document;
        storeDocumentPreview(documentResponse.document);
        const status = await client.getIdentityCheck(identityToken);
        renderStatus(status);
        documentStatus.textContent = "CNH Digital autenticada. Continue para a biometria.";
    } catch (error) {
        documentStatus.textContent = friendlyDocumentError(errorMessage(error));
        fileInput.value = "";
        resetSelectedFileUI();
    } finally {
        busy = false;
        if (!biometryPanel.hidden) {
            return;
        }
        uploadButton.disabled = false;
        fileInput.disabled = false;
    }
}



interface PreparedPDFUpload {
    blob: Blob;
    fileName: string;
    sha256: string;
    size: number;
}

async function preparePDFUpload(file: File): Promise<PreparedPDFUpload> {
    const bytes = new Uint8Array(await file.arrayBuffer());
    if (bytes.byteLength < 5) {
        throw new Error("O arquivo selecionado está vazio ou incompleto.");
    }

    const header = String.fromCharCode(...bytes.slice(0, 5));
    if (header !== "%PDF-") {
        throw new Error("O arquivo selecionado não contém um PDF original.");
    }

    const digest = await crypto.subtle.digest("SHA-256", bytes);
    const sha256 = Array.from(new Uint8Array(digest))
        .map((value) => value.toString(16).padStart(2, "0"))
        .join("");

    const safeFileName = file.name.toLowerCase().endsWith(".pdf")
        ? file.name
        : "cnh-digital.pdf";

    return {
        blob: new Blob([bytes], { type: "application/pdf" }),
        fileName: safeFileName,
        sha256,
        size: bytes.byteLength,
    };
}

function formatBytes(bytes: number): string {
    if (bytes < 1024) {
        return `${bytes} B`;
    }
    if (bytes < 1024 * 1024) {
        return `${(bytes / 1024).toFixed(1)} KB`;
    }
    return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
}

function storeDocumentPreview(details: IdentityDocumentDetails): void {
    if (!details.referencePhotoDataUrl) {
        return;
    }

    try {
        sessionStorage.setItem(DOCUMENT_PREVIEW_STORAGE_KEY, JSON.stringify({
            ...details,
            referencePhotoDataUrl: details.referencePhotoDataUrl,
        }));
    } catch {
        // A prévia é auxiliar; falha de sessionStorage não deve interromper a biometria.
    }
}

function restoreDocumentPreview(): IdentityDocumentDetails | null {
    try {
        const value = sessionStorage.getItem(DOCUMENT_PREVIEW_STORAGE_KEY);
        if (!value) {
            return null;
        }
        const parsed = JSON.parse(value) as IdentityDocumentDetails;
        return parsed && typeof parsed === "object" ? parsed : null;
    } catch {
        return null;
    }
}

function resetSelectedFileUI(): void {
    fileDrop.classList.remove("file-selected");
    fileTitle.textContent = "Selecionar CNH Digital";
    fileName.textContent = "Arquivo PDF";
}

async function runBiometry(): Promise<void> {
    if (busy) {
        return;
    }

    busy = true;
    startButton.hidden = true;
    startButton.disabled = true;
    startButton.textContent = "Estou pronto";
    processingOverlay.hidden = true;
    biometricStatus.textContent = "Iniciando câmera...";
    cameraState.textContent = "Iniciando câmera...";
    setProximityIndicator(0, "red");

    let completed = false;

    try {
        const runtimeFingerprint = await getRuntimeFingerprint();
        await camera.start();
        cameraState.textContent = "Câmera ativa";
        biometricStatus.textContent = "Posicione o rosto dentro do oval.";

        while (!completed) {
            const session = await client.createIdentitySession(identityToken);
            captureLifecycle.begin(
                session.captureRunId,
                session.captureProtocolVersion,
            );
            localCaptureGate.reset();
            currentBiometryPhase = "far";
            captureLifecycle.transition("far");
            const farRelaxedQuality = await waitForFacePhase("far");

            const farFrames = await capturePhase("far", farRelaxedQuality);
            localCaptureGate.lockFarReference();
            await showCaptureSuccess("Primeira captura concluída");

            currentBiometryPhase = "near";
            captureLifecycle.transition("near");
            const nearRelaxedQuality = await waitForFacePhase("near");
            const nearFrames = await capturePhase("near", nearRelaxedQuality);
            await showCaptureSuccess("Segunda captura concluída");

            biometricStatus.textContent = "Analisando seu rosto...";
            cameraState.textContent = "Captura concluída";
            guidePhaseText.textContent = "Analisando";
            processingOverlay.hidden = false;
            setProximityIndicator(100, "green");

            try {
                const capturedFrames = [...farFrames, ...nearFrames];
                captureLifecycle.transition("submitting");
                const result = await client.completeIdentityCheck(
                    identityToken,
                    session,
                    capturedFrames,
                    runtimeFingerprint,
                    captureLifecycle.protocolMetadata(),
                    loadGeometryTelemetry(session.captureRunId),
                );
                captureLifecycle.transition("complete");
                renderIdentityResult(result, capturedFrames);
                completed = true;
            } catch (error) {
                const message = errorMessage(error);
                if (isRecaptureRequired(message)) {
                    captureLifecycle.cancel();
                    processingOverlay.hidden = true;
                    setProximityIndicator(0, "red");
                    faceGuide.className = "face-guide phase-far";
                    guidePhaseText.textContent = "Captura 1 de 2";
                    biometricStatus.textContent = "A captura não ficou boa o suficiente. Vamos refazer sem desligar a câmera.";
                    await sleep(900);
                    continue;
                }
                throw error;
            }
        }
    } catch (error) {
        captureLifecycle.cancel();
        processingOverlay.hidden = true;
        const message = errorMessage(error);
        const infrastructureFailure = isInfrastructureBiometryError(message);
        biometricStatus.textContent = friendlyBiometryError(message);
        startButton.hidden = false;
        startButton.disabled = false;
        startButton.textContent = infrastructureFailure || currentBiometryPhase === "near"
            ? "Tentar novamente"
            : "Estou pronto";
        cameraState.textContent = infrastructureFailure ? "Falha temporária na análise" : "Câmera pausada";
        camera.stop();
    } finally {
        if (completed) {
            camera.stop();
        }
        busy = false;
    }
}

async function waitForFacePhase(phase: GuidedCapturePhase): Promise<boolean> {
    let stableSamples = 0;
    let networkFailures = 0;
    let qualityLimitedSamples = 0;
    let guideSamples = 0;
    const phaseStartedAt = performance.now();

    faceGuide.className = `face-guide phase-${phase}`;
    guidePhaseText.textContent = phase === "far" ? "Captura 1 de 2" : "Captura 2 de 2";
    biometricStatus.textContent = "Posicione o rosto no oval e siga as orientações.";
    setProximityIndicator(0, "red");
    startButton.hidden = true;
    startButton.disabled = true;

    const requiredStableSamples = phase === "far" ? 1 : GUIDE_READY_SAMPLES;

    while (stableSamples < requiredStableSamples) {
        const quality = camera.qualityForGuide();
        let guide = getFreshLocalGuide(phaseStartedAt);

        if (!guide) {
            if (performance.now() - phaseStartedAt < LOCAL_GUIDE_FALLBACK_AFTER_MS) {
                cameraState.textContent = "Analisando rosto";
                biometricStatus.textContent = "Preparando análise local...";
                await sleep(GUIDE_SAMPLE_MS);
                continue;
            }

            try {
                guide = await requestServerGuideFallback(quality);
                networkFailures = 0;
            } catch (error) {
                networkFailures++;
                if (networkFailures >= GUIDE_MAX_NETWORK_FAILURES) {
                    throw error;
                }
                cameraState.textContent = "Analisando rosto";
                biometricStatus.textContent = "Ajustando o enquadramento...";
                await sleep(GUIDE_SAMPLE_MS);
                continue;
            }
        }

        guideSamples++;
        const assessment = assessGuide(guide, phase);
        const localGate = guide.source === "local"
            ? localCaptureGate.evaluate(phase, phaseStartedAt)
            : null;

        if (localGate && (!localGate.stable || !localGate.nearScaleReady)) {
            stableSamples = 0;
            renderLocalGateIssue(localGate, assessment);
            await sleep(GUIDE_SAMPLE_MS);
            continue;
        }

        const clientQualityGood = quality.acceptable;
        const clientQualityUsable = isClientQualityUsable(quality);
        const qualityLimited = assessment.captureReady &&
            clientQualityUsable &&
            !clientQualityGood;

        if (
            phase === "far" &&
            guideSamples >= FIRST_CAPTURE_MAX_GUIDE_SAMPLES &&
            (!assessment.ready || !clientQualityGood)
        ) {
            cameraState.textContent = "Aguardando você";
            biometricStatus.textContent = assessment.captureReady
                ? "O enquadramento está utilizável. Toque em “Estou pronto” para capturar."
                : "Ajuste o rosto conforme a orientação e toque em “Estou pronto” quando estiver preparado.";
            faceGuide.classList.remove("guide-ready");
            if (assessment.captureReady) {
                faceGuide.classList.add("guide-near");
                setProximityIndicator(assessment.proximityPercent, "yellow");
            } else {
                faceGuide.classList.remove("guide-near");
                setProximityIndicator(assessment.proximityPercent, "red");
            }

            await waitForManualReady();
            biometricStatus.textContent = "Certo. Vamos tentar a captura.";
            return true;
        }

        if (assessment.captureReady && !clientQualityUsable) {
            qualityLimitedSamples = 0;
            stableSamples = 0;
            const lightingHint = clientQualityInstruction(quality);
            cameraState.textContent = lightingHint.state;
            biometricStatus.textContent = lightingHint.message;
            faceGuide.classList.remove("guide-ready", "guide-near");
            setProximityIndicator(assessment.proximityPercent, "red");
            await sleep(GUIDE_SAMPLE_MS);
            continue;
        }

        if (qualityLimited) {
            qualityLimitedSamples++;
            const lightingHint = clientQualityInstruction(quality);
            cameraState.textContent = lightingHint.state;
            biometricStatus.textContent = lightingHint.message;
            faceGuide.classList.remove("guide-ready");
            faceGuide.classList.add("guide-near");
            setProximityIndicator(assessment.proximityPercent, "yellow");

            if (qualityLimitedSamples >= QUALITY_FALLBACK_SAMPLES) {
                if (phase === "far") {
                    cameraState.textContent = "Pronto para tentar";
                    biometricStatus.textContent = "A qualidade está próxima do ideal. Se estiver pronto, continue.";
                    await waitForManualReady();
                    biometricStatus.textContent = "Certo. Vamos capturar e validar a qualidade no servidor.";
                    return true;
                }

                const heldStill = await holdStillForAutomaticCapture(phase);
                if (heldStill) {
                    return true;
                }
                qualityLimitedSamples = 0;
            }

            await sleep(GUIDE_SAMPLE_MS);
            continue;
        }

        qualityLimitedSamples = Math.max(0, qualityLimitedSamples - 1);

        if (!assessment.ready || phase === "near") {
            renderGuideAssessment(assessment);
        } else {
            cameraState.textContent = "Posição ideal";
            biometricStatus.textContent = "Capturando...";
            faceGuide.classList.remove("guide-near");
            faceGuide.classList.add("guide-ready");
            setProximityIndicator(100, "green");
        }

        if (assessment.ready) {
            stableSamples++;
        } else if (!assessment.captureReady) {
            stableSamples = 0;
        }

        if (stableSamples >= requiredStableSamples) {
            faceGuide.classList.remove("guide-near");
            faceGuide.classList.add("guide-ready");
            biometricStatus.textContent = "Posição ideal. Mantenha-se assim...";
            setProximityIndicator(100, "green");
            await sleep(180);
            return false;
        }

        await sleep(GUIDE_SAMPLE_MS);
    }

    return false;
}

async function holdStillForAutomaticCapture(phase: GuidedCapturePhase): Promise<boolean> {
    startButton.hidden = true;
    startButton.disabled = true;
    faceGuide.classList.remove("guide-ready");
    faceGuide.classList.add("guide-near");

    for (let seconds = NEAR_HOLD_SECONDS; seconds >= 1; seconds--) {
        biometricStatus.textContent = `Fique parado por ${seconds} segundo${seconds === 1 ? "" : "s"}...`;
        cameraState.textContent = "Mantenha-se parado";
        setProximityIndicator(100, "yellow");

        const quality = camera.qualityForGuide();
        if (!isClientQualityUsable(quality)) {
            const lightingHint = clientQualityInstruction(quality);
            cameraState.textContent = lightingHint.state;
            biometricStatus.textContent = lightingHint.message;
            setProximityIndicator(0, "red");
            return false;
        }

        let guide: GuideGeometry;
        try {
            guide = await resolveGuideWithFallback(quality);
        } catch {
            return false;
        }

        const assessment = assessGuide(guide, phase);
        const localGate = guide.source === "local"
            ? localCaptureGate.evaluate(phase)
            : null;
        if (localGate && (!localGate.stable || !localGate.nearScaleReady)) {
            renderLocalGateIssue(localGate, assessment);
            return false;
        }
        if (!assessment.captureReady) {
            renderGuideAssessment(assessment);
            return false;
        }

        await sleep(1_000);
    }

    faceGuide.classList.remove("guide-near");
    faceGuide.classList.add("guide-ready");
    cameraState.textContent = "Capturando";
    biometricStatus.textContent = "Capturando agora...";
    setProximityIndicator(100, "green");
    return true;
}

async function capturePhase(
    phase: GuidedCapturePhase,
    relaxedQuality = false,
): Promise<GuidedCapturedFrame[]> {
    const frames: GuidedCapturedFrame[] = [];

    biometricStatus.textContent = phase === "far"
        ? "Capturando a primeira imagem..."
        : "Capturando a segunda imagem...";

    const captureWidth = phase === "near" ? 960 : 640;
    const captureJPEGQuality = phase === "near" ? 0.92 : 0.86;

    while (frames.length < PHASE_CAPTURE_FRAMES) {
        const guideQuality = camera.qualityForGuide();
        let guide: GuideGeometry;
        try {
            guide = await resolveGuideWithFallback(guideQuality);
        } catch {
            relaxedQuality = await waitForFacePhase(phase);
            continue;
        }

        const assessment = assessGuide(guide, phase);
        const localGate = guide.source === "local"
            ? localCaptureGate.evaluate(phase)
            : null;
        if (localGate && (!localGate.stable || !localGate.nearScaleReady)) {
            frames.length = 0;
            renderLocalGateIssue(localGate, assessment);
            await sleep(GUIDE_SAMPLE_MS);
            continue;
        }

        renderGuideAssessment(assessment);

        if (!assessment.captureReady) {
            frames.length = 0;
            relaxedQuality = await waitForFacePhase(phase);
            continue;
        }

        const quality = camera.qualityForGuide(captureWidth);
        if (
            !isClientQualityUsable(quality) ||
            (!quality.acceptable && !relaxedQuality)
        ) {
            frames.length = 0;
            relaxedQuality = await waitForFacePhase(phase);
            continue;
        }

        const snapshot = await camera.captureBlob(captureWidth, captureJPEGQuality);
        frames.push({
            imageBlob: snapshot.imageBlob,
            phase,
            clientQuality: {
                brightness: quality.brightness,
                contrast: quality.contrast,
                sharpness: quality.sharpness,
            },
        });

        faceGuide.classList.remove("guide-near");
        faceGuide.classList.add("guide-ready");
        cameraState.textContent = "Capturando";
        biometricStatus.textContent = phase === "far"
            ? "Boa posição. Fazendo a primeira captura..."
            : "Boa posição. Fazendo a segunda captura...";
        setProximityIndicator(100, "green");
        await sleep(PHASE_CAPTURE_SAMPLE_MS);
    }

    return frames;
}

async function showCaptureSuccess(message: string): Promise<void> {
    biometricStatus.textContent = message;
    captureFlash.classList.add("visible");
    await sleep(220);
    captureFlash.classList.remove("visible");
    await sleep(280);
}

function assessGuide(
    guide: GuideGeometry,
    phase: GuidedCapturePhase,
): GuideAssessment {
    const config = guide.source === "local"
        ? LOCAL_PHASE_GUIDE[phase]
        : SERVER_PHASE_GUIDE[phase];
    const centerTargetY = guide.source === "local" ? 0.50 : 0.46;

    if (!guide.faceDetected) {
        return {
            ready: false,
            captureReady: false,
            message: "Posicione o rosto dentro do oval.",
            state: "Procurando rosto",
            tone: "red",
            proximityPercent: 0,
        };
    }

    const proximityPercent = Math.round(
        Math.max(6, Math.min(100, (guide.heightRatio / config.target) * 100)),
    );

    if (Math.abs(guide.rollDegrees) > 14) {
        return {
            ready: false,
            captureReady: false,
            message: "Olhe para a câmera e mantenha a cabeça reta.",
            state: "Ajuste a cabeça",
            tone: "red",
            proximityPercent,
        };
    }

    const horizontalOffset = Math.abs(guide.centerX - 0.5);
    const verticalOffset = Math.abs(guide.centerY - centerTargetY);
    if (horizontalOffset > 0.14 || verticalOffset > 0.16) {
        return {
            ready: false,
            captureReady: false,
            message: "Centralize o rosto dentro do oval.",
            state: "Centralize o rosto",
            tone: "red",
            proximityPercent,
        };
    }

    if (guide.heightRatio < config.captureMin) {
        return {
            ready: false,
            captureReady: false,
            message: "Aproxime o rosto.",
            state: "Muito longe",
            tone: "red",
            proximityPercent,
        };
    }

    if (guide.heightRatio < config.idealMin) {
        return {
            ready: false,
            captureReady: true,
            message: "Aproxime só um pouco.",
            state: "Quase na posição",
            tone: "yellow",
            proximityPercent,
        };
    }

    if (guide.heightRatio > config.captureMax) {
        return {
            ready: false,
            captureReady: false,
            message: "Afaste o rosto.",
            state: "Muito perto",
            tone: "red",
            proximityPercent: 100,
        };
    }

    if (guide.heightRatio > config.idealMax) {
        return {
            ready: false,
            captureReady: true,
            message: "Afaste só um pouco.",
            state: "Quase na posição",
            tone: "yellow",
            proximityPercent: 100,
        };
    }

    if (horizontalOffset > 0.10 || verticalOffset > 0.12) {
        return {
            ready: false,
            captureReady: true,
            message: "Quase lá. Centralize um pouco mais.",
            state: "Quase na posição",
            tone: "yellow",
            proximityPercent,
        };
    }

    return {
        ready: true,
        captureReady: true,
        message: "Posição ideal. Mantenha-se assim.",
        state: "Posição ideal",
        tone: "green",
        proximityPercent: 100,
    };
}

function getFreshLocalGuide(minTimestampMs = 0): GuideGeometry | null {
    const guide = latestLocalGuide;
    if (!guide) {
        return null;
    }
    if (guide.timestampMs < minTimestampMs) {
        return null;
    }
    if (performance.now() - guide.timestampMs > LOCAL_GUIDE_MAX_AGE_MS) {
        return null;
    }

    return {
        source: "local",
        faceDetected: guide.faceDetected,
        centerX: guide.centerX,
        centerY: guide.centerY,
        widthRatio: guide.widthRatio,
        heightRatio: guide.heightRatio,
        rollDegrees: guide.rollDegrees,
    };
}

async function resolveGuideWithFallback(
    clientQuality: { brightness: number; contrast: number; sharpness: number; acceptable: boolean; issue: string | null },
): Promise<GuideGeometry> {
    return getFreshLocalGuide() ?? requestServerGuideFallback(clientQuality);
}

async function requestServerGuideFallback(
    clientQuality: { brightness: number; contrast: number; sharpness: number; acceptable: boolean; issue: string | null },
): Promise<GuideGeometry> {
    const snapshot = camera.snapshotForGuide();
    const requestTimestampMs = performance.now();
    const guide = await client.guideIdentityFace(identityToken, snapshot.imageBase64);

    window.dispatchEvent(new CustomEvent("faceproof:server-guide", {
        detail: {
            timestampMs: requestTimestampMs,
            guide,
            clientQuality,
        },
    }));

    return {
        source: "server",
        faceDetected: guide.faceDetected && guide.confidence >= 0.72,
        centerX: guide.centerX,
        centerY: guide.centerY,
        widthRatio: guide.widthRatio,
        heightRatio: guide.heightRatio,
        rollDegrees: guide.rollDegrees,
    };
}

function isClientQualityUsable(
    quality: { brightness: number; contrast: number; sharpness: number },
): boolean {
    return quality.brightness >= 0.04 &&
        quality.brightness <= 0.98 &&
        quality.contrast >= 0.010 &&
        quality.sharpness >= 0.002;
}

function clientQualityInstruction(
    quality: { brightness: number; contrast: number; sharpness: number; issue: string | null },
): { message: string; state: string } {
    if (quality.brightness < 0.11) {
        return {
            message: "Está escuro. Aumente a iluminação do rosto.",
            state: "Pouca luz",
        };
    }
    if (quality.brightness > 0.94) {
        return {
            message: "Está muito claro. Evite luz forte diretamente no rosto.",
            state: "Luz excessiva",
        };
    }
    if (quality.sharpness < 0.008) {
        return {
            message: "Imagem pouco nítida. Mantenha o celular firme.",
            state: "Imagem desfocada",
        };
    }
    if (quality.contrast < 0.028) {
        return {
            message: "Melhore a iluminação do rosto.",
            state: "Pouco contraste",
        };
    }
    return {
        message: quality.issue ?? "Ajuste a câmera.",
        state: "Ajuste a câmera",
    };
}

function renderLocalGateIssue(
    gate: LocalCaptureGateResult,
    assessment: GuideAssessment,
): void {
    window.dispatchEvent(new CustomEvent("faceproof:local-capture-gate", {
        detail: gate,
    }));

    if (!gate.nearScaleReady) {
        cameraState.textContent = "Aproxime o rosto";
        biometricStatus.textContent = "A segunda captura precisa ficar claramente mais próxima da câmera.";
        faceGuide.classList.remove("guide-ready", "guide-near");
        setProximityIndicator(assessment.proximityPercent, "red");
        return;
    }

    cameraState.textContent = "Mantenha-se parado";
    biometricStatus.textContent = gate.sampleCount < 3 || gate.spanMs < 320
        ? "Aguarde um instante enquanto estabilizamos a captura..."
        : "Movimento detectado. Fique parado por um instante.";
    faceGuide.classList.remove("guide-ready");
    faceGuide.classList.add("guide-near");
    setProximityIndicator(assessment.proximityPercent, "yellow");
}

function renderGuideAssessment(assessment: GuideAssessment): void {
    biometricStatus.textContent = assessment.message;
    cameraState.textContent = assessment.state;
    faceGuide.classList.toggle("guide-ready", assessment.tone === "green");
    faceGuide.classList.toggle("guide-near", assessment.tone === "yellow");
    setProximityIndicator(assessment.proximityPercent, assessment.tone);
}

function setProximityIndicator(percent: number, tone: GuideTone): void {
    progressBar.style.width = `${Math.max(0, Math.min(100, percent))}%`;
    progressBar.classList.remove("proximity-red", "proximity-yellow", "proximity-green");
    progressBar.classList.add(`proximity-${tone}`);
}

function scheduleAutomaticBiometry(): void {
    if (autoBiometryScheduled) {
        return;
    }

    autoBiometryScheduled = true;
    window.setTimeout(() => {
        autoBiometryScheduled = false;
        if (biometryPanel.hidden) {
            return;
        }
        if (busy) {
            scheduleAutomaticBiometry();
            return;
        }
        void runBiometry();
    }, AUTO_CAMERA_DELAY_MS);
}

async function waitForManualReady(): Promise<void> {
    startButton.hidden = false;
    startButton.disabled = false;
    startButton.textContent = "Estou pronto";
    biometricStatus.textContent = "A imagem está quase boa. Se você já estiver bem posicionado, toque em “Estou pronto”.";

    await new Promise<void>((resolve) => {
        manualReadyResolver = resolve;
    });

    startButton.hidden = true;
    startButton.disabled = true;
}

function renderStatus(status: IdentityCheckStatus): void {
    expiresText.textContent = formatExpiration(status.expiresAt);
    processingOverlay.hidden = true;

    switch (status.status) {
    case "pending_document":
    case "processing_document":
        documentPanel.hidden = false;
        biometryPanel.hidden = true;
        finalPanel.hidden = true;
        break;
    case "biometry_pending":
        documentPanel.hidden = true;
        biometryPanel.hidden = false;
        finalPanel.hidden = true;
        startButton.hidden = true;
        startButton.disabled = true;
        startButton.textContent = "Estou pronto";
        biometricStatus.textContent = "Iniciando câmera...";
        cameraState.textContent = "Iniciando câmera...";
        scheduleAutomaticBiometry();
        break;
    case "approved":
    case "review":
    case "rejected":
        documentPanel.hidden = true;
        biometryPanel.hidden = true;
        finalPanel.hidden = false;
        renderFinalStatus(status.status);
        break;
    case "expired":
        showFatal("Este link de verificação expirou.");
        break;
    }
}

function renderIdentityResult(
    result: IdentityCompletionResponse,
    capturedFrames: GuidedCapturedFrame[],
): void {
    processingOverlay.hidden = true;
    documentPanel.hidden = true;
    biometryPanel.hidden = true;
    finalPanel.hidden = false;

    const decisionLabel = {
        approved: "IDENTIDADE CONFIRMADA",
        review: result.similarity >= result.matchThreshold
            ? "PROVA DE VIDA EM REVISÃO"
            : "ANÁLISE COMPLEMENTAR",
        rejected: "VERIFICAÇÃO NÃO APROVADA",
    }[result.decision];

    const details = {
        ...result.document,
        ...(documentDetails ?? {}),
    };
    if (bestCaptureObjectURL) {
        URL.revokeObjectURL(bestCaptureObjectURL);
        bestCaptureObjectURL = "";
    }
    const bestCaptureBlob = capturedFrames[result.bestFrameIndex]?.imageBlob;
    if (bestCaptureBlob) {
        bestCaptureObjectURL = URL.createObjectURL(bestCaptureBlob);
    }
    const bestCapture = bestCaptureObjectURL;
    const referencePhoto = documentDetails?.referencePhotoDataUrl ?? "";
    const match = faceMatchPresentation(result.similarity, result.matchThreshold);

    resultPanel.innerHTML = `
        <div class="result-header result-${escapeHtml(result.decision)}">
            <span>${escapeHtml(decisionLabel)}</span>
        </div>

        <section class="match-strength match-strength-${match.tone}">
            <span class="match-strength-eyebrow">Correspondência facial</span>
            <strong class="match-strength-label">${escapeHtml(match.label)}</strong>
            <div class="match-strength-subtitle">
                ${escapeHtml(match.summary)}
            </div>

            <div
                class="match-scale"
                style="--threshold-position: ${scorePosition(result.matchThreshold)}%"
                aria-label="Posição do score facial em relação ao limiar técnico"
            >
                <div class="match-scale-track">
                    <div
                        class="match-scale-threshold"
                        style="left: ${scorePosition(result.matchThreshold)}%"
                        title="Limiar técnico ${faceScore(result.matchThreshold)}"
                    ></div>
                    <div
                        class="match-scale-score match-scale-score-${match.tone}"
                        style="left: ${scorePosition(result.similarity)}%"
                        title="Score biométrico ${faceScore(result.similarity)}"
                    ></div>
                </div>
                <div class="match-scale-labels">
                    <span>-1</span>
                    <span class="match-scale-threshold-label">Limiar ${faceScore(result.matchThreshold)}</span>
                    <span>+1</span>
                </div>
            </div>

            <div class="match-margin-grid">
                <div>
                    <span>Margem sobre o limiar</span>
                    <strong>${signedFaceScore(match.margin)}</strong>
                </div>
                <div>
                    <span>Distância relativa</span>
                    <strong>${escapeHtml(match.relativeMarginLabel)}</strong>
                </div>
            </div>
        </section>

        <div class="identity-profile">
            <div class="identity-summary">
                <h3>Dados da CNH</h3>
                <div class="identity-data-grid">
                    ${dataLine("Nome", details.name)}
                    ${dataLine("CPF", details.cpf)}
                    ${dataLine("Nascimento", details.birthDate)}
                    ${dataLine("Categoria", details.category)}
                    ${dataLine("Validade", details.expiryDate)}
                    ${dataLine("UF de emissão", details.issuingUf)}
                </div>
            </div>

            <div class="face-comparison">
                ${photoCard("Foto da CNH", referencePhoto)}
                <div class="face-comparison-mark" aria-hidden="true">↔</div>
                ${photoCard("Melhor captura", bestCapture)}
            </div>
        </div>

        <div class="metrics-grid">
            ${metric("Prova de vida", percentage(result.livenessScore))}
            ${metric("Passive PAD", percentage(result.signals.passivePad.score))}
            ${metric("Qualidade", percentage(result.quality.score))}
            ${metric("Captura guiada", percentage(result.signals.guidedCapture.score))}
        </div>

        <details class="technical-details">
            <summary>Detalhes técnicos</summary>
            <div class="technical-details-body">
                <div class="technical-row">
                    <span>Score biométrico bruto</span>
                    <strong>${faceScore(result.similarity)}</strong>
                </div>
                <div class="technical-row">
                    <span>Limiar configurado</span>
                    <strong>${faceScore(result.matchThreshold)}</strong>
                </div>
                <div class="technical-row">
                    <span>Margem</span>
                    <strong>${signedFaceScore(match.margin)}</strong>
                </div>
                ${frameScoreLine(result.frameSimilarities)}
                ${nativeShadowDetails(result.nativeShadow, result.diagnostics)}
                <p>
                    O score facial é uma similaridade cosseno do modelo biométrico.
                    Ele não representa uma porcentagem de certeza ou probabilidade.
                </p>
            </div>
        </details>

        <div class="identity-checks">
            ${checkLine("Assinatura digital do PDF", result.document.signatureValid)}
            ${checkLine("Assinatura VIO", result.document.vioSignatureValid)}
            ${checkLine("CPF esperado", result.document.cpfMatch)}
            ${checkLine("Data mínima do documento", result.document.freshnessValid)}
        </div>
    `;
}

interface FaceMatchPresentation {
    label: string;
    summary: string;
    tone: "high" | "pass" | "below";
    margin: number;
    relativeMarginLabel: string;
}

function faceMatchPresentation(similarity: number, threshold: number): FaceMatchPresentation {
    const margin = similarity - threshold;
    const relativeMargin = threshold > 0
        ? (margin / threshold) * 100
        : 0;

    if (similarity < threshold) {
        return {
            label: "ABAIXO DO LIMIAR",
            summary: "O score facial não atingiu o limiar técnico configurado.",
            tone: "below",
            margin,
            relativeMarginLabel: `${Math.abs(relativeMargin).toFixed(0)}% abaixo do limiar`,
        };
    }

    if (relativeMargin >= 25) {
        return {
            label: "ALTA CORRESPONDÊNCIA",
            summary: "O score facial está confortavelmente acima do limiar técnico.",
            tone: "high",
            margin,
            relativeMarginLabel: `${relativeMargin.toFixed(0)}% acima do limiar`,
        };
    }

    return {
        label: "CORRESPONDÊNCIA ACIMA DO LIMIAR",
        summary: "O score facial superou o limiar técnico configurado.",
        tone: "pass",
        margin,
        relativeMarginLabel: `${relativeMargin.toFixed(0)}% acima do limiar`,
    };
}

function scorePosition(value: number): number {
    const clamped = Math.max(-1, Math.min(1, value));
    return ((clamped + 1) / 2) * 100;
}

function signedFaceScore(value: number): string {
    const prefix = value >= 0 ? "+" : "";
    return `${prefix}${value.toFixed(3)}`;
}

function dataLine(label: string, value?: string): string {
    if (!value) {
        return "";
    }
    return `<div class="identity-data-item"><span>${escapeHtml(label)}</span><strong>${escapeHtml(value)}</strong></div>`;
}

function photoCard(label: string, imageBase64: string): string {
    const body = imageBase64
        ? `<img src="${escapeHtml(imageBase64)}" alt="${escapeHtml(label)}">`
        : `<div class="face-photo-placeholder">Imagem indisponível</div>`;
    return `
        <figure class="face-photo-card">
            ${body}
            <figcaption>${escapeHtml(label)}</figcaption>
        </figure>
    `;
}

function nativeShadowDetails(
    shadow: NativeShadowComparison | undefined,
    diagnostics: string[],
): string {
    if (!isLocalDevelopmentHost()) {
        return "";
    }

    if (!shadow) {
        return nativeShadowDiagnosticFallback(diagnostics);
    }

    const padValue = shadow.padComparedFrames > 0
        ? `${shadow.padComparedFrames} frames · Δ máx. ${shadowDelta(shadow.passivePadMaxDelta)}`
        : "não comparado";
    const errors = shadow.errors?.length
        ? `<div class="technical-row"><span>Shadow warnings</span><strong>${escapeHtml(shadow.errors.join(" · "))}</strong></div>`
        : "";

    return `
        <div class="technical-row"><span>Secure Core C++ shadow</span><strong>${escapeHtml(shadow.status.toUpperCase())}</strong></div>
        <div class="technical-row"><span>Frames Python × C++</span><strong>${shadow.pythonFrames} × ${shadow.nativeFrames}</strong></div>
        <div class="technical-row"><span>YuNet bbox Δ máx.</span><strong>${shadow.bboxMaxDeltaPx.toFixed(3)} px</strong></div>
        <div class="technical-row"><span>YuNet confiança Δ máx.</span><strong>${shadowDelta(shadow.confidenceMaxDelta)}</strong></div>
        <div class="technical-row"><span>Qualidade Δ máx.</span><strong>${shadowDelta(shadow.qualityMaxDelta)}</strong></div>
        <div class="technical-row"><span>SFace cosine mín.</span><strong>${shadow.selectedEmbeddingMinCosine.toFixed(8)}</strong></div>
        <div class="technical-row"><span>SFace embedding Δ máx.</span><strong>${shadowDelta(shadow.selectedEmbeddingMaxDelta)}</strong></div>
        <div class="technical-row"><span>Embedding combinado cosine</span><strong>${shadow.combinedEmbeddingCosine.toFixed(8)}</strong></div>
        <div class="technical-row"><span>Embedding combinado Δ máx.</span><strong>${shadowDelta(shadow.combinedEmbeddingMaxDelta)}</strong></div>
        <div class="technical-row"><span>PAD nativo</span><strong>${escapeHtml(padValue)}</strong></div>
        ${errors}
    `;
}

function nativeShadowDiagnosticFallback(diagnostics: string[]): string {
    const diagnostic = diagnostics.find((item) =>
        item.startsWith("native shadow "),
    );
    if (!diagnostic) {
        return "";
    }

    const value = diagnostic.slice("native shadow ".length);
    return `<div class="technical-row"><span>Secure Core C++ shadow</span><strong>${escapeHtml(value)}</strong></div>`;
}

function shadowDelta(value: number): string {
    if (!Number.isFinite(value)) {
        return "—";
    }
    if (value === 0) {
        return "0";
    }
    return Math.abs(value) < 0.0001
        ? value.toExponential(2)
        : value.toFixed(6);
}

function isLocalDevelopmentHost(): boolean {
    return ["localhost", "127.0.0.1", "::1"].includes(
        window.location.hostname,
    );
}

function frameScoreLine(scores: number[]): string {
    if (!scores.length) {
        return "";
    }
    const values = scores.map((score) => faceScore(score)).join(" · ");
    return `<div class="frame-score-line"><span>Frames próximos usados no match</span><strong>${escapeHtml(values)}</strong></div>`;
}

function faceScore(value: number): string {
    return value.toFixed(3);
}

function renderFinalStatus(status: "approved" | "review" | "rejected"): void {
    const labels = {
        approved: "Identidade confirmada.",
        review: "A verificação será analisada.",
        rejected: "Não foi possível confirmar a identidade.",
    };
    resultPanel.innerHTML = `<div class="result-header result-${status}"><span>${labels[status]}</span></div>`;
}

function showFatal(message: string): void {
    camera.stop();
    documentPanel.hidden = true;
    biometryPanel.hidden = true;
    finalPanel.hidden = false;
    resultPanel.innerHTML = `<div class="fatal-message">${escapeHtml(message)}</div>`;
}

function consumeIdentityToken(): string {
    const fragment = new URLSearchParams(window.location.hash.slice(1));
    const fromFragment = fragment.get("identity")?.trim() ?? "";
    if (fromFragment) {
        const previousToken = sessionStorage.getItem(TOKEN_STORAGE_KEY)?.trim() ?? "";
        if (previousToken && previousToken !== fromFragment) {
            sessionStorage.removeItem(DOCUMENT_PREVIEW_STORAGE_KEY);
        }
        sessionStorage.setItem(TOKEN_STORAGE_KEY, fromFragment);
        history.replaceState(null, "", window.location.pathname + window.location.search);
        return fromFragment;
    }
    return sessionStorage.getItem(TOKEN_STORAGE_KEY)?.trim() ?? "";
}

function loadGeometryTelemetry(expectedRunId: string): import("./types.js").GeometryTelemetry | undefined {
    try {
        const value = sessionStorage.getItem("faceproof.liveness-v2.telemetry");
        if (!value) {
            return undefined;
        }
        const parsed = JSON.parse(value) as import("./types.js").GeometryTelemetry;
        if (!parsed || typeof parsed !== "object" || parsed.runId !== expectedRunId) {
            return undefined;
        }
        return parsed;
    } catch {
        return undefined;
    }
}

function friendlyDocumentError(message: string): string {
    if (message.includes("dependency") || message.includes("missing pdf") || message.includes("pdftoppm")) {
        return `O servidor ainda não está pronto para validar a CNH: ${message}`;
    }
    if (message.includes("reference photo")) {
        return "A CNH foi lida, mas não foi possível obter uma foto de referência válida.";
    }
    if (message.includes("minimum date")) {
        return "Este PDF da CNH foi gerado antes da data mínima exigida. Gere uma CNH Digital atualizada e tente novamente.";
    }
    if (message.includes("does not correspond")) {
        return "A CNH enviada não corresponde a esta verificação.";
    }
    const generic = "Não foi possível autenticar esta CNH Digital. Envie o PDF original gerado pelo aplicativo oficial.";
    if (
        ["localhost", "127.0.0.1", "::1"].includes(window.location.hostname) ||
        window.location.hostname.endsWith(".trycloudflare.com")
    ) {
        return `${generic} Detalhe: ${message}`;
    }
    return generic;
}

function friendlyBiometryError(message: string): string {
    if (isInfrastructureBiometryError(message)) {
        return "A análise demorou mais que o esperado ou a conexão com o servidor foi interrompida. Tente novamente.";
    }
    if (message.includes("attempt limit")) {
        return "O limite de tentativas biométricas desta verificação foi atingido.";
    }
    if (message.includes("expired")) {
        return "Este link de verificação expirou.";
    }
    if (isRecaptureRequired(message)) {
        return "A captura não ficou nítida o suficiente. Reenquadre o rosto e tente novamente.";
    }
    return message;
}

function isRecaptureRequired(message: string): boolean {
    return message.includes("recapture required") || message.includes("capture quality insufficient");
}

function isInfrastructureBiometryError(message: string): boolean {
    return message.includes("HTTP 502") ||
        message.includes("HTTP 503") ||
        message.includes("FaceProof API unavailable") ||
        message.includes("biometric engine failed");
}

function formatExpiration(value: string): string {
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
        return "";
    }
    return date.toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "short" });
}

function metric(label: string, value: string): string {
    return `<div class="metric"><span>${escapeHtml(label)}</span><strong>${escapeHtml(value)}</strong></div>`;
}

function checkLine(label: string, passed: boolean): string {
    return `<div class="identity-check"><span>${passed ? "✓" : "×"}</span><strong>${escapeHtml(label)}</strong></div>`;
}

function percentage(value: number): string {
    return `${(value * 100).toFixed(1)}%`;
}

function requiredElement<T extends HTMLElement>(id: string): T {
    const element = document.getElementById(id);
    if (!element) {
        throw new Error(`Missing required element: ${id}`);
    }
    return element as T;
}

function errorMessage(error: unknown): string {
    return error instanceof Error ? error.message : "Erro inesperado";
}

function escapeHtml(value: string): string {
    return value
        .replaceAll("&", "&amp;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;")
        .replaceAll('"', "&quot;")
        .replaceAll("'", "&#039;");
}

async function sleep(milliseconds: number): Promise<void> {
    await new Promise<void>((resolve) => window.setTimeout(resolve, milliseconds));
}
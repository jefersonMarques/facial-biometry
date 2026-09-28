import { resolveApiBaseUrl } from "./api-base-url.js";
import { BiometricClient } from "./biometric-client.js";
import { CameraCapture } from "./camera-capture.js";
import type {
    GuidedCapturedFrame,
    GuidedCapturePhase,
    IdentityCheckStatus,
    IdentityCompletionResponse,
    IdentityGuideResult,
} from "./types.js";

const TOKEN_STORAGE_KEY = "faceproof.identity.token";
const GUIDE_SAMPLE_MS = 420;
const GUIDE_READY_SAMPLES = 2;
const PHASE_CAPTURE_FRAMES = 4;
const PHASE_CAPTURE_SAMPLE_MS = 140;
const GUIDE_MAX_NETWORK_FAILURES = 5;

const client = new BiometricClient(resolveApiBaseUrl());
const documentPanel = requiredElement<HTMLElement>("documentPanel");
const biometryPanel = requiredElement<HTMLElement>("biometryPanel");
const finalPanel = requiredElement<HTMLElement>("finalPanel");
const fileInput = requiredElement<HTMLInputElement>("cnhFile");
const uploadButton = requiredElement<HTMLButtonElement>("uploadButton");
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

const camera = new CameraCapture(video);
let identityToken = "";
let busy = false;

void initialize();

uploadButton.addEventListener("click", () => void uploadDocument());
startButton.addEventListener("click", () => void runBiometry());
window.addEventListener("beforeunload", () => camera.stop());

async function initialize(): Promise<void> {
    identityToken = consumeIdentityToken();
    if (!identityToken) {
        showFatal("Link de verificação inválido ou incompleto.");
        return;
    }

    try {
        const status = await client.getIdentityCheck(identityToken);
        renderStatus(status);
    } catch (error) {
        showFatal(errorMessage(error));
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
    if (file.type && file.type !== "application/pdf") {
        documentStatus.textContent = "Envie o arquivo PDF original da CNH Digital.";
        return;
    }

    busy = true;
    uploadButton.disabled = true;
    fileInput.disabled = true;
    documentStatus.textContent = "Validando assinatura digital, QR Code e documento...";

    try {
        await client.uploadIdentityDocument(identityToken, file);
        const status = await client.getIdentityCheck(identityToken);
        renderStatus(status);
        documentStatus.textContent = "CNH Digital autenticada. Continue para a biometria.";
    } catch (error) {
        documentStatus.textContent = friendlyDocumentError(errorMessage(error));
        fileInput.value = "";
    } finally {
        busy = false;
        if (!biometryPanel.hidden) {
            return;
        }
        uploadButton.disabled = false;
        fileInput.disabled = false;
    }
}

async function runBiometry(): Promise<void> {
    if (busy) {
        return;
    }

    busy = true;
    startButton.disabled = true;
    startButton.textContent = "Verificação em andamento";
    biometricStatus.textContent = "Abrindo a câmera...";
    progressBar.style.width = "0%";

    let completed = false;

    try {
        await camera.start();
        cameraState.textContent = "Câmera ativa";

        while (!completed) {
            await waitForFacePhase("far");

            const session = await client.createIdentitySession(identityToken);
            const farFrames = await capturePhase("far");
            await showCaptureSuccess("Primeira captura concluída");

            await waitForFacePhase("near");
            const nearFrames = await capturePhase("near");
            await showCaptureSuccess("Segunda captura concluída");

            biometricStatus.textContent = "Analisando sua identidade...";
            guidePhaseText.textContent = "Verificando";
            progressBar.style.width = "100%";

            try {
                const result = await client.completeIdentityCheck(
                    identityToken,
                    session,
                    [...farFrames, ...nearFrames],
                );
                renderIdentityResult(result);
                completed = true;
            } catch (error) {
                const message = errorMessage(error);
                if (isRecaptureRequired(message)) {
                    progressBar.style.width = "0%";
                    faceGuide.className = "face-guide phase-far";
                    guidePhaseText.textContent = "1 de 2 · Mais longe";
                    biometricStatus.textContent = "A captura não ficou boa o suficiente. Vamos refazer sem desligar a câmera.";
                    await sleep(900);
                    continue;
                }
                throw error;
            }
        }
    } catch (error) {
        biometricStatus.textContent = friendlyBiometryError(errorMessage(error));
        startButton.disabled = false;
        startButton.textContent = "Continuar verificação facial";
        cameraState.textContent = "Câmera pausada";
        camera.stop();
    } finally {
        if (completed) {
            camera.stop();
        }
        busy = false;
    }
}

async function waitForFacePhase(phase: GuidedCapturePhase): Promise<void> {
    let stableSamples = 0;
    let networkFailures = 0;

    faceGuide.className = `face-guide phase-${phase}`;
    guidePhaseText.textContent = phase === "far" ? "1 de 2 · Mais longe" : "2 de 2 · Mais perto";
    biometricStatus.textContent = phase === "far"
        ? "Posicione o rosto dentro do oval."
        : "Aproxime o rosto e encaixe-o no oval maior.";
    progressBar.style.width = "0%";

    while (stableSamples < GUIDE_READY_SAMPLES) {
        const snapshot = camera.snapshotForGuide();

        if (!snapshot.quality.acceptable) {
            stableSamples = 0;
            faceGuide.classList.remove("guide-ready");
            cameraState.textContent = snapshot.quality.issue ?? "Ajuste a câmera";
            biometricStatus.textContent = snapshot.quality.issue ?? "Melhore a imagem para continuar.";
            progressBar.style.width = "0%";
            await sleep(GUIDE_SAMPLE_MS);
            continue;
        }

        let guide: IdentityGuideResult;
        try {
            guide = await client.guideIdentityFace(identityToken, snapshot.imageBase64);
            networkFailures = 0;
        } catch (error) {
            networkFailures++;
            if (networkFailures >= GUIDE_MAX_NETWORK_FAILURES) {
                throw error;
            }
            biometricStatus.textContent = "Ajustando o enquadramento...";
            await sleep(GUIDE_SAMPLE_MS);
            continue;
        }

        const instruction = guideInstruction(guide, phase);
        cameraState.textContent = guide.faceDetected ? "Rosto detectado" : "Procurando rosto";

        if (!instruction.ready) {
            stableSamples = 0;
            faceGuide.classList.remove("guide-ready");
            biometricStatus.textContent = instruction.message;
            progressBar.style.width = "0%";
            await sleep(GUIDE_SAMPLE_MS);
            continue;
        }

        stableSamples++;
        faceGuide.classList.add("guide-ready");
        biometricStatus.textContent = "Perfeito. Mantenha-se assim...";
        progressBar.style.width = `${Math.round((stableSamples / GUIDE_READY_SAMPLES) * 100)}%`;
        await sleep(GUIDE_SAMPLE_MS);
    }
}

async function capturePhase(phase: GuidedCapturePhase): Promise<GuidedCapturedFrame[]> {
    const frames: GuidedCapturedFrame[] = [];

    biometricStatus.textContent = phase === "far"
        ? "Capturando a primeira imagem..."
        : "Capturando a segunda imagem...";
    progressBar.style.width = "0%";

    while (frames.length < PHASE_CAPTURE_FRAMES) {
        const snapshot = camera.snapshotForGuide(480, 0.82);

        if (!snapshot.quality.acceptable) {
            frames.length = 0;
            faceGuide.classList.remove("guide-ready");
            biometricStatus.textContent = snapshot.quality.issue ?? "Mantenha o rosto imóvel.";
            await waitForFacePhase(phase);
            continue;
        }

        const guide = await client.guideIdentityFace(identityToken, snapshot.imageBase64);
        const instruction = guideInstruction(guide, phase);

        if (!instruction.ready) {
            frames.length = 0;
            faceGuide.classList.remove("guide-ready");
            biometricStatus.textContent = instruction.message;
            await waitForFacePhase(phase);
            continue;
        }

        faceGuide.classList.add("guide-ready");
        frames.push({
            imageBase64: snapshot.imageBase64,
            phase,
            clientQuality: {
                brightness: snapshot.quality.brightness,
                contrast: snapshot.quality.contrast,
                sharpness: snapshot.quality.sharpness,
            },
        });
        progressBar.style.width = `${Math.round((frames.length / PHASE_CAPTURE_FRAMES) * 100)}%`;
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

function guideInstruction(
    guide: IdentityGuideResult,
    phase: GuidedCapturePhase,
): { ready: boolean; message: string } {
    if (!guide.faceDetected || guide.confidence < 0.82) {
        return { ready: false, message: "Posicione seu rosto dentro do oval." };
    }

    if (Math.abs(guide.rollDegrees) > 12) {
        return { ready: false, message: "Mantenha a cabeça reta e olhe para a câmera." };
    }

    const centered = Math.abs(guide.centerX - 0.5) <= 0.11 && Math.abs(guide.centerY - 0.46) <= 0.13;
    if (!centered) {
        return { ready: false, message: "Centralize o rosto no oval." };
    }

    if (phase === "far") {
        if (guide.heightRatio < 0.26) {
            return { ready: false, message: "Aproxime-se um pouco." };
        }
        if (guide.heightRatio > 0.42) {
            return { ready: false, message: "Afaste-se um pouco." };
        }
    } else {
        if (guide.heightRatio < 0.46) {
            return { ready: false, message: "Aproxime o rosto até preencher o oval." };
        }
        if (guide.heightRatio > 0.70) {
            return { ready: false, message: "Afaste-se só um pouco." };
        }
    }

    if (guide.quality.score < 0.38) {
        if (guide.quality.sharpness < 0.30) {
            return { ready: false, message: "Imagem pouco nítida. Mantenha o aparelho firme." };
        }
        return { ready: false, message: "Melhore a iluminação do rosto." };
    }

    return { ready: true, message: "Perfeito. Mantenha-se assim..." };
}

function renderStatus(status: IdentityCheckStatus): void {
    expiresText.textContent = formatExpiration(status.expiresAt);

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
        startButton.disabled = false;
        startButton.textContent = "Iniciar verificação facial";
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

function renderIdentityResult(result: IdentityCompletionResponse): void {
    documentPanel.hidden = true;
    biometryPanel.hidden = true;
    finalPanel.hidden = false;

    const decisionLabel = {
        approved: "IDENTIDADE CONFIRMADA",
        review: "ANÁLISE NECESSÁRIA",
        rejected: "VERIFICAÇÃO NÃO APROVADA",
    }[result.decision];

    resultPanel.innerHTML = `
        <div class="result-header result-${escapeHtml(result.decision)}">
            <span>${escapeHtml(decisionLabel)}</span>
            <strong>${percentage(result.similarity)}</strong>
        </div>
        <div class="metrics-grid">
            ${metric("Prova de vida", percentage(result.livenessScore))}
            ${metric("Rosto × CNH", percentage(result.similarity))}
            ${metric("Passive PAD", percentage(result.signals.passivePad.score))}
            ${metric("Qualidade", percentage(result.quality.score))}
            ${metric("Captura guiada", percentage(result.signals.guidedCapture.score))}
            ${metric("Presença facial", percentage(result.quality.facePresence))}
        </div>
        <div class="identity-checks">
            ${checkLine("Assinatura digital do PDF", result.document.signatureValid)}
            ${checkLine("Assinatura VIO", result.document.vioSignatureValid)}
            ${checkLine("CPF esperado", result.document.cpfMatch)}
            ${checkLine("Data mínima do documento", result.document.freshnessValid)}
        </div>
    `;
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
        sessionStorage.setItem(TOKEN_STORAGE_KEY, fromFragment);
        history.replaceState(null, "", window.location.pathname + window.location.search);
        return fromFragment;
    }
    return sessionStorage.getItem(TOKEN_STORAGE_KEY)?.trim() ?? "";
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
    if (["localhost", "127.0.0.1", "::1"].includes(window.location.hostname)) {
        return `${generic} Detalhe: ${message}`;
    }
    return generic;
}

function friendlyBiometryError(message: string): string {
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

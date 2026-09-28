import { resolveApiBaseUrl } from "./api-base-url.js";
import { BiometricClient } from "./biometric-client.js";
import { CameraCapture } from "./camera-capture.js";
import type { FrameQualityAssessment, IdentityCheckStatus, IdentityCompletionResponse } from "./types.js";

const TOKEN_STORAGE_KEY = "faceproof.identity.token";

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
const lightLayer = requiredElement<HTMLDivElement>("lightLayer");
const progressBar = requiredElement<HTMLDivElement>("progressBar");
const cameraState = requiredElement<HTMLDivElement>("cameraState");
const expiresText = requiredElement<HTMLSpanElement>("expiresText");

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
    biometricStatus.textContent = "Preparando câmera...";
    progressBar.style.width = "0%";

    try {
        await camera.start();
        cameraState.textContent = "Câmera pronta";

        const session = await client.createIdentitySession(identityToken);
        const capture = await camera.capture(session, {
            onProgress(progress) {
                progressBar.style.width = `${Math.round(progress * 100)}%`;
            },
            onLightChange(value) {
                const opacity = Math.max(0, Math.min(0.72, value * 0.72));
                lightLayer.style.background = `rgba(255, 255, 255, ${opacity.toFixed(3)})`;
            },
            onQualityChange(quality) {
                renderCameraQuality(quality);
            },
            onStatus(message) {
                biometricStatus.textContent = message;
            },
        });

        const result = await client.completeIdentityCheck(identityToken, session, capture);
        renderIdentityResult(result);
    } catch (error) {
        biometricStatus.textContent = friendlyBiometryError(errorMessage(error));
        startButton.disabled = false;
    } finally {
        lightLayer.style.background = "transparent";
        camera.stop();
        busy = false;
    }
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

function renderCameraQuality(quality: FrameQualityAssessment): void {
    cameraState.textContent = quality.acceptable ? "Qualidade de captura boa" : quality.issue ?? "Ajuste a câmera";
}

function showFatal(message: string): void {
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
    return "Não foi possível autenticar esta CNH Digital. Envie o PDF original gerado pelo aplicativo oficial.";
}

function friendlyBiometryError(message: string): string {
    if (message.includes("attempt limit")) {
        return "O limite de tentativas desta verificação foi atingido.";
    }
    if (message.includes("expired")) {
        return "Este link de verificação expirou.";
    }
    return message;
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

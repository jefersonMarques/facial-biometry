import { BiometricClient } from "./biometric-client.js";
import { CameraCapture } from "./camera-capture.js";
import type { CompletionResponse, SessionKind } from "./types.js";

const apiBaseUrl = new URLSearchParams(window.location.search).get("api") ?? "http://localhost:8080";
const client = new BiometricClient(apiBaseUrl);

const video = requiredElement<HTMLVideoElement>("camera");
const subjectInput = requiredElement<HTMLInputElement>("subjectId");
const enrollmentButton = requiredElement<HTMLButtonElement>("enrollButton");
const verificationButton = requiredElement<HTMLButtonElement>("verifyButton");
const progressBar = requiredElement<HTMLDivElement>("progressBar");
const statusText = requiredElement<HTMLDivElement>("statusText");
const resultPanel = requiredElement<HTMLDivElement>("resultPanel");
const lightLayer = requiredElement<HTMLDivElement>("lightLayer");
const cameraState = requiredElement<HTMLDivElement>("cameraState");

const camera = new CameraCapture(video);
let busy = false;

void initialize();

async function initialize(): Promise<void> {
    try {
        await camera.start();
        cameraState.textContent = "Câmera pronta";
        setControlsEnabled(true);
    } catch (error) {
        cameraState.textContent = "Câmera indisponível";
        statusText.textContent = errorMessage(error);
    }
}

enrollmentButton.addEventListener("click", () => void run("enrollment"));
verificationButton.addEventListener("click", () => void run("verification"));
window.addEventListener("beforeunload", () => camera.stop());

async function run(kind: SessionKind): Promise<void> {
    if (busy) {
        return;
    }

    const subjectId = subjectInput.value.trim();
    if (!subjectId) {
        statusText.textContent = "Informe um identificador para a pessoa.";
        subjectInput.focus();
        return;
    }

    busy = true;
    setControlsEnabled(false);
    resultPanel.hidden = true;
    progressBar.style.width = "0%";
    statusText.textContent = "Criando sessão segura...";

    try {
        const session = await client.createSession(subjectId, kind);
        const frames = await camera.capture(session, {
            onProgress(progress) {
                progressBar.style.width = `${Math.round(progress * 100)}%`;
            },
            onLightChange(value) {
                const opacity = Math.max(0, Math.min(0.72, value * 0.72));
                lightLayer.style.background = `rgba(255, 255, 255, ${opacity.toFixed(3)})`;
            },
            onStatus(message) {
                statusText.textContent = message;
            },
        });

        const result = await client.completeSession(session, frames);
        renderResult(result);
    } catch (error) {
        statusText.textContent = errorMessage(error);
        resultPanel.hidden = true;
    } finally {
        lightLayer.style.background = "transparent";
        busy = false;
        setControlsEnabled(true);
    }
}

function renderResult(result: CompletionResponse): void {
    const decisionLabel = {
        approved: "APROVADO",
        review: "REVISÃO",
        rejected: "REJEITADO",
    }[result.decision];

    statusText.textContent = result.kind === "enrollment"
        ? `Cadastro: ${decisionLabel}`
        : `Verificação: ${decisionLabel}`;

    const similarity = result.similarity === undefined ? "—" : percentage(result.similarity);
    const threshold = result.matchThreshold === undefined ? "—" : result.matchThreshold.toFixed(3);

    resultPanel.innerHTML = `
        <div class="result-header result-${escapeHtml(result.decision)}">
            <span>${escapeHtml(decisionLabel)}</span>
            <strong>${percentage(result.livenessScore)}</strong>
        </div>
        <div class="metrics-grid">
            ${metric("Liveness", percentage(result.livenessScore))}
            ${metric("Passive PAD", percentage(result.signals.passivePad.score))}
            ${metric("Movimento temporal", percentage(result.signals.temporalMotion.score))}
            ${metric("Resposta à luz", percentage(result.signals.illumination.score))}
            ${metric("Qualidade", percentage(result.quality.score))}
            ${metric("Presença facial", percentage(result.quality.facePresence))}
            ${metric("Similaridade 1:1", similarity)}
            ${metric("Threshold", threshold)}
        </div>
        ${result.diagnostics.length > 0
            ? `<div class="diagnostics">${result.diagnostics.map((message) => `<div>${escapeHtml(message)}</div>`).join("")}</div>`
            : ""}
    `;
    resultPanel.hidden = false;
}

function metric(label: string, value: string): string {
    return `<div class="metric"><span>${escapeHtml(label)}</span><strong>${escapeHtml(value)}</strong></div>`;
}

function percentage(value: number): string {
    return `${(value * 100).toFixed(1)}%`;
}

function setControlsEnabled(enabled: boolean): void {
    enrollmentButton.disabled = !enabled;
    verificationButton.disabled = !enabled;
    subjectInput.disabled = !enabled;
}

function requiredElement<T extends HTMLElement>(id: string): T {
    const element = document.getElementById(id);
    if (!element) {
        throw new Error(`Missing required element: ${id}`);
    }
    return element as T;
}

function errorMessage(error: unknown): string {
    return error instanceof Error ? error.message : "Unexpected error";
}

function escapeHtml(value: string): string {
    return value
        .replaceAll("&", "&amp;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;")
        .replaceAll('"', "&quot;")
        .replaceAll("'", "&#039;");
}

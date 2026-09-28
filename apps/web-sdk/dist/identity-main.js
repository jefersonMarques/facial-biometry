import { resolveApiBaseUrl } from "./api-base-url.js";
import { BiometricClient } from "./biometric-client.js";
import { CameraCapture } from "./camera-capture.js";
const TOKEN_STORAGE_KEY = "faceproof.identity.token";
const GUIDE_SAMPLE_MS = 360;
const GUIDE_READY_SAMPLES = 2;
const PHASE_CAPTURE_FRAMES = 4;
const PHASE_CAPTURE_SAMPLE_MS = 140;
const GUIDE_MAX_NETWORK_FAILURES = 5;
const AUTO_CAMERA_DELAY_MS = 250;
const AUTO_FACE_DETECTION_WINDOW_MS = 8_000;
const PHASE_GUIDE = {
    far: {
        idealMin: 0.34,
        idealMax: 0.44,
        captureMin: 0.31,
        captureMax: 0.47,
        target: 0.39,
    },
    near: {
        idealMin: 0.51,
        idealMax: 0.69,
        captureMin: 0.47,
        captureMax: 0.73,
        target: 0.60,
    },
};
const client = new BiometricClient(resolveApiBaseUrl());
const documentPanel = requiredElement("documentPanel");
const biometryPanel = requiredElement("biometryPanel");
const finalPanel = requiredElement("finalPanel");
const fileInput = requiredElement("cnhFile");
const uploadButton = requiredElement("uploadButton");
const fileDrop = fileInput.closest(".file-drop");
const fileTitle = requiredElement("cnhFileTitle");
const fileName = requiredElement("cnhFileName");
const startButton = requiredElement("startBiometryButton");
const documentStatus = requiredElement("documentStatus");
const biometricStatus = requiredElement("biometricStatus");
const resultPanel = requiredElement("identityResult");
const video = requiredElement("camera");
const progressBar = requiredElement("progressBar");
const cameraState = requiredElement("cameraState");
const expiresText = requiredElement("expiresText");
const faceGuide = requiredElement("faceGuide");
const guidePhaseText = requiredElement("guidePhaseText");
const captureFlash = requiredElement("captureFlash");
const camera = new CameraCapture(video);
let identityToken = "";
let busy = false;
let autoBiometryScheduled = false;
let manualReadyResolver = null;
let documentDetails = null;
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
window.addEventListener("beforeunload", () => camera.stop());
async function initialize() {
    identityToken = consumeIdentityToken();
    if (!identityToken) {
        showFatal("Link de verificação inválido ou incompleto.");
        return;
    }
    try {
        const status = await client.getIdentityCheck(identityToken);
        renderStatus(status);
    }
    catch (error) {
        showFatal(errorMessage(error));
    }
}
async function uploadDocument() {
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
        const documentResponse = await client.uploadIdentityDocument(identityToken, prepared.blob, prepared.fileName, prepared.sha256);
        documentDetails = documentResponse.document;
        const status = await client.getIdentityCheck(identityToken);
        renderStatus(status);
        documentStatus.textContent = "CNH Digital autenticada. Continue para a biometria.";
    }
    catch (error) {
        documentStatus.textContent = friendlyDocumentError(errorMessage(error));
        fileInput.value = "";
        resetSelectedFileUI();
    }
    finally {
        busy = false;
        if (!biometryPanel.hidden) {
            return;
        }
        uploadButton.disabled = false;
        fileInput.disabled = false;
    }
}
async function preparePDFUpload(file) {
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
function formatBytes(bytes) {
    if (bytes < 1024) {
        return `${bytes} B`;
    }
    if (bytes < 1024 * 1024) {
        return `${(bytes / 1024).toFixed(1)} KB`;
    }
    return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
}
function resetSelectedFileUI() {
    fileDrop.classList.remove("file-selected");
    fileTitle.textContent = "Selecionar CNH Digital";
    fileName.textContent = "Arquivo PDF";
}
async function runBiometry() {
    if (busy) {
        return;
    }
    busy = true;
    startButton.hidden = true;
    startButton.disabled = true;
    startButton.textContent = "Estou pronto";
    biometricStatus.textContent = "Iniciando câmera...";
    cameraState.textContent = "Iniciando câmera...";
    setProximityIndicator(0, "red");
    let completed = false;
    try {
        await camera.start();
        cameraState.textContent = "Câmera ativa";
        biometricStatus.textContent = "Posicione o rosto dentro do oval.";
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
            setProximityIndicator(100, "green");
            try {
                const capturedFrames = [...farFrames, ...nearFrames];
                const result = await client.completeIdentityCheck(identityToken, session, capturedFrames);
                renderIdentityResult(result, capturedFrames);
                completed = true;
            }
            catch (error) {
                const message = errorMessage(error);
                if (isRecaptureRequired(message)) {
                    setProximityIndicator(0, "red");
                    faceGuide.className = "face-guide phase-far";
                    guidePhaseText.textContent = "1 de 2 · Mais longe";
                    biometricStatus.textContent = "A captura não ficou boa o suficiente. Vamos refazer sem desligar a câmera.";
                    await sleep(900);
                    continue;
                }
                throw error;
            }
        }
    }
    catch (error) {
        biometricStatus.textContent = friendlyBiometryError(errorMessage(error));
        startButton.hidden = false;
        startButton.disabled = false;
        startButton.textContent = "Estou pronto";
        cameraState.textContent = "Câmera pausada";
        camera.stop();
    }
    finally {
        if (completed) {
            camera.stop();
        }
        busy = false;
    }
}
async function waitForFacePhase(phase) {
    let stableSamples = 0;
    let networkFailures = 0;
    const startedAt = performance.now();
    let fallbackOffered = false;
    faceGuide.className = `face-guide phase-${phase}`;
    guidePhaseText.textContent = phase === "far" ? "1 de 2 · Mais longe" : "2 de 2 · Mais perto";
    biometricStatus.textContent = phase === "far"
        ? "Posicione o rosto e siga as orientações."
        : "Aproxime o rosto e siga as orientações.";
    setProximityIndicator(0, "red");
    while (stableSamples < GUIDE_READY_SAMPLES) {
        if (phase === "far" &&
            !fallbackOffered &&
            performance.now() - startedAt >= AUTO_FACE_DETECTION_WINDOW_MS) {
            fallbackOffered = true;
            await waitForManualReady();
            biometricStatus.textContent = "Verificando seu enquadramento...";
        }
        const snapshot = camera.snapshotForGuide();
        if (!snapshot.quality.acceptable) {
            stableSamples = 0;
            faceGuide.classList.remove("guide-ready");
            const lightingHint = clientQualityInstruction(snapshot.quality);
            cameraState.textContent = lightingHint.state;
            biometricStatus.textContent = lightingHint.message;
            setProximityIndicator(0, "red");
            await sleep(GUIDE_SAMPLE_MS);
            continue;
        }
        let guide;
        try {
            guide = await client.guideIdentityFace(identityToken, snapshot.imageBase64);
            networkFailures = 0;
        }
        catch (error) {
            networkFailures++;
            if (networkFailures >= GUIDE_MAX_NETWORK_FAILURES) {
                throw error;
            }
            cameraState.textContent = "Analisando rosto";
            biometricStatus.textContent = "Ajustando o enquadramento...";
            await sleep(GUIDE_SAMPLE_MS);
            continue;
        }
        const assessment = assessGuide(guide, phase);
        renderGuideAssessment(assessment);
        if (assessment.ready) {
            stableSamples++;
        }
        else if (assessment.captureReady) {
            stableSamples = Math.max(0, stableSamples - 1);
        }
        else {
            stableSamples = 0;
        }
        if (stableSamples >= GUIDE_READY_SAMPLES) {
            faceGuide.classList.add("guide-ready");
            biometricStatus.textContent = "Posição ideal. Mantenha-se assim...";
            setProximityIndicator(100, "green");
            await sleep(180);
            return;
        }
        await sleep(GUIDE_SAMPLE_MS);
    }
}
async function capturePhase(phase) {
    const frames = [];
    biometricStatus.textContent = phase === "far"
        ? "Capturando a primeira imagem..."
        : "Capturando a segunda imagem...";
    const captureWidth = phase === "near" ? 960 : 640;
    const captureJPEGQuality = phase === "near" ? 0.92 : 0.86;
    while (frames.length < PHASE_CAPTURE_FRAMES) {
        const guideSnapshot = camera.snapshotForGuide(360, 0.72);
        if (!guideSnapshot.quality.acceptable) {
            const lightingHint = clientQualityInstruction(guideSnapshot.quality);
            faceGuide.classList.remove("guide-ready");
            cameraState.textContent = lightingHint.state;
            biometricStatus.textContent = lightingHint.message;
            setProximityIndicator(0, "red");
            await waitForFacePhase(phase);
            continue;
        }
        const guide = await client.guideIdentityFace(identityToken, guideSnapshot.imageBase64);
        const assessment = assessGuide(guide, phase);
        renderGuideAssessment(assessment);
        if (!assessment.captureReady) {
            await waitForFacePhase(phase);
            continue;
        }
        const snapshot = camera.snapshotForGuide(captureWidth, captureJPEGQuality);
        if (!snapshot.quality.acceptable) {
            const lightingHint = clientQualityInstruction(snapshot.quality);
            biometricStatus.textContent = lightingHint.message;
            await sleep(GUIDE_SAMPLE_MS);
            continue;
        }
        frames.push({
            imageBase64: snapshot.imageBase64,
            phase,
            clientQuality: {
                brightness: snapshot.quality.brightness,
                contrast: snapshot.quality.contrast,
                sharpness: snapshot.quality.sharpness,
            },
        });
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
async function showCaptureSuccess(message) {
    biometricStatus.textContent = message;
    captureFlash.classList.add("visible");
    await sleep(220);
    captureFlash.classList.remove("visible");
    await sleep(280);
}
function assessGuide(guide, phase) {
    const config = PHASE_GUIDE[phase];
    if (!guide.faceDetected || guide.confidence < 0.72) {
        return {
            ready: false,
            captureReady: false,
            message: "Posicione o rosto dentro do oval.",
            state: "Procurando rosto",
            tone: "red",
            proximityPercent: 0,
        };
    }
    const proximityPercent = Math.round(Math.max(6, Math.min(100, (guide.heightRatio / config.target) * 100)));
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
    const verticalOffset = Math.abs(guide.centerY - 0.46);
    if (horizontalOffset > 0.12 || verticalOffset > 0.14) {
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
    if (horizontalOffset > 0.09 || verticalOffset > 0.11) {
        return {
            ready: false,
            captureReady: true,
            message: "Quase lá. Centralize um pouco mais.",
            state: "Quase na posição",
            tone: "yellow",
            proximityPercent,
        };
    }
    if (guide.quality.score < 0.34) {
        return {
            ready: false,
            captureReady: true,
            message: "Quase lá. Mantenha o aparelho firme.",
            state: "Ajustando nitidez",
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
function clientQualityInstruction(quality) {
    if (quality.brightness < 0.16) {
        return {
            message: "Está escuro. Aumente a iluminação do rosto.",
            state: "Pouca luz",
        };
    }
    if (quality.brightness > 0.88) {
        return {
            message: "Está muito claro. Evite luz forte diretamente no rosto.",
            state: "Luz excessiva",
        };
    }
    if (quality.sharpness < 0.018) {
        return {
            message: "Imagem pouco nítida. Mantenha o celular firme.",
            state: "Imagem desfocada",
        };
    }
    if (quality.contrast < 0.055) {
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
function renderGuideAssessment(assessment) {
    biometricStatus.textContent = assessment.message;
    cameraState.textContent = assessment.state;
    faceGuide.classList.toggle("guide-ready", assessment.tone === "green");
    faceGuide.classList.toggle("guide-near", assessment.tone === "yellow");
    setProximityIndicator(assessment.proximityPercent, assessment.tone);
}
function setProximityIndicator(percent, tone) {
    progressBar.style.width = `${Math.max(0, Math.min(100, percent))}%`;
    progressBar.classList.remove("proximity-red", "proximity-yellow", "proximity-green");
    progressBar.classList.add(`proximity-${tone}`);
}
function scheduleAutomaticBiometry() {
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
async function waitForManualReady() {
    startButton.hidden = false;
    startButton.disabled = false;
    startButton.textContent = "Estou pronto";
    biometricStatus.textContent = "Posicione o rosto preenchendo o oval e toque em “Estou pronto”.";
    await new Promise((resolve) => {
        manualReadyResolver = resolve;
    });
    startButton.hidden = true;
    startButton.disabled = true;
}
function renderStatus(status) {
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
function renderIdentityResult(result, capturedFrames) {
    documentPanel.hidden = true;
    biometryPanel.hidden = true;
    finalPanel.hidden = false;
    const decisionLabel = {
        approved: "IDENTIDADE CONFIRMADA",
        review: "ANÁLISE NECESSÁRIA",
        rejected: "VERIFICAÇÃO NÃO APROVADA",
    }[result.decision];
    const details = {
        ...result.document,
        ...(documentDetails ?? {}),
    };
    const bestCapture = capturedFrames[result.bestFrameIndex]?.imageBase64 ?? "";
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
function faceMatchPresentation(similarity, threshold) {
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
function scorePosition(value) {
    const clamped = Math.max(-1, Math.min(1, value));
    return ((clamped + 1) / 2) * 100;
}
function signedFaceScore(value) {
    const prefix = value >= 0 ? "+" : "";
    return `${prefix}${value.toFixed(3)}`;
}
function dataLine(label, value) {
    if (!value) {
        return "";
    }
    return `<div class="identity-data-item"><span>${escapeHtml(label)}</span><strong>${escapeHtml(value)}</strong></div>`;
}
function photoCard(label, imageBase64) {
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
function frameScoreLine(scores) {
    if (!scores.length) {
        return "";
    }
    const values = scores.map((score) => faceScore(score)).join(" · ");
    return `<div class="frame-score-line"><span>Frames próximos usados no match</span><strong>${escapeHtml(values)}</strong></div>`;
}
function faceScore(value) {
    return value.toFixed(3);
}
function renderFinalStatus(status) {
    const labels = {
        approved: "Identidade confirmada.",
        review: "A verificação será analisada.",
        rejected: "Não foi possível confirmar a identidade.",
    };
    resultPanel.innerHTML = `<div class="result-header result-${status}"><span>${labels[status]}</span></div>`;
}
function showFatal(message) {
    camera.stop();
    documentPanel.hidden = true;
    biometryPanel.hidden = true;
    finalPanel.hidden = false;
    resultPanel.innerHTML = `<div class="fatal-message">${escapeHtml(message)}</div>`;
}
function consumeIdentityToken() {
    const fragment = new URLSearchParams(window.location.hash.slice(1));
    const fromFragment = fragment.get("identity")?.trim() ?? "";
    if (fromFragment) {
        sessionStorage.setItem(TOKEN_STORAGE_KEY, fromFragment);
        history.replaceState(null, "", window.location.pathname + window.location.search);
        return fromFragment;
    }
    return sessionStorage.getItem(TOKEN_STORAGE_KEY)?.trim() ?? "";
}
function friendlyDocumentError(message) {
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
    if (["localhost", "127.0.0.1", "::1"].includes(window.location.hostname) ||
        window.location.hostname.endsWith(".trycloudflare.com")) {
        return `${generic} Detalhe: ${message}`;
    }
    return generic;
}
function friendlyBiometryError(message) {
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
function isRecaptureRequired(message) {
    return message.includes("recapture required") || message.includes("capture quality insufficient");
}
function formatExpiration(value) {
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
        return "";
    }
    return date.toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "short" });
}
function metric(label, value) {
    return `<div class="metric"><span>${escapeHtml(label)}</span><strong>${escapeHtml(value)}</strong></div>`;
}
function checkLine(label, passed) {
    return `<div class="identity-check"><span>${passed ? "✓" : "×"}</span><strong>${escapeHtml(label)}</strong></div>`;
}
function percentage(value) {
    return `${(value * 100).toFixed(1)}%`;
}
function requiredElement(id) {
    const element = document.getElementById(id);
    if (!element) {
        throw new Error(`Missing required element: ${id}`);
    }
    return element;
}
function errorMessage(error) {
    return error instanceof Error ? error.message : "Erro inesperado";
}
function escapeHtml(value) {
    return value
        .replaceAll("&", "&amp;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;")
        .replaceAll('"', "&quot;")
        .replaceAll("'", "&#039;");
}
async function sleep(milliseconds) {
    await new Promise((resolve) => window.setTimeout(resolve, milliseconds));
}

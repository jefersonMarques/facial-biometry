import { ExperimentalGeometryLiveness, } from "./liveness-v2.js";
const video = document.getElementById("camera");
const phaseLabel = document.getElementById("guidePhaseText");
const biometryPanel = document.getElementById("biometryPanel");
const finalPanel = document.getElementById("finalPanel");
const resultPanel = document.getElementById("identityResult");
if (video && phaseLabel && biometryPanel) {
    const diagnostics = createDiagnosticsPanel(biometryPanel);
    const probe = new ExperimentalGeometryLiveness();
    let lastPhase = null;
    let lastSummary = null;
    let startInProgress = false;
    const warmup = probe.initialize().catch((error) => {
        renderUnavailable(diagnostics, error);
        throw error;
    });
    const renderSummary = (summary) => {
        lastSummary = summary;
        diagnostics.hidden = false;
        const output = diagnostics.querySelector("pre");
        if (output) {
            output.textContent = formatSummary(summary);
        }
        persistSummary(summary);
        renderFinalDiagnostics(resultPanel, finalPanel, summary);
    };
    const syncPhase = (forceReset = false) => {
        const phase = phaseFromLabel(phaseLabel.textContent ?? "");
        if (phase === "far" && (forceReset || lastPhase !== "far")) {
            probe.reset();
        }
        probe.setPhase(phase);
        if (phase === null && lastPhase !== null) {
            renderSummary(probe.summarize());
        }
        lastPhase = phase;
    };
    const start = async () => {
        if (startInProgress) {
            return;
        }
        startInProgress = true;
        diagnostics.hidden = false;
        const output = diagnostics.querySelector("pre");
        if (output) {
            output.textContent = "Carregando motor local de landmarks...";
        }
        try {
            await warmup;
            syncPhase(true);
            await probe.start(video, renderSummary);
        }
        catch (error) {
            renderUnavailable(diagnostics, error);
        }
        finally {
            startInProgress = false;
        }
    };
    const phaseObserver = new MutationObserver(() => syncPhase());
    phaseObserver.observe(phaseLabel, { childList: true, characterData: true, subtree: true });
    const resultObserver = resultPanel
        ? new MutationObserver(() => {
            if (lastSummary) {
                renderFinalDiagnostics(resultPanel, finalPanel, lastSummary);
            }
        })
        : null;
    resultObserver?.observe(resultPanel, { childList: true });
    video.addEventListener("playing", () => void start());
    video.addEventListener("pause", () => probe.stop());
    video.addEventListener("ended", () => probe.stop());
    if (!video.paused && video.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA) {
        void start();
    }
    window.addEventListener("beforeunload", () => {
        phaseObserver.disconnect();
        resultObserver?.disconnect();
        probe.dispose();
    });
}
function phaseFromLabel(value) {
    const normalized = value.trim().toLowerCase();
    if (normalized.includes("captura 1")) {
        return "far";
    }
    if (normalized.includes("captura 2")) {
        return "near";
    }
    return null;
}
function createDiagnosticsPanel(parent) {
    const existing = document.getElementById("livenessV2Diagnostics");
    if (existing) {
        return existing;
    }
    const details = document.createElement("details");
    details.id = "livenessV2Diagnostics";
    details.open = true;
    details.hidden = false;
    details.style.cssText = "margin-top:16px;padding:14px 16px;border:1px solid rgba(255,255,255,.12);border-radius:12px;background:rgba(8,15,28,.58);";
    const summary = document.createElement("summary");
    summary.textContent = "Liveness V2 · diagnóstico experimental";
    summary.style.cssText = "cursor:pointer;font-weight:700;";
    const note = document.createElement("p");
    note.textContent = "Somente diagnóstico. Este sinal ainda não altera aprovação, revisão ou rejeição.";
    note.style.cssText = "margin:10px 0 8px;font-size:12px;opacity:.72;";
    const output = document.createElement("pre");
    output.textContent = "Preparando motor local...";
    output.style.cssText = "margin:0;white-space:pre-wrap;font:12px/1.55 ui-monospace,SFMono-Regular,Consolas,monospace;";
    details.append(summary, note, output);
    parent.append(details);
    return details;
}
function renderUnavailable(panel, error) {
    panel.hidden = false;
    const output = panel.querySelector("pre");
    if (output) {
        const message = error instanceof Error ? error.message : "erro desconhecido";
        output.textContent = `Motor geométrico indisponível: ${message}\nA biometria atual continua funcionando normalmente.`;
    }
}
function formatSummary(summary) {
    const status = summary.status === "experimental" ? "EVIDÊNCIA COLETADA" : "AMOSTRA INSUFICIENTE";
    return [
        `Estado: ${status}`,
        `Frames locais: ${summary.sampleCount} (longe ${summary.farSamples} · perto ${summary.nearSamples})`,
        `Escala perto/longe: ${summary.scaleRatio.toFixed(3)}x`,
        `Transição de escala: ${percentage(summary.transitionScore)}`,
        `Mudança de perspectiva: ${(summary.perspectiveChange * 100).toFixed(3)}%`,
        `Mudança de profundidade: ${summary.depthChange.toFixed(5)}`,
        `Estabilidade por fase: ${percentage(summary.phaseStability)}`,
        `Geometry evidence: ${percentage(summary.evidenceScore)} · NÃO CALIBRADO`,
    ].join("\n");
}
function renderFinalDiagnostics(resultPanel, finalPanel, summary) {
    if (!resultPanel || !finalPanel || finalPanel.hidden) {
        return;
    }
    document.getElementById("livenessV2FinalDiagnostics")?.remove();
    const details = document.createElement("details");
    details.id = "livenessV2FinalDiagnostics";
    details.style.cssText = "margin-top:16px;padding:14px 16px;border:1px solid rgba(255,255,255,.12);border-radius:12px;";
    const title = document.createElement("summary");
    title.textContent = "Liveness V2 · geometria experimental";
    title.style.cssText = "cursor:pointer;font-weight:700;";
    const output = document.createElement("pre");
    output.textContent = formatSummary(summary);
    output.style.cssText = "margin:12px 0 0;white-space:pre-wrap;font:12px/1.55 ui-monospace,SFMono-Regular,Consolas,monospace;";
    details.append(title, output);
    resultPanel.append(details);
}
function persistSummary(summary) {
    try {
        sessionStorage.setItem("faceproof.liveness-v2.last-summary", JSON.stringify(summary));
    }
    catch {
        // Diagnóstico experimental; falhas de armazenamento não interferem no fluxo principal.
    }
}
function percentage(value) {
    return `${(value * 100).toFixed(1)}%`;
}

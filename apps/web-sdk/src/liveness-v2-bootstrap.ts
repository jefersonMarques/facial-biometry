import {
    ExperimentalGeometryLiveness,
    type GeometryCapturePhase,
    type GeometryLivenessSummary,
} from "./liveness-v2.js";
import type { WasmShadowDiagnostics } from "./liveness-core-shadow.js";

const video = document.getElementById("camera") as HTMLVideoElement | null;
const phaseLabel = document.getElementById("guidePhaseText") as HTMLSpanElement | null;
const biometryPanel = document.getElementById("biometryPanel") as HTMLElement | null;
const finalPanel = document.getElementById("finalPanel") as HTMLElement | null;
const resultPanel = document.getElementById("identityResult") as HTMLElement | null;

type EngineState = "preparing" | "ready" | "error";

interface RuntimeDiagnostics {
    engineState: EngineState;
    phase: GeometryCapturePhase | null;
    videoReadyState: number;
    videoWidth: number;
    videoHeight: number;
    cameraActive: boolean;
    errorMessage: string;
}

if (video && phaseLabel && biometryPanel) {
    const diagnosticsHost = biometryPanel.querySelector(".stage-content") as HTMLElement | null;
    const diagnostics = createDiagnosticsPanel(diagnosticsHost ?? biometryPanel);
    const probe = new ExperimentalGeometryLiveness();
    let lastPhase: GeometryCapturePhase | null = null;
    let lastSummary: GeometryLivenessSummary = probe.summarize();
    let startInProgress = false;

    const runtime: RuntimeDiagnostics = {
        engineState: "preparing",
        phase: null,
        videoReadyState: video.readyState,
        videoWidth: video.videoWidth,
        videoHeight: video.videoHeight,
        cameraActive: false,
        errorMessage: "",
    };

    const renderDiagnostics = (): void => {
        refreshRuntime(runtime, video, phaseLabel);
        diagnostics.hidden = false;
        const output = diagnostics.querySelector("pre");
        if (output) {
            output.textContent = formatDiagnostics(
                lastSummary,
                runtime,
                probe.getWasmShadowDiagnostics(),
            );
        }
    };

    let initialization: Promise<void> | null = null;

    const ensureInitialized = async (): Promise<void> => {
        if (initialization) {
            return initialization;
        }

        runtime.engineState = "preparing";
        runtime.errorMessage = "";
        renderDiagnostics();

        initialization = new Promise<void>((resolve) => {
            window.setTimeout(resolve, 350);
        })
            .then(() => probe.initialize())
            .then(() => {
                runtime.engineState = "ready";
                renderDiagnostics();
            })
            .catch((error: unknown) => {
                initialization = null;
                runtime.engineState = "error";
                runtime.errorMessage = errorMessage(error);
                renderDiagnostics();
                throw error;
            });

        return initialization;
    };

    const renderSummary = (summary: GeometryLivenessSummary): void => {
        lastSummary = summary;
        renderDiagnostics();
        persistSummary(summary);
        renderFinalDiagnostics(
            resultPanel,
            finalPanel,
            summary,
            runtime,
            probe.getWasmShadowDiagnostics(),
        );
    };

    const syncPhase = (forceReset = false): void => {
        const phase = phaseFromLabel(phaseLabel.textContent ?? "");
        if (phase === "far" && (forceReset || lastPhase !== "far")) {
            probe.reset();
            lastSummary = probe.summarize();
        }
        probe.setPhase(phase);
        if (phase === null && lastPhase !== null) {
            renderSummary(probe.summarize());
        }
        lastPhase = phase;
        renderDiagnostics();
    };

    const start = async (): Promise<void> => {
        if (startInProgress) {
            return;
        }
        startInProgress = true;
        renderDiagnostics();

        try {
            await ensureInitialized();
            syncPhase(true);
            await probe.start(video, renderSummary);
            runtime.engineState = "ready";
            renderDiagnostics();
        } catch (error) {
            runtime.engineState = "error";
            runtime.errorMessage = errorMessage(error);
            renderDiagnostics();
        } finally {
            startInProgress = false;
        }
    };

    const phaseObserver = new MutationObserver(() => syncPhase());
    phaseObserver.observe(phaseLabel, { childList: true, characterData: true, subtree: true });

    const finalPanelObserver = finalPanel
        ? new MutationObserver(() => {
            if (!finalPanel.hidden) {
                renderFinalDiagnostics(
                    resultPanel,
                    finalPanel,
                    lastSummary,
                    runtime,
                    probe.getWasmShadowDiagnostics(),
                );
            }
        })
        : null;
    finalPanelObserver?.observe(finalPanel as HTMLElement, {
        attributes: true,
        attributeFilter: ["hidden"],
    });

    const diagnosticTimer = window.setInterval(() => {
        lastSummary = probe.summarize();
        renderDiagnostics();
    }, 500);

    video.addEventListener("playing", () => void start());
    video.addEventListener("pause", () => {
        probe.stop();
        renderDiagnostics();
    });
    video.addEventListener("ended", () => {
        probe.stop();
        renderDiagnostics();
    });

    if (!video.paused && video.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA) {
        void start();
    } else {
        renderDiagnostics();
    }

    window.addEventListener("beforeunload", () => {
        window.clearInterval(diagnosticTimer);
        phaseObserver.disconnect();
        finalPanelObserver?.disconnect();
        probe.dispose();
    });
}

function phaseFromLabel(value: string): GeometryCapturePhase | null {
    const normalized = value.trim().toLowerCase();
    if (normalized.includes("captura 1")) {
        return "far";
    }
    if (normalized.includes("captura 2")) {
        return "near";
    }
    return null;
}

function createDiagnosticsPanel(parent: HTMLElement): HTMLDetailsElement {
    const existing = document.getElementById("livenessV2Diagnostics") as HTMLDetailsElement | null;
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

    const copyButton = document.createElement("button");
    copyButton.type = "button";
    copyButton.textContent = "Copiar diagnóstico";
    copyButton.style.cssText = "margin:0 0 10px;padding:7px 11px;border:1px solid rgba(255,255,255,.22);border-radius:8px;background:rgba(255,255,255,.08);color:inherit;cursor:pointer;font:inherit;";
    copyButton.addEventListener("click", () => void copyDiagnostics(details, copyButton));

    const output = document.createElement("pre");
    output.textContent = "Preparando motor local...";
    output.style.cssText = "margin:0;white-space:pre-wrap;font:12px/1.55 ui-monospace,SFMono-Regular,Consolas,monospace;";

    details.append(summary, note, copyButton, output);
    parent.append(details);
    return details;
}

async function copyDiagnostics(panel: HTMLElement, button: HTMLButtonElement): Promise<void> {
    const output = panel.querySelector("pre");
    const text = output?.textContent?.trim() ?? "";
    if (!text) {
        return;
    }

    try {
        await navigator.clipboard.writeText(text);
    } catch {
        const textarea = document.createElement("textarea");
        textarea.value = text;
        textarea.style.position = "fixed";
        textarea.style.opacity = "0";
        document.body.append(textarea);
        textarea.select();
        document.execCommand("copy");
        textarea.remove();
    }

    const previousText = button.textContent;
    button.textContent = "Copiado ✓";
    window.setTimeout(() => {
        button.textContent = previousText;
    }, 1_500);
}

function refreshRuntime(
    runtime: RuntimeDiagnostics,
    video: HTMLVideoElement,
    phaseLabel: HTMLSpanElement,
): void {
    runtime.phase = phaseFromLabel(phaseLabel.textContent ?? "");
    runtime.videoReadyState = video.readyState;
    runtime.videoWidth = video.videoWidth;
    runtime.videoHeight = video.videoHeight;
    runtime.cameraActive = !video.paused &&
        !video.ended &&
        video.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA;
}

function formatDiagnostics(
    summary: GeometryLivenessSummary,
    runtime: RuntimeDiagnostics,
    wasm: WasmShadowDiagnostics,
): string {
    const engineLabel = {
        preparing: "carregando",
        ready: "pronto",
        error: "erro",
    }[runtime.engineState];

    const phaseLabel = runtime.phase === "far"
        ? "longe"
        : runtime.phase === "near"
            ? "perto"
            : "fora da captura";

    const status = summary.status === "experimental" ? "EVIDÊNCIA COLETADA" : "AMOSTRA INSUFICIENTE";
    const lines = [
        `Motor: ${engineLabel}`,
        `Câmera: ${runtime.cameraActive ? "ativa" : "inativa"} · ${runtime.videoWidth}x${runtime.videoHeight} · readyState ${runtime.videoReadyState}`,
        `Fase observada: ${phaseLabel}`,
        `Estado geométrico: ${status}`,
        `Frames geométricos válidos: ${summary.sampleCount} (longe ${summary.farSamples} · perto ${summary.nearSamples})`,
        `Escala perto/longe: ${summary.scaleRatio.toFixed(3)}x`,
        `Transição de escala: ${percentage(summary.transitionScore)}`,
        `Mudança de perspectiva: ${(summary.perspectiveChange * 100).toFixed(3)}%`,
        `Mudança de profundidade: ${summary.depthChange.toFixed(5)}`,
        `Estabilidade por fase: ${percentage(summary.phaseStability)}`,
        `Geometry evidence: ${percentage(summary.evidenceScore)} · NÃO CALIBRADO`,
        ...formatWasmDiagnostics(summary, wasm),
    ];

    if (runtime.errorMessage) {
        lines.push(`Erro do motor: ${runtime.errorMessage}`);
    }

    return lines.join("\n");
}

function formatWasmDiagnostics(
    typescriptSummary: GeometryLivenessSummary,
    wasm: WasmShadowDiagnostics,
): string[] {
    if (wasm.state === "idle") {
        return ["WASM shadow: aguardando"];
    }
    if (wasm.state === "loading") {
        return ["WASM shadow: carregando"];
    }
    if (wasm.state === "error") {
        return [
            "WASM shadow: indisponível · fluxo principal preservado",
            `Erro WASM: ${wasm.errorMessage || "não informado"}`,
        ];
    }
    if (!wasm.summary) {
        return ["WASM shadow: pronto · aguardando amostras"];
    }

    const candidate = wasm.summary;
    const deltas = [
        Math.abs(candidate.scaleRatio - typescriptSummary.scaleRatio),
        Math.abs(candidate.transitionScore - typescriptSummary.transitionScore),
        Math.abs(candidate.perspectiveChange - typescriptSummary.perspectiveChange),
        Math.abs(candidate.depthChange - typescriptSummary.depthChange),
        Math.abs(candidate.phaseStability - typescriptSummary.phaseStability),
        Math.abs(candidate.evidenceScore - typescriptSummary.evidenceScore),
    ];
    const maxDelta = Math.max(...deltas);
    const parity = maxDelta <= 1e-9 ? "OK" : "DIVERGENTE";

    return [
        `WASM shadow: pronto · ${candidate.sampleCount} frames`,
        `WASM Geometry evidence: ${percentage(candidate.evidenceScore)}`,
        `Paridade TS × WASM: ${parity} · delta máx. ${maxDelta.toExponential(2)}`,
    ];
}

function renderFinalDiagnostics(
    resultPanel: HTMLElement | null,
    finalPanel: HTMLElement | null,
    summary: GeometryLivenessSummary,
    runtime: RuntimeDiagnostics,
    wasm: WasmShadowDiagnostics,
): void {
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

    const copyButton = document.createElement("button");
    copyButton.type = "button";
    copyButton.textContent = "Copiar diagnóstico";
    copyButton.style.cssText = "margin:12px 0 0;padding:7px 11px;border:1px solid rgba(255,255,255,.22);border-radius:8px;background:rgba(255,255,255,.08);color:inherit;cursor:pointer;font:inherit;";
    copyButton.addEventListener("click", () => void copyDiagnostics(details, copyButton));

    const output = document.createElement("pre");
    output.textContent = formatDiagnostics(summary, runtime, wasm);
    output.style.cssText = "margin:12px 0 0;white-space:pre-wrap;font:12px/1.55 ui-monospace,SFMono-Regular,Consolas,monospace;";

    details.append(title, copyButton, output);
    resultPanel.append(details);
}

function persistSummary(summary: GeometryLivenessSummary): void {
    try {
        sessionStorage.setItem("faceproof.liveness-v2.last-summary", JSON.stringify(summary));
    } catch {
        // Diagnóstico experimental; falhas de armazenamento não interferem no fluxo principal.
    }
}

function errorMessage(error: unknown): string {
    return error instanceof Error ? error.message : String(error);
}

function percentage(value: number): string {
    return `${(value * 100).toFixed(1)}%`;
}

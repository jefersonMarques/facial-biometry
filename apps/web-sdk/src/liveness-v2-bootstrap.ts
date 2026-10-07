import {
    CAPTURE_LIFECYCLE_EVENT,
    type CaptureLifecycleEventDetail,
    type CaptureLifecycleState,
} from "./capture-lifecycle.js";
import {
    ExperimentalGeometryLiveness,
    type GeometryCapturePhase,
    type GeometryLivenessSummary,
} from "./liveness-v2.js";
import type {
    LocalFaceGuideMetrics,
    WasmShadowDiagnostics,
} from "./liveness-core-shadow.js";
import type {
    FrameQualityAssessment,
    IdentityGuideResult,
} from "./types.js";

const video = document.getElementById("camera") as HTMLVideoElement | null;
const biometryPanel = document.getElementById("biometryPanel") as HTMLElement | null;
const finalPanel = document.getElementById("finalPanel") as HTMLElement | null;
const resultPanel = document.getElementById("identityResult") as HTMLElement | null;

type EngineState = "preparing" | "ready" | "error";

interface ServerGuideSnapshot {
    timestampMs: number;
    guide: IdentityGuideResult;
    clientQuality: FrameQualityAssessment;
}

interface LocalGuideEventDetail {
    timestampMs: number;
    guide: LocalFaceGuideMetrics;
}

interface RuntimeDiagnostics {
    engineState: EngineState;
    phase: GeometryCapturePhase | null;
    lifecycleState: CaptureLifecycleState;
    lifecycleRunId: string;
    lifecycleSequence: number;
    videoReadyState: number;
    videoWidth: number;
    videoHeight: number;
    cameraActive: boolean;
    errorMessage: string;
}

if (video && biometryPanel) {
    const diagnosticsHost = biometryPanel.querySelector(".stage-content") as HTMLElement | null;
    const diagnostics = createDiagnosticsPanel(diagnosticsHost ?? biometryPanel);
    const probe = new ExperimentalGeometryLiveness();
    let lastPhase: GeometryCapturePhase | null = null;
    let lastSummary: GeometryLivenessSummary = probe.summarize();
    let latestServerGuide: ServerGuideSnapshot | null = null;
    const localGuideHistory: LocalFaceGuideMetrics[] = [];
    let startInProgress = false;

    const runtime: RuntimeDiagnostics = {
        engineState: "preparing",
        phase: null,
        lifecycleState: "idle",
        lifecycleRunId: "",
        lifecycleSequence: 0,
        videoReadyState: video.readyState,
        videoWidth: video.videoWidth,
        videoHeight: video.videoHeight,
        cameraActive: false,
        errorMessage: "",
    };

    const renderDiagnostics = (): void => {
        refreshRuntime(runtime, video);
        diagnostics.hidden = false;
        const output = diagnostics.querySelector("pre");
        if (output) {
            output.textContent = formatDiagnostics(
                lastSummary,
                runtime,
                probe.getWasmShadowDiagnostics(),
                latestServerGuide,
                localGuideHistory,
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
        const wasm = probe.getWasmShadowDiagnostics();
        persistTelemetry(summary, wasm, runtime.lifecycleRunId);
        renderFinalDiagnostics(
            resultPanel,
            finalPanel,
            summary,
            runtime,
            wasm,
            latestServerGuide,
            localGuideHistory,
        );
    };

    const applyLifecycle = (detail: CaptureLifecycleEventDetail): void => {
        if (detail.protocolVersion !== "1" || !detail.runId) {
            return;
        }
        if (
            detail.runId === runtime.lifecycleRunId &&
            detail.sequence <= runtime.lifecycleSequence
        ) {
            return;
        }

        const previousPhase = lastPhase;
        const runChanged = detail.runId !== runtime.lifecycleRunId;
        const phase: GeometryCapturePhase | null =
            detail.state === "far" || detail.state === "near"
                ? detail.state
                : null;

        runtime.lifecycleState = detail.state;
        runtime.lifecycleRunId = detail.runId;
        runtime.lifecycleSequence = detail.sequence;
        runtime.phase = phase;

        if (detail.state === "far" && (runChanged || previousPhase !== "far")) {
            probe.reset();
            lastSummary = probe.summarize();
        }

        probe.setPhase(phase);
        lastPhase = phase;

        if (phase === null && previousPhase !== null) {
            renderSummary(probe.summarize());
            return;
        }

        renderDiagnostics();
    };

    const handleCaptureLifecycle = (event: Event): void => {
        const detail = (event as CustomEvent<CaptureLifecycleEventDetail>).detail;
        if (!detail) {
            return;
        }
        applyLifecycle(detail);
    };

    const start = async (): Promise<void> => {
        if (startInProgress) {
            return;
        }
        startInProgress = true;
        renderDiagnostics();

        try {
            await ensureInitialized();
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

    const finalPanelObserver = finalPanel
        ? new MutationObserver(() => {
            if (!finalPanel.hidden) {
                renderFinalDiagnostics(
                    resultPanel,
                    finalPanel,
                    lastSummary,
                    runtime,
                    probe.getWasmShadowDiagnostics(),
                    latestServerGuide,
                    localGuideHistory,
                );
            }
        })
        : null;
    finalPanelObserver?.observe(finalPanel as HTMLElement, {
        attributes: true,
        attributeFilter: ["hidden"],
    });

    const handleLocalGuide = (event: Event): void => {
        const detail = (event as CustomEvent<LocalGuideEventDetail>).detail;
        if (!detail?.guide) {
            return;
        }
        localGuideHistory.push(detail.guide);
        if (localGuideHistory.length > 24) {
            localGuideHistory.splice(0, localGuideHistory.length - 24);
        }
        renderDiagnostics();
    };

    const handleServerGuide = (event: Event): void => {
        const detail = (event as CustomEvent<ServerGuideSnapshot>).detail;
        if (!detail?.guide) {
            return;
        }
        latestServerGuide = detail;
        renderDiagnostics();
    };

    window.addEventListener(CAPTURE_LIFECYCLE_EVENT, handleCaptureLifecycle);
    window.addEventListener("faceproof:local-guide", handleLocalGuide);
    window.addEventListener("faceproof:server-guide", handleServerGuide);

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
        finalPanelObserver?.disconnect();
        window.removeEventListener(CAPTURE_LIFECYCLE_EVENT, handleCaptureLifecycle);
        window.removeEventListener("faceproof:local-guide", handleLocalGuide);
        window.removeEventListener("faceproof:server-guide", handleServerGuide);
        probe.dispose();
    });
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
): void {
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
    serverGuide: ServerGuideSnapshot | null,
    localGuideHistory: LocalFaceGuideMetrics[],
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
        `Lifecycle: ${runtime.lifecycleState} · seq ${runtime.lifecycleSequence} · run ${runtime.lifecycleRunId ? runtime.lifecycleRunId.slice(0, 8) : "—"}`,
        `Fase explícita: ${phaseLabel}`,
        `Estado geométrico: ${status}`,
        `Frames geométricos válidos: ${summary.sampleCount} (longe ${summary.farSamples} · perto ${summary.nearSamples})`,
        `Escala perto/longe: ${summary.scaleRatio.toFixed(3)}x`,
        `Transição de escala: ${percentage(summary.transitionScore)}`,
        `Mudança de perspectiva: ${(summary.perspectiveChange * 100).toFixed(3)}%`,
        `Mudança de profundidade: ${summary.depthChange.toFixed(5)}`,
        `Estabilidade por fase: ${percentage(summary.phaseStability)}`,
        `Geometry evidence: ${percentage(summary.evidenceScore)} · NÃO CALIBRADO`,
        ...formatWasmDiagnostics(summary, wasm),
        ...formatGuideCalibration(serverGuide, localGuideHistory),
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
    if (
        candidate.sampleCount !== typescriptSummary.sampleCount ||
        candidate.farSamples !== typescriptSummary.farSamples ||
        candidate.nearSamples !== typescriptSummary.nearSamples
    ) {
        return [
            `WASM shadow: sincronizando · TS ${typescriptSummary.sampleCount} frames · WASM ${candidate.sampleCount} frames`,
            `Amostras: TS ${typescriptSummary.farSamples}/${typescriptSummary.nearSamples} · WASM ${candidate.farSamples}/${candidate.nearSamples}`,
            "Paridade TS × WASM: aguardando mesmas amostras",
        ];
    }

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

function formatGuideCalibration(
    server: ServerGuideSnapshot | null,
    localHistory: LocalFaceGuideMetrics[],
): string[] {
    if (localHistory.length === 0) {
        return ["Guide local C++: aguardando primeira amostra"];
    }

    if (!server) {
        const local = localHistory[localHistory.length - 1];
        if (!local) {
            return ["Guide local C++: aguardando primeira amostra"];
        }
        return [
            "Guide local C++: ATIVO · /identity/guide sem POST contínuo",
            `C++: centro ${local.centerX.toFixed(3)}/${local.centerY.toFixed(3)} · tamanho ${local.widthRatio.toFixed(3)}×${local.heightRatio.toFixed(3)} · roll ${local.rollDegrees.toFixed(1)}° · faceSize ${percentage(local.faceSizeScore)}`,
            "Servidor continua validando integralmente os frames finais.",
        ];
    }

    const local = nearestLocalGuide(server.timestampMs, localHistory);
    if (!local) {
        return ["Guide local: sem amostra temporal próxima"];
    }

    const ageMs = Math.abs(local.timestampMs - server.timestampMs);
    const serverGuide = server.guide;
    const localTop = local.centerY - local.heightRatio / 2;
    const localBottom = local.centerY + local.heightRatio / 2;
    const serverTop = serverGuide.centerY - serverGuide.heightRatio / 2;
    const serverBottom = serverGuide.centerY + serverGuide.heightRatio / 2;
    const heightScale = local.heightRatio > 1e-6
        ? serverGuide.heightRatio / local.heightRatio
        : 0;

    return [
        "Guide local C++ · calibração shadow",
        `C++: centro ${local.centerX.toFixed(3)}/${local.centerY.toFixed(3)} · tamanho ${local.widthRatio.toFixed(3)}×${local.heightRatio.toFixed(3)} · roll ${local.rollDegrees.toFixed(1)}° · faceSize ${percentage(local.faceSizeScore)}`,
        `YuNet: centro ${serverGuide.centerX.toFixed(3)}/${serverGuide.centerY.toFixed(3)} · tamanho ${serverGuide.widthRatio.toFixed(3)}×${serverGuide.heightRatio.toFixed(3)} · roll ${serverGuide.rollDegrees.toFixed(1)}° · faceSize ${percentage(serverGuide.quality.faceSize)} · conf ${percentage(serverGuide.confidence)}`,
        `Vertical: top C++ ${localTop.toFixed(3)} / YuNet ${serverTop.toFixed(3)} · bottom C++ ${localBottom.toFixed(3)} / YuNet ${serverBottom.toFixed(3)} · fator altura ${heightScale.toFixed(3)}x`,
        `Delta aprox. (${ageMs.toFixed(0)}ms): X ${signed(local.centerX - serverGuide.centerX, 3)} · Y ${signed(local.centerY - serverGuide.centerY, 3)} · altura ${signed(local.heightRatio - serverGuide.heightRatio, 3)} · roll ${signed(local.rollDegrees - serverGuide.rollDegrees, 1)}°`,
        `Qualidade servidor: ${percentage(serverGuide.quality.score)} · cliente JS atual: brilho ${percentage(server.clientQuality.brightness)} · nitidez ${percentage(server.clientQuality.sharpness)}`,
    ];
}

function nearestLocalGuide(
    timestampMs: number,
    history: LocalFaceGuideMetrics[],
): LocalFaceGuideMetrics | null {
    let best: LocalFaceGuideMetrics | null = null;
    let bestDistance = Number.POSITIVE_INFINITY;

    for (const item of history) {
        const distance = Math.abs(item.timestampMs - timestampMs);
        if (distance < bestDistance) {
            best = item;
            bestDistance = distance;
        }
    }

    return best;
}

function signed(value: number, digits: number): string {
    const prefix = value >= 0 ? "+" : "";
    return `${prefix}${value.toFixed(digits)}`;
}

function renderFinalDiagnostics(
    resultPanel: HTMLElement | null,
    finalPanel: HTMLElement | null,
    summary: GeometryLivenessSummary,
    runtime: RuntimeDiagnostics,
    wasm: WasmShadowDiagnostics,
    serverGuide: ServerGuideSnapshot | null,
    localGuideHistory: LocalFaceGuideMetrics[],
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
    output.textContent = formatDiagnostics(
        summary,
        runtime,
        wasm,
        serverGuide,
        localGuideHistory,
    );
    output.style.cssText = "margin:12px 0 0;white-space:pre-wrap;font:12px/1.55 ui-monospace,SFMono-Regular,Consolas,monospace;";

    details.append(title, copyButton, output);
    resultPanel.append(details);
}

function persistTelemetry(
    summary: GeometryLivenessSummary,
    wasm: WasmShadowDiagnostics,
    runId: string,
): void {
    try {
        sessionStorage.setItem("faceproof.liveness-v2.last-summary", JSON.stringify(summary));

        const wasmMaxDelta = geometryWasmMaxDelta(summary, wasm);
        sessionStorage.setItem("faceproof.liveness-v2.telemetry", JSON.stringify({
            runId,
            ...summary,
            wasmStatus: wasm.state,
            ...(wasmMaxDelta === null ? {} : { wasmMaxDelta }),
        }));
    } catch {
        // Telemetria experimental; falhas de armazenamento não interferem no fluxo principal.
    }
}

function geometryWasmMaxDelta(
    typescriptSummary: GeometryLivenessSummary,
    wasm: WasmShadowDiagnostics,
): number | null {
    const candidate = wasm.summary;
    if (
        !candidate ||
        candidate.sampleCount !== typescriptSummary.sampleCount ||
        candidate.farSamples !== typescriptSummary.farSamples ||
        candidate.nearSamples !== typescriptSummary.nearSamples
    ) {
        return null;
    }

    return Math.max(
        Math.abs(candidate.scaleRatio - typescriptSummary.scaleRatio),
        Math.abs(candidate.transitionScore - typescriptSummary.transitionScore),
        Math.abs(candidate.perspectiveChange - typescriptSummary.perspectiveChange),
        Math.abs(candidate.depthChange - typescriptSummary.depthChange),
        Math.abs(candidate.phaseStability - typescriptSummary.phaseStability),
        Math.abs(candidate.evidenceScore - typescriptSummary.evidenceScore),
    );
}

function errorMessage(error: unknown): string {
    return error instanceof Error ? error.message : String(error);
}

function percentage(value: number): string {
    return `${(value * 100).toFixed(1)}%`;
}

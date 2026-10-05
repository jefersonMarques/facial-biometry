const storageKey = "faceproof.admin.key";
const publicBaseStorageKey = "faceproof.admin.public-base-url";

const scenarioLabels = {
    unknown: "Sem classificação",
    genuine_live: "Genuine live",
    impostor_live: "Impostor live",
    printed_photo: "Foto impressa",
    screen_photo: "Foto em tela",
    replay_video: "Replay de vídeo",
};

const statusLabels = {
    pending_document: "Documento pendente",
    processing_document: "Processando documento",
    biometry_pending: "Biometria pendente",
    approved: "Aprovado",
    review: "Review",
    rejected: "Rejeitado",
    expired: "Expirado",
};

const loginPanel = document.getElementById("loginPanel");
const loginForm = document.getElementById("loginForm");
const adminKeyInput = document.getElementById("adminKey");
const loginError = document.getElementById("loginError");
const dashboard = document.getElementById("dashboard");
const refreshButton = document.getElementById("refreshButton");
const logoutButton = document.getElementById("logoutButton");
const connectionState = document.getElementById("connectionState");
const checkForm = document.getElementById("checkForm");
const checkCpf = document.getElementById("checkCpf");
const checkMinimumDate = document.getElementById("checkMinimumDate");
const checkExpires = document.getElementById("checkExpires");
const checkCampaign = document.getElementById("checkCampaign");
const checkScenario = document.getElementById("checkScenario");
const checkPublicBaseUrl = document.getElementById("checkPublicBaseUrl");
const generateCheckButton = document.getElementById("generateCheckButton");
const generatedCheck = document.getElementById("generatedCheck");
const generatedCheckMeta = document.getElementById("generatedCheckMeta");
const generatedCheckUrl = document.getElementById("generatedCheckUrl");
const copyGeneratedCheck = document.getElementById("copyGeneratedCheck");
const generatedCheckFeedback = document.getElementById("generatedCheckFeedback");
const summaryCards = document.getElementById("summaryCards");
const scenarioRows = document.getElementById("scenarioRows");
const campaignForm = document.getElementById("campaignForm");
const campaignName = document.getElementById("campaignName");
const campaignDescription = document.getElementById("campaignDescription");
const campaignList = document.getElementById("campaignList");
const campaignFilter = document.getElementById("campaignFilter");
const filtersForm = document.getElementById("filtersForm");
const scenarioFilter = document.getElementById("scenarioFilter");
const statusFilter = document.getElementById("statusFilter");
const checkRows = document.getElementById("checkRows");
const lastUpdated = document.getElementById("lastUpdated");
const detailDialog = document.getElementById("detailDialog");
const detailTitle = document.getElementById("detailTitle");
const detailBody = document.getElementById("detailBody");
const closeDetail = document.getElementById("closeDetail");

let adminKey = sessionStorage.getItem(storageKey) ?? "";
let campaigns = [];

checkPublicBaseUrl.value = localStorage.getItem(publicBaseStorageKey) ?? "";

loginForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    const value = adminKeyInput.value.trim();
    if (!value) {
        return;
    }
    adminKey = value;
    sessionStorage.setItem(storageKey, value);
    await connect();
});

refreshButton.addEventListener("click", () => void loadDashboard());
logoutButton.addEventListener("click", () => {
    adminKey = "";
    sessionStorage.removeItem(storageKey);
    dashboard.hidden = true;
    loginPanel.hidden = false;
    logoutButton.hidden = true;
    setConnection(false);
});

checkForm.addEventListener("submit", async (event) => {
    event.preventDefault();

    const cpf = checkCpf.value.replace(/\D/g, "");
    const publicBaseUrl = checkPublicBaseUrl.value.trim();
    if (publicBaseUrl) {
        localStorage.setItem(publicBaseStorageKey, publicBaseUrl);
    } else {
        localStorage.removeItem(publicBaseStorageKey);
    }

    generateCheckButton.disabled = true;
    generatedCheckFeedback.textContent = "";
    try {
        const response = await api("/v1/admin/checks", {
            method: "POST",
            body: JSON.stringify({
                cpf,
                minimumDocumentDate: checkMinimumDate.value,
                expiresInMinutes: Number(checkExpires.value),
                campaignId: checkCampaign.value,
                scenario: checkScenario.value,
                publicBaseUrl,
            }),
        });

        generatedCheck.hidden = false;
        generatedCheckUrl.value = response.verificationUrl;
        generatedCheckMeta.textContent =
            `${response.id} · ${scenarioLabel(response.scenario)} · esperado: ${response.expectedDecision || "—"} · expira ${dateTime(response.expiresAt)}`;
        generatedCheckFeedback.textContent = "Link registrado e pronto para envio.";
        checkCpf.value = "";

        const summary = await api("/v1/admin/summary");
        renderSummary(summary);
        await loadChecks();
    } catch (error) {
        generatedCheck.hidden = false;
        generatedCheckUrl.value = "";
        generatedCheckMeta.textContent = "Não foi possível gerar o link.";
        generatedCheckFeedback.textContent = errorMessage(error);
    } finally {
        generateCheckButton.disabled = false;
    }
});

copyGeneratedCheck.addEventListener("click", async () => {
    const value = generatedCheckUrl.value.trim();
    if (!value) {
        return;
    }

    try {
        await navigator.clipboard.writeText(value);
        generatedCheckFeedback.textContent = "Link copiado.";
    } catch {
        generatedCheckUrl.focus();
        generatedCheckUrl.select();
        document.execCommand("copy");
        generatedCheckFeedback.textContent = "Link copiado.";
    }
});

filtersForm.addEventListener("submit", (event) => {
    event.preventDefault();
    void loadChecks();
});

campaignForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    const name = campaignName.value.trim();
    if (!name) {
        return;
    }

    const button = campaignForm.querySelector("button");
    button.disabled = true;
    try {
        await api("/v1/admin/campaigns", {
            method: "POST",
            body: JSON.stringify({
                name,
                description: campaignDescription.value.trim(),
            }),
        });
        campaignForm.reset();
        await loadCampaigns();
        await loadChecks();
    } catch (error) {
        window.alert(errorMessage(error));
    } finally {
        button.disabled = false;
    }
});

checkRows.addEventListener("click", (event) => {
    const row = event.target.closest("tr[data-check-id]");
    if (!row) {
        return;
    }
    void openDetail(row.dataset.checkId);
});

closeDetail.addEventListener("click", () => detailDialog.close());
detailDialog.addEventListener("click", (event) => {
    if (event.target === detailDialog) {
        detailDialog.close();
    }
});

if (adminKey) {
    void connect();
}

async function connect() {
    loginError.hidden = true;
    try {
        await loadDashboard();
        loginPanel.hidden = true;
        dashboard.hidden = false;
        logoutButton.hidden = false;
        setConnection(true);
    } catch (error) {
        setConnection(false);
        loginError.hidden = false;
        loginError.textContent = errorMessage(error);
        sessionStorage.removeItem(storageKey);
        adminKey = "";
    }
}

async function loadDashboard() {
    if (!adminKey) {
        return;
    }

    refreshButton.disabled = true;
    try {
        const [summary] = await Promise.all([
            api("/v1/admin/summary"),
            loadCampaigns(),
            loadChecks(),
        ]);
        renderSummary(summary);
        lastUpdated.textContent = `Atualizado ${new Date().toLocaleTimeString("pt-BR")}`;
        setConnection(true);
    } catch (error) {
        setConnection(false);
        throw error;
    } finally {
        refreshButton.disabled = false;
    }
}

async function loadCampaigns() {
    const response = await api("/v1/admin/campaigns");
    campaigns = response.items ?? [];
    renderCampaigns(campaigns);
    renderCampaignFilter(campaigns);
    renderCheckCampaigns(campaigns);
}

async function loadChecks() {
    const params = new URLSearchParams({ limit: "250" });
    if (scenarioFilter.value) {
        params.set("scenario", scenarioFilter.value);
    }
    if (statusFilter.value) {
        params.set("status", statusFilter.value);
    }
    if (campaignFilter.value) {
        params.set("campaignId", campaignFilter.value);
    }

    const response = await api(`/v1/admin/checks?${params.toString()}`);
    renderChecks(response.items ?? []);
}

function renderSummary(summary) {
    const cards = [
        ["Verificações", integer(summary.total), `${integer(summary.completed)} concluídas`],
        ["Aprovadas", integer(summary.approved), "decisão final"],
        ["Rejeitadas", integer(summary.rejected), "decisão final"],
        ["Conclusão", percentage(summary.completionRate), `${integer(summary.pending)} pendentes`],
        ["Face média", score(summary.avgFaceSimilarity), "similaridade cosseno"],
        ["Liveness médio", percentage(summary.avgLiveness), "score servidor"],
        ["PAD médio", percentage(summary.avgPassivePad), "passive PAD"],
        ["Qualidade média", percentage(summary.avgQuality), "frames finais"],
        ["Secure Core OK", integer(summary.nativeOk), `${integer(summary.nativePartial)} partial · ${integer(summary.nativeDrift)} drift`],
    ];

    summaryCards.innerHTML = cards.map(([label, value, note]) => `
        <article class="metric-card">
            <span>${escapeHtml(label)}</span>
            <strong>${escapeHtml(value)}</strong>
            <small>${escapeHtml(note)}</small>
        </article>
    `).join("");

    const scenarios = summary.scenarios ?? [];
    scenarioRows.innerHTML = scenarios.length
        ? scenarios.map((item) => {
            const accuracy = item.expectedEvaluated > 0
                ? percentage(item.expectedCorrect / item.expectedEvaluated)
                : "—";
            return `
                <tr>
                    <td>${escapeHtml(scenarioLabel(item.scenario))}</td>
                    <td>${integer(item.total)}</td>
                    <td>${integer(item.approved)}</td>
                    <td>${integer(item.rejected)}</td>
                    <td>${accuracy}</td>
                </tr>
            `;
        }).join("")
        : '<tr><td colspan="5" class="empty-state">Nenhum teste registrado ainda.</td></tr>';
}

function renderCampaigns(items) {
    campaignList.innerHTML = items.length
        ? items.map((item) => `
            <div class="campaign-item">
                <div>
                    <strong>${escapeHtml(item.name)}</strong>
                    <div class="muted">${escapeHtml(item.description || "Sem descrição")}</div>
                </div>
                <code>${escapeHtml(item.id)}</code>
            </div>
        `).join("")
        : '<div class="empty-state">Crie uma campanha para agrupar os próximos testes.</div>';
}

function renderCheckCampaigns(items) {
    const current = checkCampaign.value;
    checkCampaign.innerHTML = '<option value="">Sem campanha</option>' +
        items.map((item) => `<option value="${escapeAttribute(item.id)}">${escapeHtml(item.name)}</option>`).join("");
    if (items.some((item) => item.id === current)) {
        checkCampaign.value = current;
    }
}

function renderCampaignFilter(items) {
    const current = campaignFilter.value;
    campaignFilter.innerHTML = '<option value="">Todas as campanhas</option>' +
        items.map((item) => `<option value="${escapeAttribute(item.id)}">${escapeHtml(item.name)}</option>`).join("");
    if (items.some((item) => item.id === current)) {
        campaignFilter.value = current;
    }
}

function renderChecks(items) {
    checkRows.innerHTML = items.length
        ? items.map((item) => `
            <tr data-check-id="${escapeAttribute(item.checkId)}">
                <td>${dateTime(item.createdAt)}</td>
                <td><code>${escapeHtml(shortID(item.checkId))}</code></td>
                <td>
                    ${escapeHtml(scenarioLabel(item.scenario))}
                    ${item.campaignName ? `<div class="muted">${escapeHtml(item.campaignName)}</div>` : ""}
                </td>
                <td>${statusBadge(item.status, item.decision)}</td>
                <td>${nullableScore(item.faceSimilarity)}</td>
                <td>${nullablePercent(item.passivePadScore)}</td>
                <td>${nullablePercent(item.livenessScore)}</td>
                <td>${nullablePercent(item.qualityScore)}</td>
                <td>${nativeBadge(item.nativeStatus)}</td>
            </tr>
        `).join("")
        : '<tr><td colspan="9" class="empty-state">Nenhuma verificação para estes filtros.</td></tr>';
}

async function openDetail(checkId) {
    try {
        const detail = await api(`/v1/admin/checks/${encodeURIComponent(checkId)}`);
        detailTitle.textContent = checkId;
        detailBody.innerHTML = renderDetail(detail);
        detailDialog.showModal();
    } catch (error) {
        window.alert(errorMessage(error));
    }
}

function renderDetail(detail) {
    const geometry = objectValue(detail.geometry);
    const shadow = objectValue(detail.nativeShadow);
    const runtime = objectValue(detail.runtime);
    const protocol = objectValue(detail.captureProtocol);
    const documentInfo = objectValue(detail.document);

    const metrics = [
        ["Cenário", scenarioLabel(detail.scenario)],
        ["Esperado", detail.expectedDecision || "—"],
        ["Resultado", detail.decision || detail.status],
        ["Campanha", detail.campaignName || "—"],
        ["Face", nullableScore(detail.faceSimilarity)],
        ["Limiar", nullableScore(detail.matchThreshold)],
        ["Margem", signedScore(detail.margin)],
        ["Liveness", nullablePercent(detail.livenessScore)],
        ["PAD", nullablePercent(detail.passivePadScore)],
        ["Quality", nullablePercent(detail.qualityScore)],
        ["Guided capture", nullablePercent(detail.guidedCaptureScore)],
        ["Secure Core", detail.nativeStatus || "—"],
        ["Duração", duration(detail.durationMs)],
        ["Tentativas documento", integer(detail.documentAttempts)],
        ["Sessões biométricas", integer(detail.biometricSessions)],
        ["Subject hash", detail.subjectHash ? shortHash(detail.subjectHash) : "—"],
    ];

    return `
        <div class="detail-grid">
            ${metrics.map(([label, value]) => `
                <div class="detail-metric">
                    <span>${escapeHtml(label)}</span>
                    <strong>${escapeHtml(String(value))}</strong>
                </div>
            `).join("")}
        </div>

        ${detailSection("Geometria · cliente não confiável", geometry)}
        ${detailSection("Secure Core C++ shadow", shadow)}
        ${detailSection("Runtime", runtime)}
        ${detailSection("Capture protocol", protocol)}
        ${detailSection("Documento · sem PII", documentInfo)}
        ${detailSection("Diagnósticos", detail.diagnostics)}

        <section class="detail-section">
            <h3>Timeline</h3>
            <div class="timeline">
                ${renderEvents(detail.events ?? [])}
            </div>
        </section>
    `;
}

function renderEvents(events) {
    if (!events.length) {
        return '<div class="empty-state">Nenhum evento registrado.</div>';
    }
    return events.map((event) => `
        <div class="timeline-item">
            <span>${dateTime(event.occurredAt)}</span>
            <strong>${escapeHtml(event.eventType)}</strong>
            <pre>${escapeHtml(prettyJSON(event.payload))}</pre>
        </div>
    `).join("");
}

function detailSection(title, value) {
    const normalized = objectValue(value);
    if (normalized === null || normalized === undefined ||
        (typeof normalized === "object" && !Array.isArray(normalized) && Object.keys(normalized).length === 0)) {
        return "";
    }
    return `
        <section class="detail-section">
            <h3>${escapeHtml(title)}</h3>
            <pre class="json-block">${escapeHtml(prettyJSON(normalized))}</pre>
        </section>
    `;
}

async function api(path, init = {}) {
    const headers = new Headers(init.headers);
    headers.set("Authorization", `Bearer ${adminKey}`);
    if (init.body && !headers.has("Content-Type")) {
        headers.set("Content-Type", "application/json");
    }

    const response = await fetch(path, {
        ...init,
        headers,
        cache: "no-store",
        credentials: "omit",
        referrerPolicy: "no-referrer",
    });

    const contentType = response.headers.get("content-type") ?? "";
    const payload = contentType.includes("application/json")
        ? await response.json()
        : { error: await response.text() };

    if (!response.ok) {
        if (response.status === 401) {
            sessionStorage.removeItem(storageKey);
        }
        throw new Error(payload.error || `HTTP ${response.status}`);
    }
    return payload;
}

function setConnection(connected) {
    connectionState.textContent = connected ? "Conectado" : "Desconectado";
    connectionState.className = `status-pill ${connected ? "status-on" : "status-off"}`;
}

function statusBadge(status, decision) {
    const value = decision || status || "";
    const tone = value === "approved"
        ? "approved"
        : value === "rejected" || value === "expired"
            ? "rejected"
            : value === "review"
                ? "review"
                : "pending";
    return `<span class="badge badge-${tone}">${escapeHtml(statusLabels[status] || value || "—")}</span>`;
}

function nativeBadge(status) {
    if (!status) {
        return "—";
    }
    const tone = status === "ok" ? "ok" : status === "drift" ? "drift" : "partial";
    return `<span class="badge badge-${tone}">${escapeHtml(status.toUpperCase())}</span>`;
}

function scenarioLabel(value) {
    return scenarioLabels[value] || value || "Sem classificação";
}

function percentage(value) {
    if (!Number.isFinite(Number(value))) {
        return "—";
    }
    return `${(Number(value) * 100).toFixed(1)}%`;
}

function nullablePercent(value) {
    return value === null || value === undefined ? "—" : percentage(value);
}

function score(value) {
    return Number.isFinite(Number(value)) ? Number(value).toFixed(3) : "—";
}

function nullableScore(value) {
    return value === null || value === undefined ? "—" : score(value);
}

function signedScore(value) {
    if (value === null || value === undefined || !Number.isFinite(Number(value))) {
        return "—";
    }
    const number = Number(value);
    return `${number >= 0 ? "+" : ""}${number.toFixed(3)}`;
}

function integer(value) {
    return Number.isFinite(Number(value)) ? Math.round(Number(value)).toLocaleString("pt-BR") : "0";
}

function duration(value) {
    if (value === null || value === undefined || !Number.isFinite(Number(value))) {
        return "—";
    }
    const seconds = Number(value) / 1000;
    return seconds < 60 ? `${seconds.toFixed(1)} s` : `${(seconds / 60).toFixed(1)} min`;
}

function dateTime(value) {
    if (!value) {
        return "—";
    }
    const date = new Date(value);
    return Number.isNaN(date.getTime())
        ? "—"
        : date.toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "medium" });
}

function shortID(value) {
    if (!value) {
        return "—";
    }
    return value.length > 18 ? `${value.slice(0, 18)}…` : value;
}

function shortHash(value) {
    return value.length > 20 ? `${value.slice(0, 20)}…` : value;
}

function objectValue(value) {
    if (value === null || value === undefined) {
        return null;
    }
    if (typeof value === "string") {
        try {
            return JSON.parse(value);
        } catch {
            return value;
        }
    }
    return value;
}

function prettyJSON(value) {
    if (value === null || value === undefined) {
        return "";
    }
    return typeof value === "string" ? value : JSON.stringify(value, null, 2);
}

function errorMessage(error) {
    return error instanceof Error ? error.message : String(error);
}

function escapeAttribute(value) {
    return escapeHtml(String(value));
}

function escapeHtml(value) {
    return String(value)
        .replaceAll("&", "&amp;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;")
        .replaceAll('"', "&quot;")
        .replaceAll("'", "&#039;");
}

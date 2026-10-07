const authStorageKey = "faceproof.demo.auth";
const publicBaseStorageKey = "faceproof.demo.public-base-url";

const flowLabels = {
    cnh: "CNH + Face",
    face_enrollment: "Cadastro facial",
    face_verification: "Validação facial",
    photo_verification: "Foto de referência",
};

const statusLabels = {
    pending_document: "Aguardando CNH",
    processing_document: "Validando CNH",
    biometry_pending: "Aguardando biometria",
    approved: "Aprovada",
    review: "Em revisão",
    rejected: "Rejeitada",
    expired: "Expirada",
};

const loginPanel = document.getElementById("loginPanel");
const loginForm = document.getElementById("loginForm");
const demoUser = document.getElementById("demoUser");
const demoPassword = document.getElementById("demoPassword");
const loginError = document.getElementById("loginError");
const appShell = document.getElementById("appShell");
const connectionState = document.getElementById("connectionState");
const refreshButton = document.getElementById("refreshButton");
const logoutButton = document.getElementById("logoutButton");
const openGenerator = document.getElementById("openGenerator");
const closeGenerator = document.getElementById("closeGenerator");
const generatorPanel = document.getElementById("generatorPanel");
const summaryCards = document.getElementById("summaryCards");
const checkForm = document.getElementById("checkForm");
const faceActionField = document.getElementById("faceActionField");
const displayNameField = document.getElementById("displayNameField");
const subjectField = document.getElementById("subjectField");
const cpfField = document.getElementById("cpfField");
const minimumDateField = document.getElementById("minimumDateField");
const photoField = document.getElementById("photoField");
const checkDisplayName = document.getElementById("checkDisplayName");
const checkSubjectId = document.getElementById("checkSubjectId");
const checkCpf = document.getElementById("checkCpf");
const checkMinimumDate = document.getElementById("checkMinimumDate");
const checkExpires = document.getElementById("checkExpires");
const checkPublicBaseUrl = document.getElementById("checkPublicBaseUrl");
const referencePhoto = document.getElementById("referencePhoto");
const referencePhotoLabel = document.getElementById("referencePhotoLabel");
const referencePhotoPreview = document.getElementById("referencePhotoPreview");
const generateCheckButton = document.getElementById("generateCheckButton");
const generatedCheck = document.getElementById("generatedCheck");
const generatedCheckMeta = document.getElementById("generatedCheckMeta");
const generatedCheckUrl = document.getElementById("generatedCheckUrl");
const copyGeneratedCheck = document.getElementById("copyGeneratedCheck");
const generatedCheckFeedback = document.getElementById("generatedCheckFeedback");
const statusFilter = document.getElementById("statusFilter");
const verificationList = document.getElementById("verificationList");
const lastUpdated = document.getElementById("lastUpdated");
const detailDialog = document.getElementById("detailDialog");
const detailTitle = document.getElementById("detailTitle");
const detailBody = document.getElementById("detailBody");
const closeDetail = document.getElementById("closeDetail");

let authHeader = sessionStorage.getItem(authStorageKey) || "";
let referencePhotoDataURL = "";

checkPublicBaseUrl.value = localStorage.getItem(publicBaseStorageKey) || "";
checkMinimumDate.value = defaultMinimumDate();
syncFlowFields();

loginForm.addEventListener("submit", (event) => {
    event.preventDefault();
    const username = demoUser.value.trim();
    const password = demoPassword.value;
    if (!username || !password) return;

    authHeader = "Basic " + btoa(unescape(encodeURIComponent(username + ":" + password)));
    void connect();
});

logoutButton.addEventListener("click", logout);
refreshButton.addEventListener("click", () => void loadDashboard());
openGenerator.addEventListener("click", () => {
    generatorPanel.hidden = false;
    generatorPanel.scrollIntoView({ behavior: "smooth", block: "start" });
});
closeGenerator.addEventListener("click", () => { generatorPanel.hidden = true; });
statusFilter.addEventListener("change", () => void loadChecks());

document.querySelectorAll('input[name="flowFamily"], input[name="faceAction"]').forEach((input) => {
    input.addEventListener("change", syncFlowFields);
});

referencePhoto.addEventListener("change", () => void prepareReferencePhoto());

checkForm.addEventListener("submit", (event) => {
    event.preventDefault();
    void generateCheck();
});

copyGeneratedCheck.addEventListener("click", async () => {
    const value = generatedCheckUrl.value.trim();
    if (!value) return;
    try {
        await navigator.clipboard.writeText(value);
    } catch {
        generatedCheckUrl.select();
        document.execCommand("copy");
    }
    generatedCheckFeedback.textContent = "Link copiado.";
});

verificationList.addEventListener("click", (event) => {
    const card = event.target.closest("[data-check-id]");
    if (card) void openDetail(card.dataset.checkId);
});

closeDetail.addEventListener("click", () => detailDialog.close());
detailDialog.addEventListener("click", (event) => {
    if (event.target === detailDialog) detailDialog.close();
});

if (authHeader) void connect();

async function connect() {
    loginError.hidden = true;
    try {
        await api("/v1/admin/summary");
        sessionStorage.setItem(authStorageKey, authHeader);
        loginPanel.hidden = true;
        appShell.hidden = false;
        setConnected(true);
        await loadDashboard();
    } catch (error) {
        authHeader = "";
        sessionStorage.removeItem(authStorageKey);
        setConnected(false);
        loginError.hidden = false;
        loginError.textContent = errorMessage(error);
    }
}

function logout() {
    authHeader = "";
    sessionStorage.removeItem(authStorageKey);
    appShell.hidden = true;
    loginPanel.hidden = false;
    demoPassword.value = "";
    setConnected(false);
}

async function loadDashboard() {
    refreshButton.disabled = true;
    try {
        const [summary] = await Promise.all([
            api("/v1/admin/summary"),
            loadChecks(),
        ]);
        renderSummary(summary);
        lastUpdated.textContent = "Atualizado " + new Date().toLocaleTimeString("pt-BR", {
            hour: "2-digit",
            minute: "2-digit",
        });
        setConnected(true);
    } catch (error) {
        setConnected(false);
        if (String(errorMessage(error)).includes("401")) logout();
        throw error;
    } finally {
        refreshButton.disabled = false;
    }
}

async function loadChecks() {
    const params = new URLSearchParams({ limit: "60" });
    if (statusFilter.value) params.set("status", statusFilter.value);

    const response = await api("/v1/admin/checks?" + params.toString());
    renderChecks(response.items || []);
}

function renderSummary(summary) {
    const cards = [
        ["Verificações", integer(summary.total), ""],
        ["Aprovadas", integer(summary.approved), "accent"],
        ["Rejeitadas", integer(summary.rejected), ""],
        ["Pendentes", integer(summary.pending), ""],
    ];

    summaryCards.innerHTML = cards.map(([label, value, tone]) => `
        <article class="summary-card ${tone}">
            <span>${escapeHtml(label)}</span>
            <strong>${escapeHtml(value)}</strong>
        </article>
    `).join("");
}

function renderChecks(items) {
    if (!items.length) {
        verificationList.innerHTML = '<div class="empty-state">Nenhuma biometria encontrada.</div>';
        return;
    }

    verificationList.innerHTML = items.map((item) => {
        const displayName = item.displayName || item.subjectId || shortID(item.checkId);
        const flowType = item.flowType || "cnh";

        return `
            <article class="verification-card" data-check-id="${escapeAttribute(item.checkId)}">
                <div class="face-pair">
                    ${faceThumb(item.referencePhotoDataUrl, "Referência")}
                    <div class="face-arrow">→</div>
                    ${faceThumb(item.capturedPhotoDataUrl, "Captura")}
                </div>

                <div class="verification-copy">
                    <span class="eyebrow">${escapeHtml(flowLabels[flowType] || flowType)}</span>
                    <h3>${escapeHtml(displayName)}</h3>
                    <p>${escapeHtml(dateTime(item.createdAt))}</p>
                    <div class="verification-meta">
                        <span>Similaridade <strong>${flowType === "face_enrollment" ? "—" : normalizedSimilarity(item.faceSimilarity)}</strong></span>
                        <span>Liveness <strong>${nullablePercent(item.livenessScore)}</strong></span>
                        <span>PAD <strong>${nullablePercent(item.passivePadScore)}</strong></span>
                    </div>
                </div>

                <div class="verification-status">
                    ${statusBadge(item.status, item.decision)}
                </div>
            </article>
        `;
    }).join("");
}

function faceThumb(source, label) {
    return `
        <div class="face-thumb" title="${escapeAttribute(label)}">
            ${source
                ? `<img src="${escapeAttribute(source)}" alt="${escapeAttribute(label)}">`
                : '<div class="face-placeholder">·</div>'}
        </div>
    `;
}

function syncFlowFields() {
    const family = selectedValue("flowFamily") || "cnh";
    const isCNH = family === "cnh";
    const isFace = family === "face";
    const isPhoto = family === "photo";

    faceActionField.hidden = !isFace;
    displayNameField.hidden = isCNH;
    subjectField.hidden = isCNH;
    cpfField.hidden = !isCNH;
    minimumDateField.hidden = !isCNH;
    photoField.hidden = !isPhoto;

    checkCpf.required = isCNH;
    checkMinimumDate.required = isCNH;
    checkSubjectId.required = !isCNH;
    referencePhoto.required = isPhoto;

    generatedCheck.hidden = true;
    generatedCheckFeedback.textContent = "";
}

async function prepareReferencePhoto() {
    const file = referencePhoto.files?.[0];
    referencePhotoDataURL = "";
    referencePhotoPreview.hidden = true;
    referencePhotoLabel.textContent = "Selecionar foto";
    if (!file) return;

    try {
        referencePhotoDataURL = await resizeImage(file, 1000, .86);
        referencePhotoPreview.src = referencePhotoDataURL;
        referencePhotoPreview.hidden = false;
        referencePhotoLabel.textContent = file.name;
    } catch (error) {
        generatedCheckFeedback.textContent = errorMessage(error);
        referencePhoto.value = "";
    }
}

async function generateCheck() {
    const flowType = selectedFlowType();
    const publicBaseUrl = checkPublicBaseUrl.value.trim();
    if (publicBaseUrl) {
        localStorage.setItem(publicBaseStorageKey, publicBaseUrl);
    } else {
        localStorage.removeItem(publicBaseStorageKey);
    }

    const payload = {
        flowType,
        displayName: checkDisplayName.value.trim(),
        subjectId: checkSubjectId.value.trim(),
        expiresInMinutes: Number(checkExpires.value),
        publicBaseUrl,
        scenario: "genuine_live",
        expectedDecision: "approved",
    };

    if (flowType === "cnh") {
        payload.cpf = checkCpf.value.replace(/\D/g, "");
        payload.minimumDocumentDate = checkMinimumDate.value;
    }
    if (flowType === "photo_verification") {
        if (!referencePhotoDataURL) {
            generatedCheckFeedback.textContent = "Selecione uma foto de referência.";
            return;
        }
        payload.referencePhotoDataUrl = referencePhotoDataURL;
    }

    generateCheckButton.disabled = true;
    generatedCheckFeedback.textContent = "Gerando...";

    try {
        const response = await api("/v1/admin/checks", {
            method: "POST",
            body: JSON.stringify(payload),
        });

        generatedCheck.hidden = false;
        generatedCheckUrl.value = response.verificationUrl;
        generatedCheckMeta.textContent =
            (flowLabels[response.flowType || flowType] || flowType) +
            " · expira " + dateTime(response.expiresAt);
        generatedCheckFeedback.textContent = "Link pronto para demonstração.";
        await loadDashboard();
    } catch (error) {
        generatedCheck.hidden = true;
        generatedCheckFeedback.textContent = errorMessage(error);
    } finally {
        generateCheckButton.disabled = false;
    }
}

function selectedFlowType() {
    const family = selectedValue("flowFamily") || "cnh";
    if (family === "face") return selectedValue("faceAction") || "face_enrollment";
    if (family === "photo") return "photo_verification";
    return "cnh";
}

function selectedValue(name) {
    return document.querySelector(`input[name="${name}"]:checked`)?.value || "";
}

async function openDetail(checkId) {
    try {
        const detail = await api("/v1/admin/checks/" + encodeURIComponent(checkId));
        detailTitle.textContent = detail.displayName || detail.subjectId || shortID(checkId);
        detailBody.innerHTML = renderDetail(detail);
        detailDialog.showModal();
    } catch (error) {
        window.alert(errorMessage(error));
    }
}

function renderDetail(detail) {
    const flowType = detail.flowType || "cnh";
    const metrics = [
        ["Tipo", flowLabels[flowType] || flowType],
        ["Resultado", statusLabels[detail.status] || detail.decision || detail.status],
        ["Similaridade", flowType === "face_enrollment" ? "—" : normalizedSimilarity(detail.faceSimilarity)],
        ["Liveness", nullablePercent(detail.livenessScore)],
        ["Passive PAD", nullablePercent(detail.passivePadScore)],
        ["Qualidade", nullablePercent(detail.qualityScore)],
        ["Secure Core", detail.nativeStatus || "—"],
        ["Duração", duration(detail.durationMs)],
        ["ID", shortID(detail.checkId)],
    ];

    return `
        <div class="detail-face-pair">
            <div class="detail-face">
                ${detail.referencePhotoDataUrl
                    ? `<img src="${escapeAttribute(detail.referencePhotoDataUrl)}" alt="Foto esperada">`
                    : '<div class="face-placeholder">Referência</div>'}
            </div>
            <div class="detail-arrow">→</div>
            <div class="detail-face">
                ${detail.capturedPhotoDataUrl
                    ? `<img src="${escapeAttribute(detail.capturedPhotoDataUrl)}" alt="Captura realizada">`
                    : '<div class="face-placeholder">Captura</div>'}
            </div>
        </div>

        <div class="detail-grid">
            ${metrics.map(([label, value]) => `
                <div class="detail-metric">
                    <span>${escapeHtml(label)}</span>
                    <strong>${escapeHtml(String(value ?? "—"))}</strong>
                </div>
            `).join("")}
        </div>

        ${detailSection("Geometria", detail.geometry)}
        ${detailSection("Runtime", detail.runtime)}
        ${detailSection("Diagnósticos", detail.diagnostics)}

        <section class="detail-section">
            <h3>Timeline</h3>
            <div class="timeline">
                ${renderEvents(detail.events || [])}
            </div>
        </section>
    `;
}

function detailSection(title, value) {
    const normalized = objectValue(value);
    if (normalized === null || normalized === undefined ||
        (typeof normalized === "object" && !Array.isArray(normalized) &&
        Object.keys(normalized).length === 0)) {
        return "";
    }
    return `
        <section class="detail-section">
            <h3>${escapeHtml(title)}</h3>
            <pre>${escapeHtml(prettyJSON(normalized))}</pre>
        </section>
    `;
}

function renderEvents(events) {
    if (!events.length) return '<div class="empty-state">Sem eventos.</div>';
    return events.map((event) => `
        <div class="timeline-item">
            <span>${dateTime(event.occurredAt)}</span>
            <strong>${escapeHtml(event.eventType)}</strong>
            ${event.payload ? `<pre>${escapeHtml(prettyJSON(event.payload))}</pre>` : ""}
        </div>
    `).join("");
}

async function api(path, init = {}) {
    const headers = new Headers(init.headers);
    headers.set("Authorization", authHeader);
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

    const contentType = response.headers.get("content-type") || "";
    const payload = contentType.includes("application/json")
        ? await response.json()
        : { error: await response.text() };

    if (!response.ok) {
        throw new Error((payload.error || "HTTP " + response.status) + " (" + response.status + ")");
    }
    return payload;
}

async function resizeImage(file, maxSide, quality) {
    if (!file.type.startsWith("image/")) throw new Error("Selecione uma imagem válida.");

    const bitmap = await createImageBitmap(file);
    const scale = Math.min(1, maxSide / Math.max(bitmap.width, bitmap.height));
    const width = Math.max(1, Math.round(bitmap.width * scale));
    const height = Math.max(1, Math.round(bitmap.height * scale));

    const canvas = document.createElement("canvas");
    canvas.width = width;
    canvas.height = height;
    const context = canvas.getContext("2d", { alpha: false });
    if (!context) throw new Error("Não foi possível preparar a foto.");

    context.drawImage(bitmap, 0, 0, width, height);
    bitmap.close();

    return canvas.toDataURL("image/jpeg", quality);
}

function setConnected(connected) {
    connectionState.classList.toggle("connected", connected);
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
    return '<span class="badge badge-' + tone + '">' +
        escapeHtml(statusLabels[status] || value || "—") +
        "</span>";
}

function defaultMinimumDate() {
    const date = new Date();
    date.setFullYear(date.getFullYear() - 1);
    return date.toISOString().slice(0, 10);
}

function nullablePercent(value) {
    return value === null || value === undefined || !Number.isFinite(Number(value))
        ? "—"
        : (Number(value) * 100).toFixed(1) + "%";
}

function normalizedSimilarity(value) {
    if (value === null || value === undefined || !Number.isFinite(Number(value))) {
        return "—";
    }
    const clamped = Math.max(-1, Math.min(1, Number(value)));
    return Math.round(((clamped + 1) / 2) * 100) + "%";
}

function nullableScore(value) {
    return value === null || value === undefined || !Number.isFinite(Number(value))
        ? "—"
        : Number(value).toFixed(3);
}

function integer(value) {
    return Number.isFinite(Number(value))
        ? Math.round(Number(value)).toLocaleString("pt-BR")
        : "0";
}

function duration(value) {
    if (value === null || value === undefined || !Number.isFinite(Number(value))) return "—";
    const seconds = Number(value) / 1000;
    return seconds < 60 ? seconds.toFixed(1) + " s" : (seconds / 60).toFixed(1) + " min";
}

function dateTime(value) {
    if (!value) return "—";
    const date = new Date(value);
    return Number.isNaN(date.getTime())
        ? "—"
        : date.toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "short" });
}

function shortID(value) {
    const normalized = String(value || "");
    return normalized.length > 18 ? normalized.slice(0, 18) + "…" : normalized || "—";
}

function objectValue(value) {
    if (value === null || value === undefined) return null;
    if (typeof value === "string") {
        try { return JSON.parse(value); } catch { return value; }
    }
    return value;
}

function prettyJSON(value) {
    if (value === null || value === undefined) return "";
    return typeof value === "string" ? value : JSON.stringify(value, null, 2);
}

function errorMessage(error) {
    return error instanceof Error ? error.message : String(error);
}

function escapeAttribute(value) {
    return escapeHtml(String(value || ""));
}

function escapeHtml(value) {
    return String(value ?? "")
        .replaceAll("&", "&amp;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;")
        .replaceAll('"', "&quot;")
        .replaceAll("'", "&#039;");
}

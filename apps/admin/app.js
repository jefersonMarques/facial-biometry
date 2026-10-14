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
const calibrationDays = document.getElementById("calibrationDays");
const calibrationRuntime = document.getElementById("calibrationRuntime");
const calibrationPair = document.getElementById("calibrationPair");
const calibrationStatus = document.getElementById("calibrationStatus");
const calibrationCounts = document.getElementById("calibrationCounts");
const calibrationHistograms = document.getElementById("calibrationHistograms");
const calibrationHealth = document.getElementById("calibrationHealth");
let calibrationCache = null;
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
calibrationDays.addEventListener("change", () => void loadCalibration());
calibrationRuntime.addEventListener("change", () => void loadCalibration());
calibrationPair.addEventListener("change", () => {
    if (calibrationCache) renderCalibration(calibrationCache);
});
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

detailBody.addEventListener("click", (event) => {
    if (!event.target.closest("#saveCalibrationTruth")) return;
    const button = detailBody.querySelector("#saveCalibrationTruth");
    const label = detailBody.querySelector("#calibrationTruthLabel")?.value || "unknown";
    const source = label === "unknown" ? "" :
        detailBody.querySelector("#calibrationTruthSource")?.value || "";
    const checkId = detailBody.querySelector("#calibrationTruthCheckId")?.value;
    const message = detailBody.querySelector("#calibrationTruthMessage");
    if (!checkId || (label !== "unknown" && !source)) {
        if (message) message.textContent = "Selecione a origem da confirmação independente.";
        return;
    }
    if (label !== "unknown" && !window.confirm(
        "Confirma que esta classificação foi estabelecida por evidência independente da decisão biométrica?"
    )) return;
    button.disabled = true;
    void api("/v1/admin/checks/" + encodeURIComponent(checkId) + "/truth", {
        method: "PATCH",
        body: JSON.stringify({label, source}),
    }).then(async () => {
        const detail = await api("/v1/admin/checks/" + encodeURIComponent(checkId));
        detailBody.innerHTML = renderDetail(detail);
        await loadCalibration();
    }).catch(error => {
        if (message) message.textContent = errorMessage(error);
    }).finally(() => { button.disabled = false; });
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
        await loadCalibration();
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

const calibrationMethods = {
    production_cubic: "Bicúbica atual",
    nearest: "Nearest Neighbor",
    linear: "Bilinear",
    lanczos4: "Lanczos4",
};

async function loadCalibration() {
    calibrationStatus.textContent = "Calculando estatísticas...";
    const params = new URLSearchParams({
        days: calibrationDays.value,
        runtime: calibrationRuntime.value,
    });
    try {
        const report = await api("/v1/admin/calibration?" + params.toString());
        calibrationCache = report;
        renderCalibration(report);
    } catch (error) {
        calibrationStatus.textContent = "Calibração indisponível: " + errorMessage(error);
        calibrationCounts.replaceChildren();
        calibrationHistograms.replaceChildren();
        calibrationHealth.replaceChildren();
    }
}

function calibrationNumber(value, digits = 4) {
    return value === null || value === undefined || !Number.isFinite(Number(value))
        ? "—" : Number(value).toFixed(digits).replace(".", ",");
}

function renderCalibration(report) {
    // Aggregated metrics only; no check IDs, names, images or embeddings.
    const selectedRuntime = calibrationRuntime.value;
    calibrationRuntime.replaceChildren(new Option("Todos os runtimes", ""));
    for (const runtime of report.runtimes || []) {
        if (!runtime.runtimeId) continue;
        const label = runtime.runtimeId === "unknown"
            ? "Runtime não identificado"
            : runtime.runtimeId.slice(0, 12) + "… (" + runtime.embeddingModel + ")";
        calibrationRuntime.add(new Option(label, runtime.runtimeId));
    }
    if ([...calibrationRuntime.options].some(option => option.value === selectedRuntime)) {
        calibrationRuntime.value = selectedRuntime;
    }
    const mixed = (report.runtimes || []).length > 1 && !report.runtimeFilter;
    calibrationStatus.textContent =
        integer(report.completedCnh) + " CNHs concluídas no período · " +
        integer(report.withBenchmark) + " com benchmark. " +
        (mixed ? "Múltiplos runtimes: use o filtro para não misturar versões." :
        "Estatísticas observacionais; sem calibração automática.");

    calibrationCounts.replaceChildren();
    const summaries = [
        ["Aprovadas", report.approved],
        ["Rejeitadas", report.rejected],
        ["Verdade desconhecida", report.unknownTruth],
        ["Titulares confirmados", report.confirmedGenuine],
        ["Impostores confirmados", report.confirmedImpostor],
        ["Qualidade média", report.avgQuality === null || report.avgQuality === undefined
            ? "—" : calibrationNumber(report.avgQuality * 100, 1) + "%"],
        ["Liveness médio", report.avgLiveness === null || report.avgLiveness === undefined
            ? "—" : calibrationNumber(report.avgLiveness * 100, 1) + "%"],
        ["PAD médio", report.avgPassivePad === null || report.avgPassivePad === undefined
            ? "—" : calibrationNumber(report.avgPassivePad * 100, 1) + "%"],
    ];
    for (const [label, value] of summaries) {
        const card = document.createElement("div");
        card.className = "calibration-stat";
        const title = document.createElement("span");
        title.textContent = label;
        const amount = document.createElement("strong");
        amount.textContent = typeof value === "string" ? value : integer(value);
        card.append(title, amount);
        calibrationCounts.append(card);
    }

    calibrationHistograms.replaceChildren();
    const pair = calibrationPair.value;
    const matching = (report.series || []).filter(item => item.pair === pair);
    if (!matching.length) {
        const note = document.createElement("p");
        note.textContent = "Ainda não há scores desta comparação no período selecionado.";
        calibrationHistograms.append(note);
    }
    for (const method of ["production_cubic", "nearest", "linear", "lanczos4"]) {
        const records = matching.filter(item => item.method === method);
        if (!records.length) continue;
        const chart = document.createElement("section");
        chart.className = "calibration-chart";
        const title = document.createElement("h3");
        title.textContent = calibrationMethods[method] || method;
        chart.append(title);
        const maxCount = Math.max(1, ...records.flatMap(item => item.histogram || []));
        for (const truth of [
            ["genuine_confirmed", "Titular confirmado"],
            ["impostor_confirmed", "Impostor confirmado"],
            ["unknown", "Sem confirmação independente"],
        ]) {
            const record = records.find(item => item.truthLabel === truth[0]);
            if (!record) continue;
            const name = document.createElement("p");
            name.className = "calibration-series-label";
            name.textContent = truth[1] + " · " + integer(record.compared) +
                " scores · média " + calibrationNumber(record.meanCosine);
            const bars = document.createElement("div");
            bars.className = "calibration-bars " + truth[0];
            (record.histogram || []).forEach((count, index) => {
                const bar = document.createElement("span");
                bar.style.height = Math.max(2, Math.round((count / maxCount) * 55)) + "px";
                bar.title = (index / 10 - 1).toFixed(1) + " até " +
                    ((index + 1) / 10 - 1).toFixed(1) + ": " + count;
                bars.append(bar);
            });
            const axis = document.createElement("div");
            axis.className = "calibration-axis";
            axis.textContent = "−1,0                                            0,0                                            +1,0";
            chart.append(name, bars, axis);
        }
        calibrationHistograms.append(chart);
    }

    calibrationHealth.replaceChildren();
    const head = document.createElement("h3");
    head.textContent = "Saúde do processamento experimental";
    calibrationHealth.append(head);
    const table = document.createElement("table");
    table.className = "benchmark-table";
    const captions = ["Método", "Tentativas", "Comparadas", "Falhas", "Qualidade média", "Duração média adicional"];
    const tr = table.createTHead().insertRow();
    captions.forEach(value => {
        const th = document.createElement("th");
        th.textContent = value;
        tr.append(th);
    });
    const tbody = table.createTBody();
    for (const item of report.methodHealth || []) {
        const row = tbody.insertRow();
        [
            calibrationMethods[item.method] || item.method,
            integer(item.attempts),
            integer(item.compared),
            integer(item.failed),
            item.avgQuality === null || item.avgQuality === undefined
                ? "—" : calibrationNumber(item.avgQuality * 100, 1) + "%",
            item.method === "production_cubic" ? "Reutilizada" :
                calibrationNumber(item.avgDurationMs, 0) + " ms",
        ].forEach(value => {
            const cell = row.insertCell();
            cell.textContent = value;
        });
    }
    calibrationHealth.append(table);
    if ((report.errors || []).length) {
        const heading = document.createElement("p");
        heading.className = "calibration-series-label";
        heading.textContent = "Falhas técnicas por método:";
        calibrationHealth.append(heading);
        const faults = document.createElement("p");
        faults.className = "calibration-notice";
        faults.textContent = report.errors.map(item =>
            (calibrationMethods[item.method] || item.method) + " · " +
            item.code + " (" + integer(item.count) + ")"
        ).join("  |  ");
        calibrationHealth.append(faults);
    }
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
        if (!detailDialog.open) detailDialog.showModal();
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

        <div class="detail-face-pair ${flowType === "cnh" ? "detail-face-triple" : ""}">
            <div class="detail-face-column">
                <div class="detail-face">
                    ${detail.referencePhotoDataUrl
                        ? `<img src="${escapeAttribute(detail.referencePhotoDataUrl)}" alt="Retrato da CNH PDF">`
                        : '<div class="face-placeholder">Referência</div>'}
                </div>
                <span>CNH PDF</span>
            </div>
            <div class="detail-arrow">↔</div>
            ${flowType === "cnh" ? `
                <div class="detail-face-column">
                    <div class="detail-face detail-face-vio">
                        ${detail.vioPhotoDataUrl
                            ? `<img src="${escapeAttribute(detail.vioPhotoDataUrl)}" alt="Retrato VIO original decodificado, baixa resolução">`
                            : '<div class="face-placeholder">VIO indisponível</div>'}
                    </div>
                    <span>VIO original</span>
                    ${detail.vioPhotoDataUrl
                        ? `<a class="vio-save" download="faceproof-vio-original.${detail.vioPhotoDataUrl.startsWith("data:image/jpeg;") ? "jpg" : "png"}" href="${escapeAttribute(detail.vioPhotoDataUrl)}">Salvar VIO</a>`
                        : '<small>Novas verificações em modo de demonstração</small>'}
                </div>
                <div class="detail-arrow">↔</div>
            ` : ""}
            <div class="detail-face-column">
                <div class="detail-face">
                    ${detail.capturedPhotoDataUrl
                        ? `<img src="${escapeAttribute(detail.capturedPhotoDataUrl)}" alt="Melhor captura biométrica">`
                        : '<div class="face-placeholder">Captura</div>'}
                </div>
                <span>Biometria</span>
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
        ${renderLowResolutionComparison(detail.events || [])}
        ${flowType === "cnh" ? renderCalibrationTruthControls(detail) : ""}

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

function renderCalibrationTruthControls(detail) {
    const choices = [
        ["unknown", "Desconhecido — não comprovado"],
        ["genuine_confirmed", "Titular confirmado"],
        ["impostor_confirmed", "Impostor confirmado"],
    ];
    const sources = [
        ["", "Selecione a evidência independente"],
        ["controlled_test", "Teste controlado"],
        ["manual_review", "Revisão documental independente"],
        ["external_verified", "Confirmação externa independente"],
    ];
    const option = (value, label, selected) =>
        "<option value=\"" + escapeAttribute(value) + "\"" +
        (value === selected ? " selected" : "") + ">" + escapeHtml(label) + "</option>";
    return "<section class=\"detail-section calibration-truth\">" +
        "<h3>Identidade confirmada independentemente</h3>" +
        "<p>Não derive esta informação da aprovação ou rejeição do FaceProof. " +
        "Os campos de cenário e resultado esperado também não são confirmação.</p>" +
        "<input id=\"calibrationTruthCheckId\" type=\"hidden\" value=\"" +
            escapeAttribute(detail.checkId) + "\">" +
        "<div class=\"calibration-truth-controls\">" +
            "<select id=\"calibrationTruthLabel\" aria-label=\"Identidade confirmada\">" +
                choices.map(c => option(c[0], c[1], detail.truthLabel || "unknown")).join("") +
            "</select>" +
            "<select id=\"calibrationTruthSource\" aria-label=\"Fonte da confirmação\">" +
                sources.map(c => option(c[0], c[1], detail.truthSource || "")).join("") +
            "</select>" +
            "<button class=\"button button-primary\" id=\"saveCalibrationTruth\" type=\"button\">Salvar</button>" +
        "</div><p id=\"calibrationTruthMessage\" class=\"form-feedback\"></p></section>";
}

function renderLowResolutionComparison(events) {
    const event = events.find((item) => item.eventType === "cnh_low_resolution_shadow");
    if (!event) return "";
    const payload = objectValue(event.payload);
    if (!payload || !Array.isArray(payload.variants)) return "";

    const cosine = (pair) => {
        const raw = pair?.robustCosine;
        return raw === null || raw === undefined || !Number.isFinite(Number(raw))
            ? "—"
            : Number(raw).toFixed(4);
    };
    const labels = {
        production_cubic: "Bicúbica (atual)",
        nearest: "Nearest Neighbor",
        linear: "Bilinear",
        lanczos4: "Lanczos4",
    };

    return `
        <section class="detail-section">
            <h3>Benchmark VIO — baixa resolução</h3>
            <p>Diagnóstico automático, não utilizado para aprovar ou reprovar.
                Scores cosseno brutos, sem interpretação como probabilidade de identidade.</p>
            <div style="overflow-x:auto">
                <table class="benchmark-table">
                    <thead><tr>
                        <th>Interpolação</th><th>CNH × VIO</th>
                        <th>VIO × Face</th><th>CNH × Face</th><th>Escala</th><th>Estado</th>
                    </tr></thead>
                    <tbody>
                        ${payload.variants.map((item) => `
                            <tr>
                                <td>${escapeHtml(labels[item.method] || item.method)}</td>
                                <td>${cosine(item.pdfVio)}</td>
                                <td>${cosine(item.vioLive)}</td>
                                <td>${cosine(payload.pdf_live)}</td>
                                <td>${Number.isFinite(Number(item.scaleX)) ? Number(item.scaleX).toFixed(2) + "×" : "—"}</td>
                                <td>${escapeHtml(item.status || "indisponível")}</td>
                            </tr>
                        `).join("")}
                    </tbody>
                </table>
            </div>
            <p>A foto VIO é de baixa resolução. Valores maiores não são necessariamente
                melhores sem medir também o comportamento com impostores.</p>
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

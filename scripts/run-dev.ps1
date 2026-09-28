$ErrorActionPreference = "Stop"
$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $Root

python models/download_models.py

if (-not (Test-Path ".venv")) {
    python -m venv .venv
}

$Python = Join-Path $Root ".venv\Scripts\python.exe"
& $Python -m pip install -r services\engine\requirements.txt

$DevDirectory = Join-Path $Root ".dev"
New-Item -ItemType Directory -Force -Path $DevDirectory | Out-Null
$SessionSecretFile = Join-Path $DevDirectory "session-secret"
$TemplateKeyFile = Join-Path $DevDirectory "template-key"
$IdentityIssuerKeyFile = Join-Path $DevDirectory "identity-issuer-key"

if (-not (Test-Path $SessionSecretFile)) {
    & python -c "import secrets; print(secrets.token_urlsafe(48))" | Set-Content -NoNewline $SessionSecretFile
}
if (-not (Test-Path $TemplateKeyFile)) {
    & python -c "import os,base64; print(base64.b64encode(os.urandom(32)).decode())" | Set-Content -NoNewline $TemplateKeyFile
}
if (-not (Test-Path $IdentityIssuerKeyFile)) {
    & python -c "import secrets; print(secrets.token_urlsafe(48))" | Set-Content -NoNewline $IdentityIssuerKeyFile
}

if (-not $env:FACEPROOF_SESSION_SECRET) {
    $env:FACEPROOF_SESSION_SECRET = Get-Content -Raw $SessionSecretFile
}
if (-not $env:FACEPROOF_TEMPLATE_KEY) {
    $env:FACEPROOF_TEMPLATE_KEY = Get-Content -Raw $TemplateKeyFile
}
if (-not $env:FACEPROOF_IDENTITY_ISSUER_KEY) {
    $env:FACEPROOF_IDENTITY_ISSUER_KEY = Get-Content -Raw $IdentityIssuerKeyFile
}

$env:FACEPROOF_TEMPLATE_DIR = Join-Path $Root "data\templates"
$env:FACEPROOF_IDENTITY_DIR = Join-Path $Root "data\identity-checks"
$env:FACEPROOF_IDENTITY_VERIFY_URL = "http://localhost:5173/verify.html"
$env:FACEPROOF_YUNET_MODEL = Join-Path $Root "models\yunet\face_detection_yunet_2023mar.onnx"
$env:FACEPROOF_SFACE_MODEL = Join-Path $Root "models\sface\face_recognition_sface_2021dec.onnx"
$env:FACEPROOF_MINIFASNET_MODEL = Join-Path $Root "models\minifasnet\MiniFASNetV2.onnx"
$env:FACEPROOF_ALLOW_REVIEW_ENROLLMENT = "true"
$env:FACEPROOF_DEBUG = "true"

$LocalBPG = Join-Path $Root "tools\bpg\bpgdec.exe"
if ((-not $env:FACEPROOF_BPGDEC_PATH) -and (Test-Path $LocalBPG)) {
    $env:FACEPROOF_BPGDEC_PATH = $LocalBPG
}

$MissingRequired = @()
foreach ($Tool in @("pdfinfo", "pdftoppm")) {
    if (-not (Get-Command $Tool -ErrorAction SilentlyContinue)) {
        $MissingRequired += $Tool
    }
}
if ($MissingRequired.Count -gt 0) {
    Write-Warning ("IDENTITY CHECK indisponivel ate instalar/configurar: " + ($MissingRequired -join ", "))
}
if (-not (Get-Command "pdfsig" -ErrorAction SilentlyContinue)) {
    Write-Host "INFO: pdfsig nao encontrado. O FaceProof usara a verificacao criptografica nativa em Go."
}

foreach ($Tool in @("pdfimages", "pdftotext")) {
    if (-not (Get-Command $Tool -ErrorAction SilentlyContinue)) {
        Write-Warning "$Tool nao encontrado (analise forense complementar ficara limitada)."
    }
}
if (-not (Get-Command "openssl" -ErrorAction SilentlyContinue)) {
    Write-Host "INFO: openssl nao encontrado. A verificacao nativa ainda confere o periodo de validade do certificado; openssl fica como evidencia complementar."
}
if ((-not $env:FACEPROOF_BPGDEC_PATH) -and (-not (Get-Command "bpgdec" -ErrorAction SilentlyContinue))) {
    Write-Host "INFO: bpgdec nao encontrado. Isso nao bloqueia mais a CNH; a foto assinada do PDF sera usada."
}

$env:FACEPROOF_WEB_DIR = Join-Path $Root "apps\web-sdk"

$Processes = @()

Write-Host ""
Write-Host "Starting FaceProof services in this window..." -ForegroundColor Cyan

$Processes += Start-Process powershell -NoNewWindow -PassThru -ArgumentList @(
    "-NoProfile",
    "-Command",
    "Set-Location '$Root\services\engine'; Write-Host '[ENGINE] starting...' -ForegroundColor DarkCyan; & '$Python' engine_server.py"
)

$Processes += Start-Process powershell -NoNewWindow -PassThru -ArgumentList @(
    "-NoProfile",
    "-Command",
    "Set-Location '$Root\services\api'; Write-Host '[API] starting...' -ForegroundColor DarkGreen; go run ./cmd/server"
)

$Processes += Start-Process powershell -NoNewWindow -PassThru -ArgumentList @(
    "-NoProfile",
    "-Command",
    "Set-Location '$Root\services\api'; Write-Host '[WEB] starting...' -ForegroundColor DarkYellow; go run ./cmd/devgateway"
)

Write-Host ""
Write-Host "FaceProof demo: http://localhost:5173"
Write-Host "Identity verification: http://localhost:5173/verify.html"
Write-Host "Identity issuer key: $IdentityIssuerKeyFile"
Write-Host "Development mode: review enrollments are stored as provisional templates."
Write-Host ""
Write-Host "Press Ctrl+C to stop all FaceProof services." -ForegroundColor Yellow
Write-Host ""

try {
    Wait-Process -InputObject $Processes
}
finally {
    Write-Host ""
    Write-Host "Stopping FaceProof services..." -ForegroundColor Yellow

    foreach ($Process in $Processes) {
        if ($Process -and -not $Process.HasExited) {
            & taskkill.exe /PID $Process.Id /T /F 2>$null | Out-Null
        }
    }
}

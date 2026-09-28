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

$LocalBPG = Join-Path $Root "tools\bpg\bpgdec.exe"
if ((-not $env:FACEPROOF_BPGDEC_PATH) -and (Test-Path $LocalBPG)) {
    $env:FACEPROOF_BPGDEC_PATH = $LocalBPG
}

$MissingRequired = @()
foreach ($Tool in @("pdfsig", "pdfinfo", "pdftoppm")) {
    if (-not (Get-Command $Tool -ErrorAction SilentlyContinue)) {
        $MissingRequired += $Tool
    }
}
if ($MissingRequired.Count -gt 0) {
    Write-Warning ("IDENTITY CHECK indisponivel ate instalar/configurar: " + ($MissingRequired -join ", "))
}

foreach ($Tool in @("pdfimages", "pdftotext")) {
    if (-not (Get-Command $Tool -ErrorAction SilentlyContinue)) {
        Write-Warning "$Tool nao encontrado (analise forense complementar ficara limitada)."
    }
}
if (-not (Get-Command "openssl" -ErrorAction SilentlyContinue)) {
    Write-Warning "openssl nao encontrado. CNHs com certificado atualmente expirado nao poderao comprovar que o certificado era valido na data da assinatura."
}
if ((-not $env:FACEPROOF_BPGDEC_PATH) -and (-not (Get-Command "bpgdec" -ErrorAction SilentlyContinue))) {
    Write-Host "INFO: bpgdec nao encontrado. Isso nao bloqueia mais a CNH; a foto assinada do PDF sera usada."
}

Start-Process powershell -ArgumentList "-NoExit", "-Command", "Set-Location '$Root\services\engine'; & '$Python' engine_server.py"
Start-Process powershell -ArgumentList "-NoExit", "-Command", "Set-Location '$Root\services\api'; go run ./cmd/server"
Start-Process powershell -ArgumentList "-NoExit", "-Command", "Set-Location '$Root\apps\web-sdk'; python -m http.server 5173"

Write-Host "FaceProof demo: http://localhost:5173"
Write-Host "Identity verification: http://localhost:5173/verify.html"
Write-Host "Identity issuer key: $IdentityIssuerKeyFile"
Write-Host "Development mode: review enrollments are stored as provisional templates."

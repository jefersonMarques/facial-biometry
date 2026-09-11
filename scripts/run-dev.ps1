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

if (-not (Test-Path $SessionSecretFile)) {
    & python -c "import secrets; print(secrets.token_urlsafe(48))" | Set-Content -NoNewline $SessionSecretFile
}
if (-not (Test-Path $TemplateKeyFile)) {
    & python -c "import os,base64; print(base64.b64encode(os.urandom(32)).decode())" | Set-Content -NoNewline $TemplateKeyFile
}

if (-not $env:FACEPROOF_SESSION_SECRET) {
    $env:FACEPROOF_SESSION_SECRET = Get-Content -Raw $SessionSecretFile
}
if (-not $env:FACEPROOF_TEMPLATE_KEY) {
    $env:FACEPROOF_TEMPLATE_KEY = Get-Content -Raw $TemplateKeyFile
}

$env:FACEPROOF_TEMPLATE_DIR = Join-Path $Root "data\templates"
$env:FACEPROOF_YUNET_MODEL = Join-Path $Root "models\yunet\face_detection_yunet_2023mar.onnx"
$env:FACEPROOF_SFACE_MODEL = Join-Path $Root "models\sface\face_recognition_sface_2021dec.onnx"
$env:FACEPROOF_MINIFASNET_MODEL = Join-Path $Root "models\minifasnet\MiniFASNetV2.onnx"

Start-Process powershell -ArgumentList "-NoExit", "-Command", "Set-Location '$Root\services\engine'; & '$Python' engine_server.py"
Start-Process powershell -ArgumentList "-NoExit", "-Command", "Set-Location '$Root\services\api'; go run ./cmd/server"
Start-Process powershell -ArgumentList "-NoExit", "-Command", "Set-Location '$Root\apps\web-sdk'; python -m http.server 5173"

Write-Host "FaceProof demo: http://localhost:5173"

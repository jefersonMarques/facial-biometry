param(
    [string]$BuildDir = "",
    [string]$PublishDir = ""
)

$ErrorActionPreference = "Stop"
$Root = Resolve-Path (Join-Path $PSScriptRoot "..")

if (-not (Get-Command emcmake -ErrorAction SilentlyContinue)) {
    throw "Emscripten nao encontrado. Ative o emsdk antes de executar este script."
}

if (-not $BuildDir) {
    $BuildDir = Join-Path $Root "build\liveness-wasm"
}
if (-not $PublishDir) {
    $PublishDir = Join-Path $Root "apps\web-sdk\wasm\liveness-core"
}

$SourceDir = Join-Path $Root "faceproof-liveness-core"

emcmake cmake -S $SourceDir -B $BuildDir -DFACEPROOF_BUILD_TESTS=OFF -DCMAKE_BUILD_TYPE=Release
cmake --build $BuildDir --config Release

$JsPath = Join-Path $BuildDir "faceproof-liveness-core.js"
$WasmPath = Join-Path $BuildDir "faceproof-liveness-core.wasm"

if (-not (Test-Path $JsPath) -or -not (Test-Path $WasmPath)) {
    throw "Build concluido sem os artefatos WASM esperados."
}

New-Item -ItemType Directory -Force -Path $PublishDir | Out-Null
Copy-Item -Force $JsPath (Join-Path $PublishDir "faceproof-liveness-core.js")
Copy-Item -Force $WasmPath (Join-Path $PublishDir "faceproof-liveness-core.wasm")

Write-Host "[ok] build: $JsPath"
Write-Host "[ok] build: $WasmPath"
Write-Host "[ok] publish: $PublishDir"

param([string]$BuildDir = "")

$ErrorActionPreference = "Stop"
$Root = Resolve-Path (Join-Path $PSScriptRoot "..")

if (-not (Get-Command emcmake -ErrorAction SilentlyContinue)) {
    throw "Emscripten nao encontrado. Ative o emsdk antes de executar este script."
}

if (-not $BuildDir) {
    $BuildDir = Join-Path $Root "build\liveness-wasm"
}

$SourceDir = Join-Path $Root "faceproof-liveness-core"

emcmake cmake -S $SourceDir -B $BuildDir -DFACEPROOF_BUILD_TESTS=OFF -DCMAKE_BUILD_TYPE=Release
cmake --build $BuildDir --config Release

$JsPath = Join-Path $BuildDir "faceproof-liveness-core.js"
$WasmPath = Join-Path $BuildDir "faceproof-liveness-core.wasm"

if (-not (Test-Path $JsPath) -or -not (Test-Path $WasmPath)) {
    throw "Build concluido sem os artefatos WASM esperados."
}

Write-Host "[ok] $JsPath"
Write-Host "[ok] $WasmPath"

param(
    [string]$BuildDir = "",
    [string]$PublishDir = ""
)

$ErrorActionPreference = "Stop"
$Root = Resolve-Path (Join-Path $PSScriptRoot "..")

if (-not (Get-Command emcmake -ErrorAction SilentlyContinue)) {
    throw "Emscripten nao encontrado. Ative o emsdk antes de executar este script."
}

$CMakeCommand = Get-Command cmake -ErrorAction SilentlyContinue
if (-not $CMakeCommand) {
    $CMakeCandidates = @()
    if ($env:ProgramFiles) {
        $CMakeCandidates += Join-Path $env:ProgramFiles "CMake\bin\cmake.exe"
    }
    if (${env:ProgramFiles(x86)}) {
        $CMakeCandidates += Join-Path ${env:ProgramFiles(x86)} "CMake\bin\cmake.exe"
    }

    $CMakeExe = $CMakeCandidates | Where-Object { Test-Path $_ } | Select-Object -First 1
    if (-not $CMakeExe) {
        throw "CMake nao encontrado. Instale o CMake ou adicione C:\Program Files\CMake\bin ao PATH."
    }

    $CMakeBin = Split-Path -Parent $CMakeExe
    $env:Path = "$CMakeBin;$env:Path"
    Write-Host "[info] CMake localizado em $CMakeExe"
}

$NinjaCommand = Get-Command ninja -ErrorAction SilentlyContinue
if (-not $NinjaCommand) {
    throw "Ninja nao encontrado. Instale com: winget install --id Ninja-build.Ninja -e"
}

if (-not $BuildDir) {
    $BuildDir = Join-Path $Root "build\liveness-wasm"
}
if (-not $PublishDir) {
    $PublishDir = Join-Path $Root "apps\web-sdk\wasm\liveness-core"
}

$SourceDir = Join-Path $Root "faceproof-liveness-core"

if (Test-Path $BuildDir) {
    Remove-Item -Recurse -Force $BuildDir
}

emcmake cmake -G Ninja -S $SourceDir -B $BuildDir -DFACEPROOF_BUILD_TESTS=OFF -DCMAKE_BUILD_TYPE=Release
cmake --build $BuildDir

$JsPath = Join-Path $BuildDir "faceproof-liveness-core.js"
$WasmPath = Join-Path $BuildDir "faceproof-liveness-core.wasm"

if (-not (Test-Path $JsPath) -or -not (Test-Path $WasmPath)) {
    throw "Build concluido sem os artefatos WASM esperados."
}

New-Item -ItemType Directory -Force -Path $PublishDir | Out-Null
Copy-Item -Force $JsPath (Join-Path $PublishDir "faceproof-liveness-core.js")
Copy-Item -Force $WasmPath (Join-Path $PublishDir "faceproof-liveness-core.wasm")

$ManifestPath = Join-Path $BuildDir "faceproof-liveness-core.manifest.json"
$JsHash = (Get-FileHash -Algorithm SHA256 $JsPath).Hash.ToLowerInvariant()
$WasmHash = (Get-FileHash -Algorithm SHA256 $WasmPath).Hash.ToLowerInvariant()
$Manifest = [ordered]@{
    schemaVersion = 1
    version = "0.1.0"
    files = @(
        [ordered]@{
            path = "faceproof-liveness-core.js"
            bytes = (Get-Item $JsPath).Length
            sha256 = $JsHash
        },
        [ordered]@{
            path = "faceproof-liveness-core.wasm"
            bytes = (Get-Item $WasmPath).Length
            sha256 = $WasmHash
        }
    )
}
$Manifest | ConvertTo-Json -Depth 5 | Set-Content -Encoding UTF8 $ManifestPath
Copy-Item -Force $ManifestPath (Join-Path $PublishDir "faceproof-liveness-core.manifest.json")

Write-Host "[ok] build: $JsPath"
Write-Host "[ok] build: $WasmPath"
Write-Host "[ok] manifest: $ManifestPath"
Write-Host "[ok] publish: $PublishDir"

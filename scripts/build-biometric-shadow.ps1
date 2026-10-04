param(
    [switch]$Force
)

$ErrorActionPreference = "Stop"
$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $Root

$OpenCVVersion = "4.14.0"
$OpenCVRoot = Join-Path $Root "build\opencv-$OpenCVVersion-windows"
$OpenCVSource = Join-Path $OpenCVRoot "src"
$OpenCVBuild = Join-Path $OpenCVRoot "build"
$OpenCVInstall = Join-Path $OpenCVRoot "install"
$ShadowBuild = Join-Path $Root "build\biometric-shadow"
$DevDirectory = Join-Path $Root ".dev"
$ManifestPath = Join-Path $DevDirectory "native-shadow.json"

function Find-OpenCVConfig {
    if (-not (Test-Path $OpenCVInstall)) {
        return $null
    }

    $StaticConfig = Join-Path $OpenCVInstall "x64\vc17\staticlib\OpenCVConfig.cmake"
    if (Test-Path $StaticConfig) {
        return Get-Item $StaticConfig
    }

    return Get-ChildItem -Path $OpenCVInstall -Filter "OpenCVConfig.cmake" -Recurse -File -ErrorAction SilentlyContinue |
        Where-Object { $_.DirectoryName -match "[\\/]staticlib$" } |
        Select-Object -First 1
}

function Find-BuiltExecutable([string]$Name) {
    foreach ($Candidate in @(
        (Join-Path $ShadowBuild "$Name.exe"),
        (Join-Path $ShadowBuild "Release\$Name.exe")
    )) {
        if (Test-Path $Candidate) {
            return (Resolve-Path $Candidate).Path
        }
    }

    $Found = Get-ChildItem -Path $ShadowBuild -Filter "$Name.exe" -Recurse -File -ErrorAction SilentlyContinue |
        Select-Object -First 1
    if ($Found) {
        return $Found.FullName
    }
    return $null
}

if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
    throw "git nao encontrado."
}
if (-not (Get-Command cmake -ErrorAction SilentlyContinue)) {
    throw "cmake nao encontrado."
}

$OpenCVConfig = Find-OpenCVConfig

$StaticLibraryDirectory = Join-Path $OpenCVInstall "x64\vc17\staticlib"
$OpenCVModulesPath = Join-Path $StaticLibraryDirectory "OpenCVModules.cmake"
$MissingADELibrary = -not (Test-Path (Join-Path $StaticLibraryDirectory "ade.lib"))
$ExportsADETarget = (
    (Test-Path $OpenCVModulesPath) -and
    ((Get-Content -Raw $OpenCVModulesPath) -match "(?m)\bade\.lib\b")
)

if (-not $Force -and $OpenCVConfig -and $MissingADELibrary -and $ExportsADETarget) {
    Write-Host "Repairing static OpenCV package: disabling unused ADE/G-API export..." -ForegroundColor Yellow

    $OpenCVRepairArgs = @(
        "-S", $OpenCVSource,
        "-B", $OpenCVBuild,
        "-DBUILD_opencv_gapi=OFF",
        "-DWITH_ADE=OFF"
    )
    & cmake @OpenCVRepairArgs
    if ($LASTEXITCODE -ne 0) {
        throw "Falha ao reconfigurar OpenCV sem ADE."
    }

    & cmake --build $OpenCVBuild --config Release --target INSTALL --parallel 2
    if ($LASTEXITCODE -ne 0) {
        throw "Falha ao reinstalar OpenCV sem ADE."
    }

    $OpenCVConfig = Find-OpenCVConfig
}

if ($Force -or -not $OpenCVConfig) {
    if ($Force) {
        Remove-Item -Recurse -Force $OpenCVBuild -ErrorAction SilentlyContinue
        Remove-Item -Recurse -Force $OpenCVInstall -ErrorAction SilentlyContinue
    }

    if (-not (Test-Path (Join-Path $OpenCVSource ".git"))) {
        Remove-Item -Recurse -Force $OpenCVSource -ErrorAction SilentlyContinue
        New-Item -ItemType Directory -Force -Path $OpenCVRoot | Out-Null
        Write-Host "Cloning pinned OpenCV $OpenCVVersion source..." -ForegroundColor DarkCyan
        git clone --depth 1 --branch $OpenCVVersion https://github.com/opencv/opencv.git $OpenCVSource
        if ($LASTEXITCODE -ne 0) {
            throw "Falha ao obter OpenCV $OpenCVVersion."
        }
    }

    Write-Host "Configuring minimal static OpenCV $OpenCVVersion..." -ForegroundColor DarkCyan
    $OpenCVConfigureArgs = @(
        "-S", $OpenCVSource,
        "-B", $OpenCVBuild,
        "-G", "Visual Studio 17 2022",
        "-A", "x64",
        "-DCMAKE_INSTALL_PREFIX=$OpenCVInstall",
        "-DBUILD_LIST=core,imgproc,imgcodecs,dnn,objdetect",
        "-DBUILD_SHARED_LIBS=OFF",
        "-DBUILD_TESTS=OFF",
        "-DBUILD_PERF_TESTS=OFF",
        "-DBUILD_EXAMPLES=OFF",
        "-DBUILD_opencv_apps=OFF",
        "-DBUILD_opencv_python3=OFF",
        "-DBUILD_JAVA=OFF",
        "-DBUILD_opencv_java=OFF",
        "-DBUILD_opencv_gapi=OFF",
        "-DWITH_ADE=OFF",
        "-DWITH_FFMPEG=OFF",
        "-DWITH_GSTREAMER=OFF",
        "-DWITH_OPENCL=OFF",
        "-DWITH_IPP=OFF",
        "-DWITH_TIFF=OFF",
        "-DWITH_OPENEXR=OFF",
        "-DWITH_WEBP=OFF"
    )
    & cmake @OpenCVConfigureArgs
    if ($LASTEXITCODE -ne 0) {
        throw "Falha ao configurar OpenCV."
    }

    Write-Host "Building minimal OpenCV $OpenCVVersion..." -ForegroundColor DarkCyan
    & cmake --build $OpenCVBuild --config Release --target INSTALL --parallel 2
    if ($LASTEXITCODE -ne 0) {
        throw "Falha ao compilar OpenCV."
    }

    $OpenCVConfig = Find-OpenCVConfig
}

if (-not $OpenCVConfig) {
    throw "OpenCVConfig.cmake nao encontrado apos o build."
}

python models\download_models.py
if ($LASTEXITCODE -ne 0) {
    throw "Falha ao preparar os modelos biometricos."
}

Write-Host "Configuring FaceProof native biometric shadow..." -ForegroundColor DarkCyan
Remove-Item -Recurse -Force $ShadowBuild -ErrorAction SilentlyContinue
$ShadowConfigureArgs = @(
    "-S", (Join-Path $Root "faceproof-biometric-core"),
    "-B", $ShadowBuild,
    "-G", "Visual Studio 17 2022",
    "-A", "x64",
    "-DFACEPROOF_BIOMETRIC_WITH_OPENCV=ON",
    "-DFACEPROOF_BIOMETRIC_WITH_ONNXRUNTIME=OFF",
    "-DOpenCV_STATIC=ON",
    "-DOpenCV_DIR=$($OpenCVConfig.DirectoryName)"
)
& cmake @ShadowConfigureArgs
if ($LASTEXITCODE -ne 0) {
    throw "Falha ao configurar FaceProof biometric shadow."
}

& cmake --build $ShadowBuild --config Release --target faceproof_biometric_vision_cli faceproof_biometric_reference_cli --parallel 2
if ($LASTEXITCODE -ne 0) {
    throw "Falha ao compilar FaceProof biometric shadow."
}

$VisionCli = Find-BuiltExecutable "faceproof_biometric_vision_cli"
$ReferenceCli = Find-BuiltExecutable "faceproof_biometric_reference_cli"
if (-not $VisionCli -or -not $ReferenceCli) {
    throw "Executaveis do native shadow nao foram encontrados."
}

New-Item -ItemType Directory -Force -Path $DevDirectory | Out-Null
$Manifest = [ordered]@{
    schemaVersion = 1
    mode = "vision-only"
    openCvVersion = $OpenCVVersion
    visionCli = $VisionCli
    referenceCli = $ReferenceCli
    padCli = $null
}
$Manifest | ConvertTo-Json -Depth 5 | Set-Content -Encoding UTF8 $ManifestPath

Write-Host ""
Write-Host "[ok] FaceProof native shadow pronto." -ForegroundColor Green
Write-Host "[ok] OpenCV: $($OpenCVConfig.DirectoryName)"
Write-Host "[ok] Vision CLI: $VisionCli"
Write-Host "[ok] Reference CLI: $ReferenceCli"
Write-Host "[ok] Manifest: $ManifestPath"
Write-Host ""
Write-Host "Use: .\scripts\run-dev.ps1 -NativeShadow" -ForegroundColor Cyan

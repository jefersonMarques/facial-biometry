param(
    [Parameter(Mandatory = $true)]
    [string]$CPF,

    [Parameter(Mandatory = $true)]
    [ValidatePattern("^\d{4}-\d{2}-\d{2}$")]
    [string]$MinimumDocumentDate,

    [int]$ExpiresInMinutes = 60,

    [string]$ApiUrl = "http://localhost:8080"
)

$ErrorActionPreference = "Stop"
$Root = Resolve-Path (Join-Path $PSScriptRoot "..")

$IssuerKey = $env:FACEPROOF_IDENTITY_ISSUER_KEY
if (-not $IssuerKey) {
    $KeyFile = Join-Path $Root ".dev\identity-issuer-key"
    if (-not (Test-Path $KeyFile)) {
        throw "Identity issuer key not found. Run scripts\run-dev.ps1 once or set FACEPROOF_IDENTITY_ISSUER_KEY."
    }
    $IssuerKey = Get-Content -Raw $KeyFile
}

$Body = @{
    cpf = $CPF
    minimumDocumentDate = $MinimumDocumentDate
    expiresInMinutes = $ExpiresInMinutes
} | ConvertTo-Json

Invoke-RestMethod -Method Post -Uri "$ApiUrl/v1/identity/checks" -Headers @{
    Authorization = "Bearer $IssuerKey"
} -ContentType "application/json" -Body $Body | ConvertTo-Json -Depth 6

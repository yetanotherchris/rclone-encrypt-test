param(
    [Parameter(Mandatory = $true)]
    [string]$Version
)

$exePath = Join-Path $PSScriptRoot "artifacts" "rclone-encrypt-test-windows-amd64.exe"

if (-not (Test-Path $exePath)) {
    throw "Unable to locate rclone-encrypt-test-windows-amd64.exe at $exePath"
}

$hash = (Get-FileHash -Path $exePath -Algorithm SHA256).Hash.ToLower()

Write-Host "Hash: $hash"

$url = "https://github.com/yetanotherchris/rclone-encrypt-test/releases/download/v$Version/rclone-encrypt-test-windows-amd64.exe"

$manifestPath = Join-Path $PSScriptRoot "rclone-encrypt-test.json"

$manifest = Get-Content -Path $manifestPath -Raw | ConvertFrom-Json

$manifest.version = $Version

$manifest.architecture."64bit".url = $url

$manifest.architecture."64bit".hash = $hash

$manifest | ConvertTo-Json -Depth 10 | Set-Content -Path $manifestPath -NoNewline

Write-Host "Updated rclone-encrypt-test.json to v$Version"

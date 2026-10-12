#requires -Version 5.1
# Run goose migrations. Usage:
#   scripts\migrate.ps1 up
#   scripts\migrate.ps1 status
#   scripts\migrate.ps1 down
param([ValidateSet('up', 'down', 'status')][string]$Action = 'up')
. "$PSScriptRoot\env.ps1"

Push-Location (Join-Path $RepoRoot 'backend')
try {
    & go run ./cmd/migrate $Action
    if ($LASTEXITCODE -ne 0) { throw "migrate $Action failed ($LASTEXITCODE)" }
} finally { Pop-Location }
Write-Host "migrate $Action done" -ForegroundColor Green

#requires -Version 5.1
# Start the backend API server (foreground). Usage: scripts\run.ps1
. "$PSScriptRoot\env.ps1"

Push-Location (Join-Path $RepoRoot 'backend')
try {
    Write-Host "serving on $env:ADDR  (Ctrl+C to stop)" -ForegroundColor Green
    & go run ./cmd/server
    if ($LASTEXITCODE -ne 0) { throw "server exited with $LASTEXITCODE" }
} finally { Pop-Location }

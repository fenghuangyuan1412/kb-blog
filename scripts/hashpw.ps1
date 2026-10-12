#requires -Version 5.1
# Generate the bcrypt hash for ADMIN_PASSWORD_HASH in .env.
# Usage: scripts\hashpw.ps1 -Password 'your-password'
param([Parameter(Mandatory = $true)][string]$Password)
. "$PSScriptRoot\env.ps1"

Push-Location (Join-Path $RepoRoot 'backend')
try {
    $hash = (& go run ./cmd/kbhash $Password) -join ''
} finally { Pop-Location }
if (-not $hash) { Write-Host 'hash generation failed' -ForegroundColor Red; exit 1 }

Write-Host ''
Write-Host 'copy this line into .env (never store the plaintext password):' -ForegroundColor Yellow
Write-Host "ADMIN_PASSWORD_HASH=$hash"

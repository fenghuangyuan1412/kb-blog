#requires -Version 5.1
# Load demo content into the dev database (TRUNCATE first, so it is idempotent).
# Usage:
#   scripts\seed.ps1            # only allowed against the "kb" dev database
#   scripts\seed.ps1 -Force     # target another database, knowingly wiping documents
param([switch]$Force)
. "$PSScriptRoot\env.ps1"

Push-Location (Join-Path $RepoRoot 'backend')
try {
    if ($Force) {
        & go run ./scripts -force
    } else {
        & go run ./scripts
    }
    if ($LASTEXITCODE -ne 0) { throw "seed failed with $LASTEXITCODE" }
} finally { Pop-Location }

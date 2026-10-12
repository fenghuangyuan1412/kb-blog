#requires -Version 5.1
# Run backend unit tests. Usage: scripts\test.ps1
# -p 1 is required: repo and service tests share the "kb" dev database and both
# TRUNCATE the documents table, so packages must not run concurrently.
. "$PSScriptRoot\env.ps1"

Push-Location (Join-Path $RepoRoot 'backend')
try {
    & go test -p 1 -count=1 ./...
    if ($LASTEXITCODE -ne 0) { throw "tests failed with $LASTEXITCODE" }
} finally { Pop-Location }
Write-Host "tests passed" -ForegroundColor Green

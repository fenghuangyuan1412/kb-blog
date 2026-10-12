#requires -Version 5.1
# Stop the portable PostgreSQL server.
# Usage: powershell -ExecutionPolicy Bypass -File scripts\pg-stop.ps1
. "$PSScriptRoot\env.ps1"

& "$(Join-Path $PgBin 'pg_ctl.exe')" -D $env:PGDATA -m fast stop
if ($LASTEXITCODE -ne 0) {
    Write-Host "pg_ctl stop returned $LASTEXITCODE (already stopped?)" -ForegroundColor Yellow
    exit 0
}
Write-Host 'PostgreSQL stopped' -ForegroundColor Green

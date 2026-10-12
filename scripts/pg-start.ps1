#requires -Version 5.1
# Start the portable PostgreSQL server (no Windows service, no Docker).
# Usage: powershell -ExecutionPolicy Bypass -File scripts\pg-start.ps1
. "$PSScriptRoot\env.ps1"

$logFile = Join-Path $env:PGDATA 'server.log'
& "$(Join-Path $PgBin 'pg_ctl.exe')" -D $env:PGDATA -l $logFile -w -t 30 `
    -o "-p $env:PGPORT -c listen_addresses=127.0.0.1" start
if ($LASTEXITCODE -ne 0) {
    Write-Host "pg_ctl start failed ($LASTEXITCODE), last log lines:" -ForegroundColor Red
    Get-Content -LiteralPath $logFile -Tail 20
    exit $LASTEXITCODE
}

& "$(Join-Path $PgBin 'psql.exe')" -h 127.0.0.1 -p $env:PGPORT -U $env:PGUSER -d postgres `
    -tAc "select 'server up: ' || version()"
if ($LASTEXITCODE -ne 0) { Write-Host 'connection check failed' -ForegroundColor Red; exit 1 }
Write-Host "PostgreSQL listening on 127.0.0.1:$env:PGPORT" -ForegroundColor Green

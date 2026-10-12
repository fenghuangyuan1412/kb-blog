#requires -Version 5.1
# One-time: create the portable PostgreSQL data directory (initdb).
# Usage: powershell -ExecutionPolicy Bypass -File scripts\pg-init.ps1
. "$PSScriptRoot\env.ps1"

$dataDir = $env:PGDATA
if (-not $dataDir) { Write-Host 'PGDATA is empty in .env' -ForegroundColor Red; exit 1 }

if (Test-Path (Join-Path $dataDir 'PG_VERSION')) {
    Write-Host "data dir already initialised: $dataDir" -ForegroundColor Yellow
    exit 0
}

New-Item -ItemType Directory -Force -Path $dataDir | Out-Null

# password comes from .env, written to a throwaway file so it never lands in shell history
$pwFile = Join-Path $env:TEMP 'kb_pg_pw.txt'
Set-Content -LiteralPath $pwFile -Value $env:PGPASSWORD -Encoding ASCII -NoNewline
try {
    # quote whole args so the space inside the locale name survives
    & "$PgBin\initdb.exe" -D $dataDir -U $env:PGUSER "--pwfile=$pwFile" -E UTF8 `
        "--locale=English_United States.1252" -A scram-sha-256 --no-sync
    if ($LASTEXITCODE -ne 0) { throw "initdb failed with $LASTEXITCODE" }
} finally {
    Remove-Item -LiteralPath $pwFile -Force -ErrorAction SilentlyContinue
}

# listen on loopback only, custom port from .env
$conf = Join-Path $dataDir 'postgresql.conf'
@(
    "port = $env:PGPORT"
    "listen_addresses = '127.0.0.1'"
    "fsync = off"          # local dev only, data loss on crash is acceptable here
    "synchronous_commit = off"
    "full_page_writes = off"
) | Add-Content -LiteralPath $conf -Encoding ASCII

Write-Host "initdb done: $dataDir (port $env:PGPORT)" -ForegroundColor Green

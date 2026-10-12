#requires -Version 5.1
# Shared environment for local development scripts.
# Dot-source it:  . "$PSScriptRoot\env.ps1"

$ErrorActionPreference = 'Stop'

$RepoRoot = Split-Path -Parent $PSScriptRoot
Write-Host "repo root: $RepoRoot"

# --- load .env (never committed) ---
$envFile = Join-Path $RepoRoot '.env'
if (-not (Test-Path $envFile)) {
    Write-Host "missing .env -- copy .env.example to .env first" -ForegroundColor Red
    exit 1
}
Get-Content -LiteralPath $envFile -Encoding UTF8 | ForEach-Object {
    $line = $_.Trim()
    if ($line -eq '' -or $line.StartsWith('#')) { return }
    $i = $line.IndexOf('=')
    if ($i -lt 1) { return }
    $key = $line.Substring(0, $i).Trim()
    $val = $line.Substring($i + 1).Trim()
    [Environment]::SetEnvironmentVariable($key, $val, 'Process')
    Set-Variable -Name $key -Value $val -Scope Global
}

# --- toolchain locations (portable installs, nothing under C:\) ---
$GoRoot = 'E:\programme\go'
$PgRoot = 'E:\programme\pgsql'
$PgBin = Join-Path $PgRoot 'bin'
$env:GOPATH = 'E:\gopath'
$env:GOROOT = $GoRoot
$env:GOPROXY = 'https://goproxy.cn,direct'
$env:GOSUMDB = 'sum.golang.google.cn'
$env:PATH = "$($GoRoot)\bin;$PgBin;$env:PATH"

if (-not (Test-Path (Join-Path $GoRoot 'bin\go.exe'))) { Write-Host "go not found at $GoRoot" -ForegroundColor Red; exit 1 }
if (-not (Test-Path (Join-Path $PgBin 'psql.exe')))   { Write-Host "psql not found at $PgBin" -ForegroundColor Red; exit 1 }

Write-Host "go:  $(Join-Path $GoRoot 'bin\go.exe')"
Write-Host "pg:  $PgBin"
Write-Host "data dir: $env:PGDATA  port: $env:PGPORT  db: $env:PGDATABASE  user: $env:PGUSER"

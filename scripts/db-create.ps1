#requires -Version 5.1
# Create the application database and enable the extensions the schema relies on.
# Idempotent: safe to run again.
. "$PSScriptRoot\env.ps1"

$psql = Join-Path $PgBin 'psql.exe'
$exists = & $psql -h 127.0.0.1 -p $env:PGPORT -U $env:PGUSER -d postgres -tAc `
    "select count(*) from pg_database where datname = '$env:PGDATABASE'"
Write-Host "database present: $exists"
if ([int]$exists -eq 0) {
    & $psql -h 127.0.0.1 -p $env:PGPORT -U $env:PGUSER -d postgres -c "CREATE DATABASE $env:PGDATABASE ENCODING 'UTF8'"
    Write-Host "created database $env:PGDATABASE" -ForegroundColor Green
} else {
    Write-Host "database $env:PGDATABASE already exists" -ForegroundColor Yellow
}

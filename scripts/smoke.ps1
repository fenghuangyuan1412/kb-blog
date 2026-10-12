#requires -Version 5.1
# Day-1 API smoke test: health -> login -> nested notes -> tree -> search -> tags
# -> draft visibility -> update -> cycle/slug/conflict rules -> cascade delete -> logout.
#
# Usage:
#   scripts\smoke.ps1                                  # prompts for the admin password
#   scripts\smoke.ps1 -Password 'xxx'                  # non-interactive
#   scripts\smoke.ps1 -BaseUrl http://localhost:8080 -Password 'xxx'
#
# The Chinese payloads live in scripts/fixtures/smoke.json; this file stays pure ASCII
# because Windows PowerShell reads .ps1 with the ANSI codepage unless there is a BOM.
param(
    [string]$BaseUrl = 'http://localhost:8080',
    [string]$AdminEmail = '',
    [string]$Password = ''
)

. "$PSScriptRoot\env.ps1"
$ErrorActionPreference = 'Stop'

if (-not $AdminEmail) { $AdminEmail = $env:ADMIN_EMAIL }
if (-not $Password) {
    $secure = Read-Host 'admin password' -AsSecureString
    $ptr = [Runtime.InteropServices.Marshal]::SecureStringToGlobalAllocUnicode($secure)
    $Password = [Runtime.InteropServices.Marshal]::PtrToStringUni($ptr)
    [Runtime.InteropServices.Marshal]::FreeHGlobal($ptr)
}
if (-not $AdminEmail -or -not $Password) { throw 'need ADMIN_EMAIL (.env) and a password' }

$fixture = Get-Content -Raw -Encoding UTF8 (Join-Path $PSScriptRoot 'fixtures\smoke.json') | ConvertFrom-Json

$script:pass = 0
$script:fail = 0
$script:log = New-Object System.Collections.Generic.List[string]

function Invoke-Api {
    param([string]$Method, [string]$Path, $Body = $null, $Session = $null)
    $p = @{ Method = $Method; Uri = "$BaseUrl$Path"; UseBasicParsing = $true; TimeoutSec = 20 }
    if ($null -ne $Body) {
        $p.Body = [Text.Encoding]::UTF8.GetBytes(($Body | ConvertTo-Json -Depth 8 -Compress))
        $p.ContentType = 'application/json; charset=utf-8'
    }
    if ($null -ne $Session) { $p.WebSession = $Session }
    try {
        $r = Invoke-WebRequest @p
        return [pscustomobject]@{ Code = [int]$r.StatusCode; Text = $r.Content }
    } catch {
        $resp = $_.Exception.Response
        if ($null -eq $resp) { throw }
        $reader = New-Object IO.StreamReader($resp.GetResponseStream(), [Text.Encoding]::UTF8)
        return [pscustomobject]@{ Code = [int]$resp.StatusCode; Text = $reader.ReadToEnd() }
    }
}

function Check-Status {
    param([string]$Name, [int]$Want, $Resp)
    if ($Resp.Code -eq $Want) {
        $script:pass++
        $line = "[OK ] $Name -> $($Resp.Code)"
    } else {
        $script:fail++
        $snip = Cut $Resp.Text
        $line = "[FAIL] $Name -> got $($Resp.Code), want $Want | $snip"
    }
    Write-Host $line -ForegroundColor $(if ($Resp.Code -eq $Want) { 'Green' } else { 'Red' })
    $script:log.Add($line)
}

function Check-Contains {
    param([string]$Name, $Resp, [string]$Needle)
    if ($Resp.Text -match [regex]::Escape($Needle)) {
        $script:pass++
        $line = "[OK ] $Name contains '$Needle'"
        $color = 'Green'
    } else {
        $script:fail++
        $snip = Cut $Resp.Text
        $line = "[FAIL] $Name missing '$Needle' | $snip"
        $color = 'Red'
    }
    Write-Host $line -ForegroundColor $color
    $script:log.Add($line)
}

function Cut([string]$s) {
    if ([string]::IsNullOrEmpty($s)) { return '' }
    if ($s.Length -le 200) { return $s }
    return $s.Substring(0, 200) + '...'
}

function New-DocBody {
    param($Node, [int64]$ParentId = 0, [string]$Slug = '', [string]$Title = '', [string]$Content = '')
    $b = [ordered]@{
        type       = $Node.type
        slug       = $Node.slug
        title      = $Node.title
        summary    = $Node.summary
        content_md = $Node.content_md
        tags       = $Node.tags
        status     = $Node.status
        sort_order = $Node.sort_order
    }
    if ($Slug) { $b.slug = $Slug }
    if ($Title) { $b.title = $Title }
    if ($Content) { $b.content_md = $Content }
    if ($ParentId -ne 0) { $b.parent_id = $ParentId }
    return $b
}

function Get-IdFromJson([string]$text) {
    return ($text | ConvertFrom-Json).id
}

Write-Host "== smoke against $BaseUrl as $AdminEmail ==" -ForegroundColor Cyan

$owner = New-Object Microsoft.PowerShell.Commands.WebRequestSession
$anon = New-Object Microsoft.PowerShell.Commands.WebRequestSession

# 1) previous crashed run may leave rows behind: clean by slug, ignore 404
foreach ($slug in $fixture.cleanup_slugs) {
    $found = Invoke-Api GET "/api/documents/$slug" $null $owner
    if ($found.Code -eq 200) {
        Invoke-Api DELETE "/api/documents/$(Get-IdFromJson $found.Text)" $null $owner | Out-Null
    }
}

# 2) health
Check-Status 'GET /healthz' 200 (Invoke-Api GET '/healthz' $null $anon)

# 3) auth
$bad = Invoke-Api POST '/api/auth/login' @{ email = $AdminEmail; password = "$Password-wrong" } $anon
Check-Status 'POST /api/auth/login (bad password)' 403 $bad
$login = Invoke-Api POST '/api/auth/login' @{ email = $AdminEmail; password = $Password } $owner
Check-Status 'POST /api/auth/login' 200 $login
Check-Contains 'login response' $login $AdminEmail
Check-Status 'GET /api/auth/me (with cookie)' 200 (Invoke-Api GET '/api/auth/me' $null $owner)
Check-Status 'GET /api/auth/me (anonymous)' 401 (Invoke-Api GET '/api/auth/me' $null $anon)

# 4) create three levels of notes
$root = Invoke-Api POST '/api/documents' (New-DocBody $fixture.root) $owner
Check-Status 'POST note root' 201 $root
$rootId = Get-IdFromJson $root.Text

$child = Invoke-Api POST '/api/documents' (New-DocBody $fixture.child $rootId) $owner
Check-Status 'POST note child (level 2)' 201 $child
$childId = Get-IdFromJson $child.Text

$leaf = Invoke-Api POST '/api/documents' (New-DocBody $fixture.leaf $childId) $owner
Check-Status 'POST note grandchild (level 3)' 201 $leaf
$leafId = Get-IdFromJson $leaf.Text

$draft = Invoke-Api POST '/api/documents' (New-DocBody $fixture.draft $rootId) $owner
Check-Status 'POST draft note' 201 $draft

# 5) rule rejections
$illegal = Invoke-Api POST '/api/documents' (New-DocBody $fixture.illegal_parent $rootId) $owner
Check-Status 'POST algorithm with parent_id' 400 $illegal
$dup = Invoke-Api POST '/api/documents' (New-DocBody $fixture.root) $owner
Check-Status 'POST duplicate slug' 409 $dup
$cycle = Invoke-Api POST "/api/documents/$rootId/move" @{ parent_id = $leafId; sort_order = 0 } $owner
Check-Status 'POST move root under its own grandchild' 400 $cycle
$missing = Invoke-Api POST '/api/documents' (New-DocBody $fixture.leaf '999999') $owner
Check-Status 'POST note with unknown parent' 400 $missing

# 6) read chain
$treeOwner = Invoke-Api GET '/api/notes/tree' $null $owner
Check-Status 'GET /api/notes/tree (owner)' 200 $treeOwner
Check-Contains 'tree has level-3 node' $treeOwner $fixture.leaf.slug
Check-Contains 'tree has draft node for owner' $treeOwner $fixture.draft.slug

$treeAnon = Invoke-Api GET '/api/notes/tree' $null $anon
Check-Status 'GET /api/notes/tree (anonymous)' 200 $treeAnon
Check-Contains 'tree has published node' $treeAnon $fixture.leaf.slug
if ($treeAnon.Text -match [regex]::Escape($fixture.draft.slug)) {
    $script:fail++; Write-Host '[FAIL] anonymous tree leaks the draft' -ForegroundColor Red
    $script:log.Add('[FAIL] anonymous tree leaks the draft')
} else {
    $script:pass++; Write-Host '[OK ] anonymous tree hides the draft' -ForegroundColor Green
    $script:log.Add('[OK ] anonymous tree hides the draft')
}

$detail = Invoke-Api GET "/api/documents/$($fixture.root.slug)" $null $owner
Check-Status 'GET /api/documents/:slug' 200 $detail
Check-Contains 'detail has children_count' $detail '"children_count":2'
Check-Status 'GET /api/documents/:unknown-slug' 404 (Invoke-Api GET '/api/documents/no-such-doc-xyz' $null $anon)
Check-Status 'GET /api/documents?size=2' 200 (Invoke-Api GET '/api/documents?page=1&size=2' $null $owner)
Check-Status 'GET /api/documents?type=note' 200 (Invoke-Api GET '/api/documents?type=note' $null $anon)
Check-Status 'GET /api/tags' 200 (Invoke-Api GET '/api/tags' $null $anon)

$q = [uri]::EscapeDataString($fixture.search_q)
$search = Invoke-Api GET "/api/search?q=$q" $null $anon
Check-Status 'GET /api/search (chinese term)' 200 $search
Check-Contains 'search hits the note' $search $fixture.leaf.slug
Check-Status 'GET /api/search without q' 400 (Invoke-Api GET '/api/search' $null $anon)

# 7) update
$updBody = [ordered]@{
    type       = $fixture.leaf.type
    slug       = $fixture.leaf.slug
    title      = $fixture.update.title
    summary    = $fixture.update.summary
    content_md = $fixture.update.content_md
    tags       = $fixture.update.tags
    status     = 'published'
    sort_order = 2
}
$updated = Invoke-Api PUT "/api/documents/$leafId" $updBody $owner
Check-Status 'PUT /api/documents/:id' 200 $updated
Check-Contains 'update really wrote the title' $updated $fixture.update.title
Check-Status 'PUT with empty title' 400 (Invoke-Api PUT "/api/documents/$leafId" (@{ type = 'note'; slug = $fixture.leaf.slug; title = '' }) $owner)

# 8) legal re-parent
$moved = Invoke-Api POST "/api/documents/$leafId/move" @{ parent_id = $rootId; sort_order = 5 } $owner
Check-Status 'POST move (legal)' 200 $moved

# 9) anonymous write must be rejected
Check-Status 'POST /api/documents (anonymous)' 401 (Invoke-Api POST '/api/documents' (New-DocBody $fixture.root '0' 'smoke-anon') $anon)

# 10) cascade delete: removing the root takes the whole subtree with it
$delRoot = Invoke-Api DELETE "/api/documents/$rootId" $null $owner
Check-Status 'DELETE root note (cascade)' 200 $delRoot
Check-Status 'GET deleted grandchild by slug' 404 (Invoke-Api GET "/api/documents/$($fixture.leaf.slug)" $null $owner)

# 11) logout kills the session
Check-Status 'POST /api/auth/logout' 200 (Invoke-Api POST '/api/auth/logout' @{} $owner)
Check-Status 'GET /api/auth/me after logout' 401 (Invoke-Api GET '/api/auth/me' $null $owner)

Write-Host ''
Write-Host "passed: $script:pass   failed: $script:fail" -ForegroundColor $(if ($script:fail -eq 0) { 'Green' } else { 'Red' })
$out = Join-Path $PSScriptRoot '..\tmp\smoke_result.txt'
$script:log -join "`r`n" | Out-File -FilePath $out -Encoding UTF8
Write-Host "detail saved to $out"
if ($script:fail -gt 0) { exit 1 }
exit 0

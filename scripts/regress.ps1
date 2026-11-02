<#
.SYNOPSIS
Tests a new engine version against an older one with an SPRT tournament in the
local arena.

.DESCRIPTION
Registers both versions (unless an engine with the same image already exists),
starts a tournament on the opening suite with an SPRT, follows it, and prints
the verdict. The exit code is 0 when the candidate passed (H1 accepted), 1 when
it failed (H0 accepted), and 2 when the tournament ended without a decision.

The arena must be running (docker compose up -d) and the opening suite must
exist (Tournaments page > Add opening suite).

.EXAMPLE
./scripts/regress.ps1 -Candidate 1.3.0

Tests zienek/chess-arena-engine:1.3.0 against the newest other registered
version of that image.

.EXAMPLE
./scripts/regress.ps1 -Candidate dev -CandidateImage chess-arena-engine:local -Baseline 1.2.0 -Elo1 50

Tests a locally built image against 1.2.0, looking for a large gain.
#>
param(
    [Parameter(Mandatory = $true)][string]$Candidate,
    [string]$Baseline,
    [string]$Repository = 'zienek/chess-arena-engine',
    [string]$CandidateImage,
    [string]$BaselineImage,
    [string]$EngineName = 'alpha-beta',
    [double]$Elo0 = 0,
    [double]$Elo1 = 20,
    [double]$Alpha = 0.05,
    [double]$Beta = 0.05,
    [int]$MaxGames = 2000,
    [int]$MoveTimeMs = 1000,
    [long]$Nodes = 0,
    [string]$PositionSet = 'Balanced openings (50)',
    [string]$Api = 'http://localhost:8080',
    [string]$Frontend = 'http://localhost:3000',
    [string]$Token = $env:ARENA_API_TOKEN,
    [int]$PollSeconds = 10,
    [switch]$NoWait
)

$ErrorActionPreference = 'Stop'

function Invoke-Arena([string]$Method, [string]$Path, $Body) {
    $headers = @{}
    if ($Token) { $headers['Authorization'] = "Bearer $Token" }
    $arguments = @{ Method = $Method; Uri = "$Api$Path"; Headers = $headers }
    if ($null -ne $Body) {
        $arguments['ContentType'] = 'application/json'
        $arguments['Body'] = ($Body | ConvertTo-Json -Depth 5 -Compress)
    }
    try {
        # Windows PowerShell returns a JSON array as one object; assigning it
        # first makes the return statement emit its elements one by one.
        $result = Invoke-RestMethod @arguments
        return $result
    } catch {
        $detail = $_.ErrorDetails.Message
        if (-not $detail) { $detail = $_.Exception.Message }
        throw "$Method $Path failed: $detail"
    }
}

# Finds the engine running $Image, registering it as $EngineName $Version if needed.
function Resolve-Engine([string]$Version, [string]$Image, $Engines) {
    $existing = @($Engines | Where-Object { $_.image -eq $Image })
    if ($existing.Count -gt 0) { return $existing[0] }
    Write-Host "Registering $EngineName $Version ($Image)"
    return Invoke-Arena 'POST' '/api/engines' @{ name = $EngineName; version = $Version; image = $Image }
}

function Format-Signed([double]$Value, [int]$Digits = 1) {
    $text = $Value.ToString("F$Digits", [Globalization.CultureInfo]::InvariantCulture)
    if ($Value -gt 0) { return "+$text" }
    return $text
}

if (-not $CandidateImage) { $CandidateImage = "${Repository}:$Candidate" }
$engines = @(Invoke-Arena 'GET' '/api/engines' $null)

if (-not $BaselineImage) {
    if (-not $Baseline) {
        # The newest other version of the same image, by version number.
        $versions = $engines |
            Where-Object { $_.image -like "${Repository}:*" -and $_.image -ne $CandidateImage } |
            ForEach-Object {
                $tag = $_.image.Substring($Repository.Length + 1)
                $parsed = $null
                if ([version]::TryParse($tag, [ref]$parsed)) { [pscustomobject]@{ Tag = $tag; Version = $parsed } }
            } |
            Sort-Object Version -Descending
        if (-not $versions) { throw "No baseline found among registered ${Repository} images; pass -Baseline." }
        $Baseline = @($versions)[0].Tag
    }
    $BaselineImage = "${Repository}:$Baseline"
}
if (-not $Baseline) { $Baseline = $BaselineImage }

$candidateEngine = Resolve-Engine $Candidate $CandidateImage $engines
$baselineEngine = Resolve-Engine $Baseline $BaselineImage $engines
if ($candidateEngine.id -eq $baselineEngine.id) { throw 'Candidate and baseline are the same engine.' }

$set = @(Invoke-Arena 'GET' '/api/position-sets' $null | Where-Object { $_.source_name -eq $PositionSet })
if ($set.Count -eq 0) {
    throw "Position set '$PositionSet' not found. Open $Frontend/tournaments and click 'Add opening suite'."
}

$limit = @{ movetime_ms = $MoveTimeMs }
if ($Nodes -gt 0) { $limit = @{ nodes = $Nodes } }
$candidateName = "$($candidateEngine.name) $($candidateEngine.version)"
$baselineName = "$($baselineEngine.name) $($baselineEngine.version)"
$tournament = Invoke-Arena 'POST' '/api/tournaments' @{
    name            = "Regression: $candidateName vs $baselineName"
    engine_a_id     = $candidateEngine.id
    engine_b_id     = $baselineEngine.id
    position_set_id = $set[0].id
    game_count      = $MaxGames
    limit           = $limit
    color_order     = 'alternate'
    sprt            = @{ elo0 = $Elo0; elo1 = $Elo1; alpha = $Alpha; beta = $Beta }
}

Write-Host ''
Write-Host "Candidate: $candidateName ($CandidateImage)"
Write-Host "Baseline:  $baselineName ($BaselineImage)"
Write-Host "SPRT:      H0 Elo <= $Elo0, H1 Elo >= $Elo1, alpha $Alpha, beta $Beta, at most $MaxGames games"
Write-Host "Follow:    $Frontend/tournaments/$($tournament.id)"
Write-Host ''
if ($NoWait) { exit 0 }

$last = ''
while ($true) {
    $detail = Invoke-Arena 'GET' "/api/tournaments/$($tournament.id)" $null
    $stats = $detail.stats
    $sprt = $detail.sprt_status
    $elo = 'n/a'
    if ($null -ne $stats.elo) {
        $elo = Format-Signed $stats.elo
        if ($null -ne $stats.elo_error) { $elo += ' +/- ' + $stats.elo_error.ToString('F1', [Globalization.CultureInfo]::InvariantCulture) }
    }
    $line = '{0,5} games  W/D/L {1}/{2}/{3}  Elo {4}  LOS {5:P1}  LLR {6} ({7} .. {8})' -f `
        $stats.games, $stats.wins, $stats.draws, $stats.losses, $elo, $stats.los,
        (Format-Signed $sprt.llr 2), (Format-Signed $sprt.lower_bound 2), (Format-Signed $sprt.upper_bound 2)
    if ($line -ne $last) {
        Write-Host ('[{0:HH:mm:ss}] {1}' -f (Get-Date), $line)
        $last = $line
    }
    if (@('queued', 'running', 'created') -notcontains $detail.status) { break }
    Start-Sleep -Seconds $PollSeconds
}

Write-Host ''
switch ($detail.sprt_result) {
    'H1' { Write-Host "PASSED: $candidateName is stronger than $baselineName (H1 accepted)." -ForegroundColor Green; exit 0 }
    'H0' { Write-Host "FAILED: $candidateName is not stronger than $baselineName (H0 accepted)." -ForegroundColor Red; exit 1 }
    default { Write-Host "No decision: the tournament is $($detail.status) after $($stats.games) games." -ForegroundColor Yellow; exit 2 }
}

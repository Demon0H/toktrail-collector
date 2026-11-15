$ErrorActionPreference = 'Stop'
$arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
$arch = switch ($arch) { 'x64' { 'amd64' } 'arm64' { 'arm64' } default { throw 'Unsupported architecture' } }
$asset = "dh-tokenrank-windows-$arch.zip"
$base = 'https://github.com/Demon0H/toktrail-collector/releases/latest/download'
$tmp = Join-Path ([IO.Path]::GetTempPath()) ('dh-tokenrank-' + [guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    Invoke-WebRequest "$base/$asset" -OutFile (Join-Path $tmp $asset)
    Invoke-WebRequest "$base/SHA256SUMS" -OutFile (Join-Path $tmp 'SHA256SUMS')
    $line = Get-Content (Join-Path $tmp 'SHA256SUMS') | Where-Object { $_.EndsWith("  $asset") }
    if (@($line).Count -ne 1) { throw 'Missing unique checksum' }
    $expected = ($line -split '\s+')[0]
    if ((Get-FileHash (Join-Path $tmp $asset) -Algorithm SHA256).Hash -ine $expected) { throw 'SHA-256 mismatch' }
    Expand-Archive -LiteralPath (Join-Path $tmp $asset) -DestinationPath $tmp
    foreach ($required in @('dh-tokenrank.exe','toktrail.exe','toktrail-background.exe')) {
        if (-not (Test-Path -LiteralPath (Join-Path $tmp "dh-tokenrank-windows-$arch/$required") -PathType Leaf)) { throw "Incomplete Windows release: missing $required" }
    }
    $dest = Join-Path $env:LOCALAPPDATA 'DH-TokenRank'
    New-Item -ItemType Directory -Path $dest -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $tmp "dh-tokenrank-windows-$arch/dh-tokenrank.exe") -Destination $dest -Force
    Copy-Item -LiteralPath (Join-Path $tmp "dh-tokenrank-windows-$arch/toktrail.exe") -Destination $dest -Force
    Copy-Item -LiteralPath (Join-Path $tmp "dh-tokenrank-windows-$arch/toktrail-background.exe") -Destination $dest -Force
    Write-Host "Installed: $dest\dh-tokenrank.exe"
    Write-Host 'Next: read AGENT_SETUP.md and explicitly authorize sources.'
} finally {
    $resolved = [IO.Path]::GetFullPath($tmp)
    $root = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
    if ($resolved.StartsWith($root, [StringComparison]::OrdinalIgnoreCase) -and (Split-Path $resolved -Leaf).StartsWith('dh-tokenrank-')) { Remove-Item -LiteralPath $resolved -Recurse -Force }
}

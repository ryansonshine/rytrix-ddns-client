# Installs the latest rytrix-ddns release for Windows.
#   irm https://raw.githubusercontent.com/ryansonshine/rytrix-ddns-client/main/install.ps1 | iex
$ErrorActionPreference = 'Stop'

$repo = 'ryansonshine/rytrix-ddns-client'
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
$name = "rytrix-ddns_windows_$arch.zip"
$base = "https://github.com/$repo/releases/latest/download"
$tmp = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null

try {
    Write-Host "Downloading $name..."
    Invoke-WebRequest -UseBasicParsing -Uri "$base/$name" -OutFile "$tmp\$name"
    Invoke-WebRequest -UseBasicParsing -Uri "$base/checksums.txt" -OutFile "$tmp\checksums.txt"

    $line = Get-Content "$tmp\checksums.txt" | Where-Object { $_ -match " $([regex]::Escape($name))$" }
    $expected = ($line -split ' ')[0]
    $actual = (Get-FileHash "$tmp\$name" -Algorithm SHA256).Hash.ToLower()
    if (-not $expected -or $expected -ne $actual) { throw "Checksum mismatch for $name; not installing." }

    $dest = Join-Path $env:LOCALAPPDATA 'Programs\rytrix-ddns'
    New-Item -ItemType Directory -Force -Path $dest | Out-Null
    Expand-Archive -Force -Path "$tmp\$name" -DestinationPath $tmp
    Copy-Item -Force "$tmp\rytrix-ddns.exe" "$dest\rytrix-ddns.exe"

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (($userPath -split ';') -notcontains $dest) {
        [Environment]::SetEnvironmentVariable('Path', "$userPath;$dest", 'User')
        $env:Path = "$env:Path;$dest"
        Write-Host "Added $dest to your PATH. Open a new terminal to use it elsewhere."
    }

    & "$dest\rytrix-ddns.exe" version
    Write-Host 'Next: rytrix-ddns setup, then rytrix-ddns install from an administrator prompt.'
}
finally {
    Remove-Item -Recurse -Force $tmp
}

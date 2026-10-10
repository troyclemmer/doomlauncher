# Builds DoomLauncher.exe with a version derived from git, so the title bar
# identifies exactly which build a machine is running.
#
#   .\build.ps1            build only
#   .\build.ps1 -Install   build, then copy into the Doom folder
param([string]$DoomDir = "C:\Program Files (x86)\BD", [switch]$Install)

$ErrorActionPreference = "Stop"
$count = (git rev-list --count HEAD).Trim()
$sha   = (git rev-parse --short HEAD).Trim()
$dirty = if ((git status --porcelain).Length -gt 0) { "+" } else { "" }
$ver   = "v1.0.$count ($sha$dirty)"

Write-Host "building $ver"
go build -ldflags "-H=windowsgui -s -w -X 'main.version=$ver'" -o DoomLauncher.exe .
go build -o wadstat.exe ./tools/wadstat

if ($Install) {
    if (Get-Process -Name DoomLauncher -ErrorAction SilentlyContinue) {
        Stop-Process -Name DoomLauncher -Force; Start-Sleep -Milliseconds 500
    }
    Copy-Item DoomLauncher.exe (Join-Path $DoomDir "DoomLauncher.exe") -Force
    Copy-Item wadstat.exe (Join-Path $DoomDir "wadstat.exe") -Force
    Write-Host "installed to $DoomDir"
}

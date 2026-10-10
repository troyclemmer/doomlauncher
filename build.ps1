# Builds DoomLauncher.exe with a version derived from git, so the title bar
# identifies exactly which build a machine is running.
#
#   .\build.ps1            build only
#   .\build.ps1 -Install   build, then copy into the Doom folder
param([string]$DoomDir = "C:\Program Files (x86)\BD", [switch]$Install)

$ErrorActionPreference = "Stop"
# Major is the only hand-set number; the second is the commit count, so it
# increments on its own. Bump $major by hand for a release you consider a
# break from what came before.
$major = 1
$count = (git rev-list --count HEAD).Trim()
$sha   = (git rev-parse --short HEAD).Trim()
# A dirty tree means the hash does not describe what was actually built.
$dirty = if ((git status --porcelain).Length -gt 0) { "+dirty" } else { "" }
$ver   = "v$major.$count ($sha$dirty)"

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

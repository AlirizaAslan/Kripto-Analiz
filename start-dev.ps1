$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$apiDir = Join-Path $root 'apps\api'

$apiUrl = 'http://localhost:8081'
$webUrl = 'http://localhost:5174'

Write-Host "Starting PulseAlpha backend and frontend..."
Write-Host "Backend: $apiUrl"
Write-Host "Frontend: $webUrl"

$backendCommand = @"
Set-Location '$apiDir'
if (Test-Path '$root\.env') {
    Get-Content '$root\.env' | ForEach-Object {
        if (`$_ -match '^\s*([^#=]+)\s*=\s*(.*)') {
            [Environment]::SetEnvironmentVariable(`$matches[1].Trim(), `$matches[2].Trim(), 'Process')
        }
    }
}
`$env:GOCACHE = Join-Path (Get-Location) '.gocache'
`$env:GOTMPDIR = Join-Path (Get-Location) '.gotmp'
`$env:GOTELEMETRY = 'off'
if (!(Test-Path `$env:GOCACHE)) { New-Item -ItemType Directory -Path `$env:GOCACHE | Out-Null }
if (!(Test-Path `$env:GOTMPDIR)) { New-Item -ItemType Directory -Path `$env:GOTMPDIR | Out-Null }
go run .\cmd\server
"@

$frontendCommand = @"
Set-Location '$root'
`$env:PULSEALPHA_API_BASE_URL = '$apiUrl'
cmd /c npm --workspace apps/web run dev -- --host 0.0.0.0 --port 5174
"@

Start-Process powershell -ArgumentList @('-NoExit', '-Command', $backendCommand) | Out-Null
Start-Sleep -Seconds 2
Start-Process powershell -ArgumentList @('-NoExit', '-Command', $frontendCommand) | Out-Null

Write-Host 'Both services were launched in separate PowerShell windows.'

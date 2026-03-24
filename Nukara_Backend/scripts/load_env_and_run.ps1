# 加载 .env 文件并启动 gateway
$envFile = "$PSScriptRoot\..\env"
if (!(Test-Path $envFile)) {
    $envFile = "$PSScriptRoot\..\.env"
}

Get-Content $envFile | ForEach-Object {
    if ($_ -match '^\s*([^#][^=]+)=(.*)$') {
        $key = $matches[1].Trim()
        $val = $matches[2].Trim()
        [System.Environment]::SetEnvironmentVariable($key, $val, "Process")
    }
}

Write-Host "[env loaded] NUKARA_ASTRON_BASE_URL=$env:NUKARA_ASTRON_BASE_URL"
Write-Host "[env loaded] NUKARA_ASTRON_CHAT_MODEL=$env:NUKARA_ASTRON_CHAT_MODEL"

Set-Location "$PSScriptRoot\.."
go run ./cmd/gateway/main.go

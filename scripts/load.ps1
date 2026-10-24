param([int]$Count = 25, [int]$DelayMs = 600)
$ErrorActionPreference = "Stop"
for ($i = 1; $i -le $Count; $i++) {
    $status = curl.exe --silent --output NUL --write-out "%{http_code}" "http://localhost:8080/work?delay_ms=$DelayMs&fail=true"
    Write-Host "request $i/$Count -> HTTP $status"
}
Write-Host "Check http://localhost:9090/alerts and http://localhost:3000"

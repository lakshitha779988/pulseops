$ErrorActionPreference = "Stop"
Write-Host "Checking PulseOps health..."
$health = Invoke-RestMethod 'http://localhost:8080/healthz'
if ($health -ne 'healthy') { throw 'Health check failed' }
$ok = Invoke-RestMethod 'http://localhost:8080/work?delay_ms=30'
if (-not $ok.success) { throw 'Expected successful work response' }
$code = curl.exe --silent --output NUL --write-out '%{http_code}' 'http://localhost:8080/work?fail=true'
if ($code -ne '500') { throw "Expected HTTP 500, got $code" }
$metrics = (Invoke-WebRequest 'http://localhost:8080/metrics').Content
if (-not ($metrics -match 'pulseops_http_requests_total')) { throw 'Metrics are missing' }
Write-Host 'Smoke tests passed: health, successful request, failure and metrics.' -ForegroundColor Green

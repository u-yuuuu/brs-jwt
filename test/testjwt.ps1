Write-Host "JWT Test" -ForegroundColor Cyan

# Логин
$body = '{"username":"sergey.petrov","password":"pass456"}'
$response = Invoke-RestMethod -Uri "http://localhost:8080/api/jwt/login" -Method Post -Body $body -ContentType "application/json"
$token = $response.token
Write-Host "Token: $token" -ForegroundColor Green

# Верифицирование
$headers = @{Authorization = "Bearer $token"}
$result = Invoke-RestMethod -Uri "http://localhost:8080/api/jwt/verify" -Method Get -Headers $headers
Write-Host "User: $($result.username)" -ForegroundColor Green
Write-Host "Type: $($result.user_type)" -ForegroundColor Green
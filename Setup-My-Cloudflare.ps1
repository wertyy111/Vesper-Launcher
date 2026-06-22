# UTF-8 encoding
$OutputEncoding = [System.Text.Encoding]::UTF8

Write-Host "=========================================================" -ForegroundColor Cyan
Write-Host "   Автоматическая настройка вашего личного Cloudflare" -ForegroundColor Cyan
Write-Host "=========================================================" -ForegroundColor Cyan
Write-Host ""

$repoRoot = $PSScriptRoot
$backendDir = Join-Path $repoRoot "backend\vesper-account-api"
$wranglerTomlPath = Join-Path $backendDir "wrangler.toml"

# 1. Wrangler Login
Write-Host "[1/7] Авторизация в вашем аккаунте Cloudflare..." -ForegroundColor Yellow
Write-Host "Сейчас откроется браузер. Нажмите 'Allow' / 'Разрешить', чтобы войти." -ForegroundColor Gray
Push-Location $backendDir
try {
    & npx wrangler login
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Ошибка при авторизации в Cloudflare."
        exit $LASTEXITCODE
    }
} finally {
    Pop-Location
}

# 2. Create D1 Database
Write-Host ""
Write-Host "[2/7] Создание новой базы данных D1 'vesper-account-db'..." -ForegroundColor Yellow
$dbId = $null
$createOutput = $null
Push-Location $backendDir
try {
    # Run creation
    $createOutput = & npx wrangler d1 create vesper-account-db 2>&1 | Out-String
    Write-Host $createOutput -ForegroundColor Gray

    # Parse database_id
    if ($createOutput -match 'database_id\s*=\s*"([^"]+)"') {
        $dbId = $Matches[1]
    } else {
        # Check if database already exists
        Write-Host "Проверка, существует ли уже база данных..." -ForegroundColor Gray
        $listOutput = & npx wrangler d1 list 2>&1 | Out-String
        if ($listOutput -match 'vesper-account-db\s*\|\s*([a-f0-9-]+)') {
            $dbId = $Matches[1]
        }
    }
} finally {
    Pop-Location
}

if (-not $dbId) {
    Write-Error "Не удалось получить ID базы данных Cloudflare D1."
    exit 1
}

Write-Host "ID вашей новой базы данных: $dbId" -ForegroundColor Green

# 3. Update wrangler.toml
Write-Host ""
Write-Host "[3/7] Обновление конфигурации wrangler.toml..." -ForegroundColor Yellow

$tomlText = Get-Content -Path $wranglerTomlPath -Raw
# Remove account_id line so wrangler uses the logged in account automatically
$tomlText = $tomlText -replace '(?m)^account_id\s*=.*?\r?\n', ''
# Replace database_id
$tomlText = $tomlText -replace 'database_id\s*=\s*"[^"]+"', "database_id = `"$dbId`""

Set-Content -Path $wranglerTomlPath -Value $tomlText -Encoding UTF8
Write-Host "Конфигурационный файл wrangler.toml успешно обновлен." -ForegroundColor Green

# 4. Apply migrations
Write-Host ""
Write-Host "[4/7] Создание таблиц в базе данных D1 (миграции)..." -ForegroundColor Yellow
Push-Location $backendDir
try {
    # Send "y" to wrangler confirmation prompt
    $processInfo = New-Object System.Diagnostics.ProcessStartInfo
    $processInfo.FileName = "cmd.exe"
    $processInfo.Arguments = "/c npx wrangler d1 migrations apply vesper-account-db --remote"
    $processInfo.WorkingDirectory = $backendDir
    $processInfo.RedirectStandardInput = $true
    $processInfo.UseShellExecute = $false
    $process = [System.Diagnostics.Process]::Start($processInfo)
    $process.StandardInput.WriteLine("y")
    $process.StandardInput.Close()
    $process.WaitForExit()
    
    if ($process.ExitCode -ne 0) {
        Write-Error "Не удалось применить миграции к базе данных."
        exit $process.ExitCode
    }
} finally {
    Pop-Location
}
Write-Host "Таблицы базы данных успешно созданы!" -ForegroundColor Green

# 5. Set GITHUB_STORAGE_TOKEN secret
Write-Host ""
Write-Host "[5/7] Установка токена GitHub в секреты Cloudflare..." -ForegroundColor Yellow
$token = "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"
Push-Location $backendDir
try {
    $processInfo = New-Object System.Diagnostics.ProcessStartInfo
    $processInfo.FileName = "cmd.exe"
    $processInfo.Arguments = "/c npx wrangler secret put GITHUB_STORAGE_TOKEN"
    $processInfo.WorkingDirectory = $backendDir
    $processInfo.RedirectStandardInput = $true
    $processInfo.UseShellExecute = $false
    $process = [System.Diagnostics.Process]::Start($processInfo)
    $process.StandardInput.WriteLine($token)
    $process.StandardInput.Close()
    $process.WaitForExit()

    if ($process.ExitCode -ne 0) {
        Write-Error "Не удалось сохранить токен GitHub в секреты Cloudflare."
        exit $process.ExitCode
    }
} finally {
    Pop-Location
}
Write-Host "Токен GitHub успешно добавлен в защищенное облачное хранилище!" -ForegroundColor Green

# 6. Deploy Cloudflare Worker
Write-Host ""
Write-Host "[6/7] Публикация бэкенда на вашем аккаунте Cloudflare..." -ForegroundColor Yellow
$workerUrl = $null
Push-Location $backendDir
try {
    $deployOutput = & npx wrangler deploy 2>&1 | Out-String
    Write-Host $deployOutput -ForegroundColor Gray

    # Parse worker URL
    if ($deployOutput -match '(https://[a-zA-Z0-9-]+\.[a-zA-Z0-9-]+\.workers\.dev)') {
        $workerUrl = $Matches[1]
    }
} finally {
    Pop-Location
}

if (-not $workerUrl) {
    Write-Error "Не удалось получить URL опубликованного воркера."
    exit 1
}

Write-Host "Бэкенд успешно опубликован! URL: $workerUrl" -ForegroundColor Green

# 7. Update launcher config & rebuild
Write-Host ""
Write-Host "[7/7] Обновление конфигурации лаунчера под ваш бэкенд..." -ForegroundColor Yellow

# Replace base URL in wrangler.toml with its own deployed URL for asset prefixing
$tomlText = Get-Content -Path $wranglerTomlPath -Raw
$tomlText = $tomlText -replace 'PUBLIC_API_BASE_URL\s*=\s*"[^"]+"', "PUBLIC_API_BASE_URL = `"$workerUrl`""
Set-Content -Path $wranglerTomlPath -Value $tomlText -Encoding UTF8

# Redeploy worker once with updated base URL
Push-Location $backendDir
try {
    & npx wrangler deploy > $null
} finally {
    Pop-Location
}

$launcherDir = Join-Path $repoRoot "MinecraftCheatLauncher"
$outDir = "D:\Новая папка (14)"

# Helper function to update JSON urls
function Update-JsonUrl {
    param(
        [string]$Path,
        [string]$NewUrl
    )
    if (Test-Path $Path) {
        $json = Get-Content -Path $Path -Raw | ConvertFrom-Json
        
        # Replace URLs
        if ($json.RegisterUrl) { $json.RegisterUrl = "$NewUrl/api/v1/auth/register" }
        if ($json.LoginUrl) { $json.LoginUrl = "$NewUrl/api/v1/auth/login" }
        if ($json.CredentialInfoUrl) { $json.CredentialInfoUrl = "$NewUrl/api/v1/auth/credential-info" }
        if ($json.MeUrl) { $json.MeUrl = "$NewUrl/api/v1/auth/me" }
        if ($json.LogoutUrl) { $json.LogoutUrl = "$NewUrl/api/v1/auth/logout" }

        if ($json.PublishUrl) { $json.PublishUrl = "$NewUrl/api/v1/profile/skin" }
        if ($json.LookupByUsernameUrl) { $json.LookupByUsernameUrl = "$NewUrl/api/v1/profile/skin/by-username" }
        if ($json.LookupByUuidUrl) { $json.LookupByUuidUrl = "$NewUrl/api/v1/profile/skin/by-uuid" }

        $json | ConvertTo-Json -Depth 10 | Set-Content -Path $Path -Encoding UTF8
    }
}

# Update source JSON configs
Update-JsonUrl (Join-Path $launcherDir "account-sync.json") $workerUrl
Update-JsonUrl (Join-Path $launcherDir "skin-sync.json") $workerUrl

# Rebuild client
Write-Host "Перекомпиляция лаунчера..." -ForegroundColor Gray
Push-Location $repoRoot
try {
    & dotnet build
} finally {
    Pop-Location
}

# Deploy to output directory
Write-Host "Копирование файлов в $outDir..." -ForegroundColor Gray
Copy-Item -Path "$launcherDir\bin\Debug\net10.0-windows\*" -Destination $outDir -Recurse -Force -ErrorAction SilentlyContinue

# Update deployed JSON configs just in case
Update-JsonUrl (Join-Path $outDir "account-sync.json") $workerUrl
Update-JsonUrl (Join-Path $outDir "skin-sync.json") $workerUrl

Write-Host ""
Write-Host "=========================================================" -ForegroundColor Green
Write-Host "  УСПЕШНО ЗАВЕРШЕНО! Всё настроено на ваш личный аккаунт!" -ForegroundColor Green
Write-Host "=========================================================" -ForegroundColor Green
Write-Host "Новый адрес бэкенда: $workerUrl" -ForegroundColor Gray
Write-Host ""
Read-Host "Нажмите Enter для выхода"

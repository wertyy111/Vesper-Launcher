@echo off
chcp 65001 > NUL
echo ===================================================
echo  Vesper Launcher Cloudflare Worker Deployer
echo ===================================================
echo.
cd /d "%~dp0backend\vesper-account-api"

echo [1/3] Запуск авторизации в Cloudflare Wrangler...
call npx wrangler login
if %ERRORLEVEL% neq 0 (
    echo.
    echo [ERROR] Ошибка авторизации в Cloudflare. Пожалуйста, попробуйте еще раз.
    goto end
)

echo.
echo [2/3] Обновление секретного токена GITHUB_STORAGE_TOKEN в Cloudflare...
echo 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| call npx wrangler secret put GITHUB_STORAGE_TOKEN --config wrangler.toml
if %ERRORLEVEL% neq 0 (
    echo.
    echo [ERROR] Не удалось установить секрет. Проверьте подключение.
    goto end
)

echo.
echo [3/3] Деплой обновленного Cloudflare Worker...
call npx wrangler deploy --config wrangler.toml
if %ERRORLEVEL% neq 0 (
    echo.
    echo [ERROR] Не удалось выполнить деплой воркера.
    goto end
)

echo.
echo ===================================================
echo  УСПЕШНО! Токен обновлен, воркер развернут!
echo ===================================================
:end
echo.
echo Нажмите любую клавишу для выхода...
pause > NUL

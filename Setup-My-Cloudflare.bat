@echo off
chcp 65001 > NUL
cd /d "%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -File .\Setup-My-Cloudflare.ps1

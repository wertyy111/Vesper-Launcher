using System;
using System.Diagnostics;
using System.IO;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using System.Windows;
using NetSparkleUpdater;
using NetSparkleUpdater.Enums;
using NetSparkleUpdater.Events;
using NetSparkleUpdater.SignatureVerifiers;

namespace MinecraftCheatLauncher;

internal sealed class LauncherAutoUpdateService
{
    private const long MinimumPreferredDownloadFreeSpaceBytes = 512L * 1024 * 1024;

    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        PropertyNameCaseInsensitive = true,
        ReadCommentHandling = JsonCommentHandling.Skip,
        AllowTrailingCommas = true
    };

    private readonly Action<Exception, string> _logError;
    private readonly Action<string> _logInfo;
    private SparkleUpdater? _updater;
    private int _fallbackLaunchRequested;

    public LauncherAutoUpdateService(Action<Exception, string> logError, Action<string> logInfo)
    {
        _logError = logError;
        _logInfo = logInfo;
    }

    public event Action? FallbackLaunchRequested;
    public event Action<LauncherUpdateUiState>? UiStateChanged;

    public async Task<bool> RunBeforeLaunchAsync()
    {
        try
        {
            ReportStatus("Проверяем обновления...", "Подключаемся к серверу обновлений...");

            var config = LoadConfig();
            if (config is null || !config.Enabled || !config.CheckOnStartup)
            {
                ReportStatus("Запускаем лаунчер...", "Автообновление отключено.", 100, false, "Готово");
                return true;
            }

            if (!Uri.TryCreate(config.AppCastUrl, UriKind.Absolute, out var appCastUri) ||
                !string.Equals(appCastUri.Scheme, Uri.UriSchemeHttps, StringComparison.OrdinalIgnoreCase))
            {
                ReportStatus("Запускаем лаунчер...", "Ссылка на обновления настроена неверно.", 100, false, "Готово");
                _logInfo("Автообновление отключено: укажи HTTPS URL в update-config.json.");
                return true;
            }

            if (string.IsNullOrWhiteSpace(config.PublicEd25519Key))
            {
                ReportStatus("Запускаем лаунчер...", "Публичный ключ обновлений не задан.", 100, false, "Готово");
                _logInfo("Автообновление отключено: не задан PublicEd25519Key в update-config.json.");
                return true;
            }

            var signatureVerifier = new Ed25519Checker(
                SecurityMode.Strict,
                config.PublicEd25519Key,
                string.Empty,
                readFileBeingVerifiedInChunks: true,
                chunkSize: 5 * 1024 * 1024);

            if (!signatureVerifier.HasValidKeyInformation())
            {
                ReportStatus("Запускаем лаунчер...", "Ключ подписи обновлений некорректен.", 100, false, "Готово");
                _logInfo("Автообновление отключено: публичный ключ NetSparkle некорректен.");
                return true;
            }

            _updater = new SparkleUpdater(config.AppCastUrl, signatureVerifier)
            {
                UserInteractionMode = UserInteractionMode.DownloadAndInstall,
                // We let our own installer relaunch the launcher after install.
                // NetSparkle relaunching the pre-update executable path can reopen an old copy and create an update loop.
                RelaunchAfterUpdate = false,
                CustomInstallerArguments = BuildInstallerArguments(config.CustomInstallerArguments),
                CheckServerFileName = false
            };

            var downloadDirectory = ResolvePreferredDownloadDirectory(config.TempDownloadDirectory);
            Directory.CreateDirectory(downloadDirectory);
            _updater.TmpDownloadFilePath = downloadDirectory;
            _logInfo($"Папка временной загрузки обновлений: {downloadDirectory}");

            var launchDecision = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);

            _updater.UpdateCheckStarted += _ =>
            {
                _logInfo("Update check started.");
                ReportStatus("Проверяем обновления...", "Подключаемся к серверу обновлений...");
            };

            _updater.UpdateCheckFinished += (_, status) =>
            {
                _logInfo($"Update check finished with status: {status}.");

                if (status == UpdateStatus.UpdateAvailable)
                {
                    var availableUpdates = _updater.LatestAppCastItems;
                    var updateItem = availableUpdates is not null && availableUpdates.Count > 0
                        ? availableUpdates[0]
                        : null;

                    if (updateItem is null)
                    {
                        ContinueWithCurrentVersion(
                            launchDecision,
                            "Список обновлений пришел пустым. Запускаем текущую версию...",
                            "NetSparkle сообщил о доступном обновлении, но не вернул пакет для скачивания.");
                        return;
                    }

                    ReportStatus("Найдена новая версия.", "Начинаем скачивание обновления...", 0, true, "Подготовка...");
                    _logInfo($"Найдено обновление до запуска окна. Версия: {updateItem.Version}. AppCast: {appCastUri}");
                    launchDecision.TrySetResult(false);
                    _ = BeginSilentDownloadAndInstallAsync(updateItem, launchDecision);
                    return;
                }

                ReportStatus("Обновлений не найдено.", "Запускаем текущую версию лаунчера...", 100, false, "Готово");
                launchDecision.TrySetResult(true);
            };

            _updater.DownloadStarted += (_, path) =>
            {
                _logInfo($"Update download started. Path: {path}");
                ReportStatus("Скачиваем обновление...", "Подготавливаем файл обновления...", 0, true, "Подготовка...");
            };

            _updater.DownloadMadeProgress += (_, _, args) =>
            {
                ReportDownloadProgress(args);
            };

            _updater.DownloadFinished += (_, path) =>
            {
                _logInfo($"Update download finished. Path: {path}");
                ReportStatus("Загрузка завершена.", "Проверяем пакет обновления...", 100, false, "100%");
            };

            _updater.DownloadHadError += (_, _, _) => ContinueWithCurrentVersion(
                launchDecision,
                "Не удалось скачать обновление. Запускаем текущую версию...",
                "Обновление не скачалось. Запускаем текущую версию.");

            _updater.DownloadCanceled += (_, _) => ContinueWithCurrentVersion(
                launchDecision,
                "Скачивание обновления отменено. Запускаем текущую версию...",
                "Скачивание обновления было отменено. Запускаем текущую версию.");

            _updater.DownloadedFileIsCorrupt += (_, _) => ContinueWithCurrentVersion(
                launchDecision,
                "Файл обновления поврежден. Запускаем текущую версию...",
                "Файл обновления поврежден. Запускаем текущую версию.");

            _updater.DownloadedFileThrewWhileCheckingSignature += (_, _) => ContinueWithCurrentVersion(
                launchDecision,
                "Подпись обновления не прошла проверку. Запускаем текущую версию...",
                "Подпись обновления не прошла проверку. Запускаем текущую версию.");

            _updater.InstallUpdateFailed += (failureReason, _) =>
            {
                ContinueWithCurrentVersion(
                    launchDecision,
                    "Не удалось установить обновление. Запускаем текущую версию...",
                    $"Установка обновления не удалась: {failureReason}. Запускаем текущую версию.");
                return true;
            };

            _updater.PreparingToExit += (_, _) =>
            {
                _logInfo("Updater is preparing launcher shutdown before install.");
                ReportStatus("Закрываем лаунчер для обновления...", "Не закрывайте окно. Установка уже запускается.", 100, false, "Установка...");
            };

            _updater.CloseApplication += () =>
            {
                _logInfo("NetSparkle requested application shutdown.");
                ReportStatus("Завершаем лаунчер...", "Сейчас откроется установщик обновления.", 100, false, "Закрытие...");

                if (Application.Current is null)
                {
                    return;
                }

                void ShutdownApplication()
                {
                    try
                    {
                        Application.Current.Shutdown();
                    }
                    catch (Exception ex)
                    {
                        _logError(ex, "Ошибка при закрытии лаунчера для обновления");
                    }
                }

                if (Application.Current.Dispatcher.CheckAccess())
                {
                    ShutdownApplication();
                }
                else
                {
                    Application.Current.Dispatcher.BeginInvoke(new Action(ShutdownApplication));
                }
            };

            _updater.InstallerProcessAboutToStart += (process, downloadFilePath) =>
            {
                TryPrepareInstallerProcess(process, downloadFilePath);
                _logInfo("Installer process is about to start.");
                _logInfo($"Installer start info: FileName={process.StartInfo.FileName}; Arguments={process.StartInfo.Arguments}; Download={downloadFilePath}");
                ReportStatus("Запускаем установку...", "Обновление скачано. Сейчас начнется установка.", 100, false, "Установка...");
                return true;
            };

            ReportStatus("Ищем новую версию...", "Получаем данные с сервера обновлений...");
            _ = _updater.CheckForUpdatesQuietly(ignoreSkippedVersions: false);
            _logInfo($"Автообновление включено. Проверяем {appCastUri} до запуска окна.");

            var startupTimeout = TimeSpan.FromSeconds(config.StartupDecisionTimeoutSeconds <= 0
                ? 15
                : config.StartupDecisionTimeoutSeconds);

            var completedTask = await Task.WhenAny(launchDecision.Task, Task.Delay(startupTimeout)).ConfigureAwait(false);
            if (completedTask != launchDecision.Task)
            {
                ReportStatus("Сервер обновлений не ответил.", "Запускаем текущую версию лаунчера...", 100, false, "Готово");
                _logInfo($"Проверка обновления на старте не уложилась в {startupTimeout.TotalSeconds:0} сек. Запускаем текущую версию.");
                return true;
            }

            return await launchDecision.Task.ConfigureAwait(false);
        }
        catch (Exception ex)
        {
            ReportStatus("Запускаем лаунчер...", "Автообновление временно недоступно.", 100, false, "Готово");
            _logError(ex, "Ошибка инициализации автообновления");
            return true;
        }
    }

    private void ContinueWithCurrentVersion(
        TaskCompletionSource<bool> launchDecision,
        string statusMessage,
        string logMessage)
    {
        ReportStatus(statusMessage, "Открываем установленную версию лаунчера...", 100, false, "Готово");
        _logInfo(logMessage);
        launchDecision.TrySetResult(true);

        if (Interlocked.Exchange(ref _fallbackLaunchRequested, 1) == 0)
        {
            FallbackLaunchRequested?.Invoke();
        }
    }

    private string BuildInstallerArguments(string? configuredArguments)
    {
        var baseArguments = string.IsNullOrWhiteSpace(configuredArguments)
            ? "/SILENT /NORESTART /SP- /CLOSEAPPLICATIONS"
            : configuredArguments.Trim();

        var installDirectory = Path.GetFullPath(AppContext.BaseDirectory)
            .TrimEnd(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar);

        if (baseArguments.IndexOf("/DIR", StringComparison.OrdinalIgnoreCase) >= 0 ||
            baseArguments.IndexOf("--dir", StringComparison.OrdinalIgnoreCase) >= 0)
        {
            return AppendDesktopShortcutPreference(baseArguments);
        }

        return AppendDesktopShortcutPreference($"{baseArguments} /DIR=\"{installDirectory}\"");
    }

    private static string AppendDesktopShortcutPreference(string arguments)
    {
        if (arguments.IndexOf("--no-desktop-shortcut", StringComparison.OrdinalIgnoreCase) >= 0)
        {
            return arguments;
        }

        var desktopShortcutPath = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.DesktopDirectory),
            "Vesper Launcher.lnk");
        return File.Exists(desktopShortcutPath)
            ? arguments
            : $"{arguments} --no-desktop-shortcut";
    }

    private async Task BeginSilentDownloadAndInstallAsync(
        AppCastItem updateItem,
        TaskCompletionSource<bool> launchDecision)
    {
        if (_updater is null)
        {
            ContinueWithCurrentVersion(
                launchDecision,
                "Сервис обновления недоступен. Запускаем текущую версию...",
                "Попытка начать загрузку обновления была отменена: SparkleUpdater не инициализирован.");
            return;
        }

        try
        {
            var fileNameWithExtension = BuildDownloadFileName(updateItem);
            _updater.TmpDownloadFileNameWithExtension = fileNameWithExtension;
            _logInfo($"Starting silent download for version {updateItem.Version}.");
            _logInfo($"Temporary download file name: {fileNameWithExtension}");
            _logInfo($"Installer arguments: {_updater.CustomInstallerArguments}");
            await _updater.InitAndBeginDownload(updateItem).ConfigureAwait(false);
        }
        catch (Exception ex)
        {
            _logError(ex, "Ошибка при запуске тихого скачивания обновления");
            ContinueWithCurrentVersion(
                launchDecision,
                "Не удалось запустить скачивание обновления. Запускаем текущую версию...",
                "Не удалось запустить InitAndBeginDownload. Запускаем текущую версию.");
        }
    }

    private void TryPrepareInstallerProcess(Process process, string? downloadFilePath)
    {
        try
        {
            var launchablePath = EnsureLaunchableInstallerPath(process.StartInfo.FileName, downloadFilePath);
            if (string.IsNullOrWhiteSpace(launchablePath) ||
                string.Equals(launchablePath, process.StartInfo.FileName, StringComparison.OrdinalIgnoreCase))
            {
                return;
            }

            process.StartInfo.FileName = launchablePath;
            var workingDirectory = Path.GetDirectoryName(launchablePath);
            if (!string.IsNullOrWhiteSpace(workingDirectory))
            {
                process.StartInfo.WorkingDirectory = workingDirectory;
            }

            _logInfo($"Prepared launchable installer copy: {launchablePath}");
        }
        catch (Exception ex)
        {
            _logError(ex, "РћС€РёР±РєР° РїСЂРё РїРѕРґРіРѕС‚РѕРІРєРµ С„Р°Р№Р»Р° СѓСЃС‚Р°РЅРѕРІС‰РёРєР° Рє Р·Р°РїСѓСЃРєСѓ");
        }
    }

    private static string? EnsureLaunchableInstallerPath(string? processFileName, string? downloadFilePath)
    {
        if (!OperatingSystem.IsWindows())
        {
            return processFileName;
        }

        var candidatePath = ChooseExistingPath(processFileName, downloadFilePath);
        if (string.IsNullOrWhiteSpace(candidatePath) || !File.Exists(candidatePath))
        {
            return processFileName;
        }

        if (!string.IsNullOrWhiteSpace(Path.GetExtension(candidatePath)))
        {
            return candidatePath;
        }

        var launchablePath = candidatePath + ".exe";
        File.Copy(candidatePath, launchablePath, overwrite: true);
        return launchablePath;
    }

    private static string? ChooseExistingPath(string? primaryPath, string? secondaryPath)
    {
        if (!string.IsNullOrWhiteSpace(primaryPath) && File.Exists(primaryPath))
        {
            return primaryPath;
        }

        if (!string.IsNullOrWhiteSpace(secondaryPath) && File.Exists(secondaryPath))
        {
            return secondaryPath;
        }

        return primaryPath ?? secondaryPath;
    }

    private static string BuildDownloadFileName(AppCastItem updateItem)
    {
        var sourceName = TryGetFileNameFromDownloadLink(updateItem.DownloadLink);
        var extension = Path.GetExtension(sourceName);
        if (string.IsNullOrWhiteSpace(extension))
        {
            extension = ".exe";
        }

        var baseName = Path.GetFileNameWithoutExtension(sourceName);
        if (string.IsNullOrWhiteSpace(baseName))
        {
            baseName = $"VesperLauncherUpdate-{NormalizeVersionForFileName(updateItem.Version?.ToString())}";
        }

        return $"{SanitizeFileName(baseName)}{extension}";
    }

    private static string TryGetFileNameFromDownloadLink(string? downloadLink)
    {
        if (string.IsNullOrWhiteSpace(downloadLink))
        {
            return string.Empty;
        }

        if (Uri.TryCreate(downloadLink, UriKind.Absolute, out var absoluteUri))
        {
            return Path.GetFileName(absoluteUri.LocalPath);
        }

        if (Uri.TryCreate(downloadLink, UriKind.Relative, out var relativeUri))
        {
            return Path.GetFileName(relativeUri.OriginalString);
        }

        return Path.GetFileName(downloadLink);
    }

    private static string NormalizeVersionForFileName(string? version)
    {
        return string.IsNullOrWhiteSpace(version)
            ? "latest"
            : SanitizeFileName(version);
    }

    private static string SanitizeFileName(string? name)
    {
        if (string.IsNullOrWhiteSpace(name))
        {
            return "update";
        }

        var invalidCharacters = Path.GetInvalidFileNameChars();
        var buffer = name.Trim().ToCharArray();
        for (var i = 0; i < buffer.Length; i++)
        {
            if (Array.IndexOf(invalidCharacters, buffer[i]) >= 0)
            {
                buffer[i] = '-';
            }
        }

        var sanitized = new string(buffer).Trim().Trim('.');
        return string.IsNullOrWhiteSpace(sanitized) ? "update" : sanitized;
    }

    private void ReportDownloadProgress(ItemDownloadProgressEventArgs args)
    {
        var detailMessage = args.TotalBytesToReceive > 0
            ? $"{FormatBytes(args.BytesReceived)} / {FormatBytes(args.TotalBytesToReceive)}"
            : $"Скачано {FormatBytes(args.BytesReceived)}";

        ReportStatus(
            "Скачиваем обновление...",
            detailMessage,
            args.ProgressPercentage,
            false,
            $"{Math.Clamp(args.ProgressPercentage, 0, 100)}%");
    }

    private void ReportStatus(
        string message,
        string? detailMessage = null,
        int? progressPercent = null,
        bool isIndeterminate = true,
        string? progressText = null)
    {
        UiStateChanged?.Invoke(new LauncherUpdateUiState
        {
            Message = message,
            DetailMessage = detailMessage,
            ProgressPercent = progressPercent,
            IsIndeterminate = isIndeterminate,
            ProgressText = progressText
        });
    }

    private static string FormatBytes(long bytes)
    {
        if (bytes <= 0)
        {
            return "0 Б";
        }

        string[] units = ["Б", "КБ", "МБ", "ГБ"];
        double value = bytes;
        var unitIndex = 0;

        while (value >= 1024 && unitIndex < units.Length - 1)
        {
            value /= 1024;
            unitIndex++;
        }

        return unitIndex == 0
            ? $"{value:0} {units[unitIndex]}"
            : $"{value:0.0} {units[unitIndex]}";
    }

    private static LauncherUpdateConfig? LoadConfig()
    {
        foreach (var path in GetConfigCandidatePaths())
        {
            if (!File.Exists(path))
            {
                continue;
            }

            var json = File.ReadAllText(path);
            return JsonSerializer.Deserialize<LauncherUpdateConfig>(json, JsonOptions);
        }

        return null;
    }

    private static string[] GetConfigCandidatePaths() =>
    [
        Path.Combine(AppContext.BaseDirectory, "update-config.json"),
        Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "update-config.json"))
    ];

    private static string ResolvePath(string path)
    {
        var expanded = Environment.ExpandEnvironmentVariables(path);
        return Path.IsPathRooted(expanded)
            ? expanded
            : Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, expanded));
    }

    private string ResolvePreferredDownloadDirectory(string? configuredPath)
    {
        var installDirectoryFallback = Path.Combine(AppContext.BaseDirectory, ".launcher-updates");
        var configuredDirectory = string.IsNullOrWhiteSpace(configuredPath)
            ? installDirectoryFallback
            : ResolvePath(configuredPath);

        if (!HasEnoughFreeSpace(configuredDirectory, MinimumPreferredDownloadFreeSpaceBytes, out var configuredFreeSpace))
        {
            if (!string.Equals(configuredDirectory, installDirectoryFallback, StringComparison.OrdinalIgnoreCase))
            {
                _logInfo(
                    $"Недостаточно места для обновления в '{configuredDirectory}' " +
                    $"({FormatBytes(configuredFreeSpace)} свободно). Переключаемся на '{installDirectoryFallback}'.");
            }

            return installDirectoryFallback;
        }

        return configuredDirectory;
    }

    private static bool HasEnoughFreeSpace(string directoryPath, long minimumBytes, out long freeBytes)
    {
        freeBytes = 0;

        try
        {
            var fullPath = Path.GetFullPath(directoryPath);
            var rootPath = Path.GetPathRoot(fullPath);
            if (string.IsNullOrWhiteSpace(rootPath))
            {
                return true;
            }

            var driveInfo = new DriveInfo(rootPath);
            freeBytes = driveInfo.AvailableFreeSpace;
            return freeBytes >= minimumBytes;
        }
        catch
        {
            // If free space can't be determined, don't block the update path here.
            return true;
        }
    }

    private sealed class LauncherUpdateConfig
    {
        public bool Enabled { get; init; }
        public bool CheckOnStartup { get; init; } = true;
        public string? AppCastUrl { get; init; }
        public string? PublicEd25519Key { get; init; }
        public string? CustomInstallerArguments { get; init; }
        public bool RelaunchAfterUpdate { get; init; } = true;
        public int StartupDecisionTimeoutSeconds { get; init; } = 15;
        public string TempDownloadDirectory { get; init; } = ".launcher-updates";
    }
}

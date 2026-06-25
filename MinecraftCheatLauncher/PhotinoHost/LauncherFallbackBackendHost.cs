using System.IO;
using System.Text.Json;
using MinecraftCheatLauncher.Core;
using MinecraftCheatLauncher.Platform;

namespace MinecraftCheatLauncher.PhotinoHost;

internal sealed class LauncherFallbackBackendHost : ILauncherBackendHost
{
    private readonly IPlatformService _platform;
    private readonly AccountFriendsService _accountFriends;
    private readonly object _sync = new();
    private string _activeSection = "none";
    private string _activeSettingsTab = "launcher";
    private string _statusText;
    private string _progressText;
    private string _selectedVersionKey = "vanilla-1.21";
    private int _memoryMb = 4096;
    private bool _autoMemory = true;
    private bool _useSystemJava = true;
    private bool _showJvmArgs;
    private string _javaPath = string.Empty;
    private string _extraJvmArgs = string.Empty;
    private string _modsSearch = string.Empty;
    private string _modsCategory = "РњРѕРґС‹";

    public LauncherFallbackBackendHost(IPlatformService platform)
    {
        _platform = platform;
        _accountFriends = new AccountFriendsService(platform);
        _statusText = $"{platform.DisplayName}: Р±Р°Р·РѕРІС‹Р№ cross-platform СЂРµР¶РёРј.";
        _progressText = "WPF РѕС‚РєР»СЋС‡С‘РЅ. РџРµСЂРµРЅРѕСЃ Р·Р°РїСѓСЃРєР° Minecraft Р±СѓРґРµС‚ СЃР»РµРґСѓСЋС‰РёРј С€Р°РіРѕРј.";
    }

    public void Start()
    {
    }

    public Task<bool> WaitForLauncherReadyAsync()
    {
        return Task.FromResult(true);
    }

    public Task<object> GetSnapshotAsync()
    {
        lock (_sync)
        {
            return Task.FromResult<object>(new
            {
                phase = "ready",
                errorMessage = (string?)null,
                update = new
                {
                    message = _statusText,
                    detailMessage = _progressText,
                    progressPercent = 0,
                    isIndeterminate = false,
                    progressText = "Р“РѕС‚РѕРІРѕ"
                },
                launcher = BuildLauncherSnapshot()
            });
        }
    }

    public Task ExecuteCommandAsync(string command, JsonElement payload)
    {
        lock (_sync)
        {
            ApplyCommand(command, payload);
        }

        return Task.CompletedTask;
    }

    public void Dispose()
    {
    }

    private object BuildLauncherSnapshot()
    {
        return new
        {
            activeSection = _activeSection,
            activeSettingsTab = _activeSettingsTab,
            isBusy = false,
            isGameRunning = false,
            canAccessFriends = false,
            notificationsCount = 0,
            theme = BuildThemeSnapshot(),
            main = BuildMainSnapshot(),
            account = BuildAccountSnapshot(),
            settings = BuildSettingsSnapshot(),
            skin = BuildSkinSnapshot(),
            background = BuildBackgroundSnapshot(),
            mods = BuildModsSnapshot(),
            friends = BuildFriendsSnapshot()
        };
    }

    private object BuildThemeSnapshot()
    {
        var assetsRoot = ResolveAssetsRoot();
        return new
        {
            title = "Vesper Launcher",
            iconUrl = ToLauncherFileUrl(FindFirstExisting(assetsRoot, "vesper-app.ico")),
            logoUrl = ToLauncherFileUrl(FindFirstExisting(assetsRoot, "vesper-logo.png", "V.png", "vesper-app.png")),
            wordmarkUrl = ToLauncherFileUrl(FindFirstExisting(assetsRoot, "vesper-launcher-wordmark.png", "vesper-logo.png")),
            backgroundUrl = ToLauncherFileUrl(FindFirstExisting(assetsRoot, "фон дня.png", "vesper-menu-art.jpg", "background.png")),
            glassTone = "light"
        };
    }

    private object BuildMainSnapshot()
    {
        var profilePath = Path.Combine(_platform.Paths.GetLauncherDataDirectory(), "instances", "1.21");
        var account = _accountFriends.CreateAccountSnapshot();
        var nickname = account.CurrentNickname;
        return new
        {
            nickname,
            usernameText = nickname,
            launchButtonText = "РЈСЃС‚Р°РЅРѕРІРёС‚СЊ",
            statusText = _statusText,
            progressText = _progressText,
            progressOverlayText = "Р“РѕС‚РѕРІРѕ",
            progressPercent = 0,
            isProgressIndeterminate = false,
            selectedVersionKey = _selectedVersionKey,
            selectedVersionId = "1.21",
            selectedVersionLabel = "1.21 - Р±Р°Р·РѕРІР°СЏ",
            inlineVersionLabel = "1.21 - Р±Р°Р·РѕРІР°СЏ",
            quickVersionHint = "Cross-platform shell",
            canLaunch = true,
            canOpenProfileFolder = true,
            hasLaunchIdentity = true,
            profilePath,
            savedUsernames = account.RecentUsernames,
            displayedMemoryMb = _memoryMb,
            availableVersions = new[]
            {
                new
                {
                    key = "vanilla-1.21",
                    displayName = "1.21 - Р±Р°Р·РѕРІР°СЏ",
                    baseVersionId = "1.21",
                    versionId = "1.21",
                    availabilityNote = "Р”РѕСЃС‚СѓРїРЅРѕ РїРѕСЃР»Рµ РїРµСЂРµРЅРѕСЃР° VersionManager.",
                    isSelected = _selectedVersionKey == "vanilla-1.21",
                    isInstalled = false,
                    installState = "NotInstalled",
                    actionText = "РЈСЃС‚Р°РЅРѕРІРёС‚СЊ",
                    loaders = Array.Empty<string>(),
                    subtitle = "Vanilla"
                }
            }
        };
    }

    private object BuildAccountSnapshot()
    {
        return _accountFriends.CreateAccountSnapshot();
    }

    private object BuildSettingsSnapshot()
    {
        return new
        {
            activeTab = _activeSettingsTab,
            tabs = new[]
            {
                new { id = "launcher", label = "Р›Р°СѓРЅС‡РµСЂ" },
                new { id = "java", label = "Java" },
                new { id = "vesper", label = "Vesper" },
                new { id = "launch", label = "Р—Р°РїСѓСЃРє" },
                new { id = "language", label = "РЇР·С‹Рє" },
                new { id = "glass", label = "РЎС‚РµРєР»Рѕ" }
            },
            useSystemJava = _useSystemJava,
            javaPath = _javaPath,
            effectiveJavaPath = _useSystemJava ? "java" : _javaPath,
            memoryMb = _memoryMb,
            displayedMemoryMb = _memoryMb,
            minimumMemoryMb = 1024,
            maximumMemoryMb = 12288,
            showJvmArgs = _showJvmArgs,
            extraJvmArgs = _extraJvmArgs,
            autoOptimizeMemory = _autoMemory,
            autoMinimizeOnLaunch = false,
            restoreLauncherAfterGameExit = true,
            clickSoundEnabled = false,
            minecraftLanguageCode = "auto",
            loginFormPlacementId = "center",
            launcherDirectoryViewId = "current",
            javaRuntimeMode = _useSystemJava ? "system" : "custom",
            javaModeHint = _useSystemJava ? "Р‘СѓРґРµС‚ РёСЃРїРѕР»СЊР·РѕРІР°РЅР° Java РёР· PATH." : "Р‘СѓРґРµС‚ РёСЃРїРѕР»СЊР·РѕРІР°РЅ СѓРєР°Р·Р°РЅРЅС‹Р№ РїСѓС‚СЊ.",
            jvmArgsHint = _showJvmArgs ? "РџРѕР»Рµ РІРєР»СЋС‡РµРЅРѕ." : "РџРѕР»Рµ РІС‹РєР»СЋС‡РµРЅРѕ.",
            autoMemoryHint = _autoMemory ? "РџР°РјСЏС‚СЊ РїРѕРґР±РёСЂР°РµС‚СЃСЏ Р°РІС‚РѕРјР°С‚РёС‡РµСЃРєРё." : "Р СѓС‡РЅР°СЏ РїР°РјСЏС‚СЊ РІРєР»СЋС‡РµРЅР°.",
            autoMinimizeHint = "Р”РѕСЃС‚СѓРїРЅРѕ РїРѕСЃР»Рµ РїРµСЂРµРЅРѕСЃР° MinecraftLauncher.",
            restoreHint = "Р”РѕСЃС‚СѓРїРЅРѕ РїРѕСЃР»Рµ РїРµСЂРµРЅРѕСЃР° ProcessMonitor.",
            displayedGameDirectory = _platform.Paths.MinecraftDirectory,
            languageOptions = new[]
            {
                new { id = "auto", label = "РђРІС‚Рѕ" },
                new { id = "ru_ru", label = "Р СѓСЃСЃРєРёР№" },
                new { id = "en_us", label = "English" }
            },
            loginPlacementOptions = new[]
            {
                new { id = "center", label = "Р¦РµРЅС‚СЂ" },
                new { id = "left", label = "РЎР»РµРІР°" }
            },
            directoryViewOptions = new[]
            {
                new { id = "current", label = "РўРµРєСѓС‰Р°СЏ" },
                new { id = "minecraft", label = ".minecraft" }
            },
            javaRuntimeOptions = new[]
            {
                new { id = "system", label = "РЎРёСЃС‚РµРјРЅР°СЏ Java" },
                new { id = "custom", label = "РЎРІРѕР№ РїСѓС‚СЊ" }
            },
            memoryPresets = new[] { 4096, 6144, 8192 }
        };
    }

    private object BuildSkinSnapshot()
    {
        return new
        {
            selectedSkinFileName = string.Empty,
            selectedSkinUrl = string.Empty,
            selectedSkinPreviewUrl = string.Empty,
            selectedSkinLabel = "РЎРєРёРЅ РЅРµ РІС‹Р±СЂР°РЅ.",
            selectedSkinIsSlim = false,
            modelPreferenceId = "auto",
            skinsDirectory = _platform.Paths.GetSkinsCacheDirectory(),
            availableSkins = Array.Empty<object>(),
            modelOptions = new[]
            {
                new { id = "auto", label = "РђРІС‚Рѕ", isSelected = true },
                new { id = "classic", label = "Classic", isSelected = false },
                new { id = "slim", label = "Slim", isSelected = false }
            }
        };
    }

    private object BuildBackgroundSnapshot()
    {
        return new
        {
            currentPresetId = "default",
            currentPresetLabel = "РЎС‚Р°РЅРґР°СЂС‚РЅС‹Р№",
            appliedBackgroundUrl = string.Empty,
            backgroundsDirectory = Path.Combine(ResolveAssetsRoot(), "Backgrounds"),
            items = Array.Empty<object>()
        };
    }

    private object BuildModsSnapshot()
    {
        return new
        {
            summary = "РљР°С‚Р°Р»РѕРі РјРѕРґРѕРІ Р±СѓРґРµС‚ РїРµСЂРµРЅРµСЃС‘РЅ РїРѕСЃР»Рµ MinecraftLauncher.",
            catalogSummary = "Cross-platform shell Р°РєС‚РёРІРµРЅ.",
            targetFolderHint = Path.Combine(_platform.Paths.MinecraftDirectory, "mods"),
            searchQuery = _modsSearch,
            selectedCategory = _modsCategory,
            categories = new[] { "РњРѕРґС‹", "РЁРµР№РґРµСЂС‹", "Р РµСЃСѓСЂСЃРїР°РєРё", "РЎР±РѕСЂРєРё" },
            isRefreshing = false,
            isCatalogLoading = false,
            canInstallSelected = false,
            installedModsCount = 0,
            selectedProjectIds = Array.Empty<string>(),
            modsDirectory = Path.Combine(_platform.Paths.MinecraftDirectory, "mods"),
            items = Array.Empty<object>()
        };
    }

    private object BuildFriendsSnapshot()
    {
        return _accountFriends.CreateFriendsSnapshot();
    }

    private void ApplyCommand(string command, JsonElement payload)
    {
        switch ((command ?? string.Empty).Trim().ToLowerInvariant())
        {
            case "":
            case "host.closesplash":
            case "host.syncbounds":
            case "bridge.requestsnapshot":
                return;
            case "shell.opensection":
                _activeSection = GetString(payload, "section", "none");
                var tabId = GetString(payload, "tabId", string.Empty);
                if (_activeSection == "settings" && !string.IsNullOrWhiteSpace(tabId))
                {
                    _activeSettingsTab = tabId;
                }
                return;
            case "shell.closesection":
                _activeSection = "none";
                return;
            case "settings.selecttab":
                _activeSection = "settings";
                _activeSettingsTab = GetString(payload, "tabId", _activeSettingsTab);
                return;
            case "settings.setmemory":
                _memoryMb = Math.Clamp(GetInt(payload, "value", _memoryMb), 1024, 12288);
                _autoMemory = false;
                _statusText = $"РџР°РјСЏС‚СЊ: {_memoryMb} MB.";
                return;
            case "settings.settoggle":
                ApplyToggle(GetString(payload, "field", string.Empty), GetBool(payload, "value", false));
                return;
            case "settings.settext":
                ApplyText(GetString(payload, "field", string.Empty), GetString(payload, "value", string.Empty));
                return;
            case "settings.setoption":
                ApplyOption(GetString(payload, "field", string.Empty), GetString(payload, "value", string.Empty));
                return;
            case "main.selectversionkey":
                _selectedVersionKey = GetString(payload, "key", _selectedVersionKey);
                return;
            case "main.openprofilefolder":
            case "settings.opengamedirectory":
                _ = _platform.Processes.OpenFolderAsync(_platform.Paths.MinecraftDirectory);
                return;
            case "account.setmode":
                _accountFriends.SetAccountMode(GetString(payload, "mode", "login"));
                _activeSection = "account";
                return;
            case "account.submit":
                var submitResult = _accountFriends.SubmitAccount(
                    GetString(payload, "mode", "login"),
                    GetString(payload, "username", _accountFriends.CurrentNickname),
                    GetString(payload, "password", string.Empty));
                _statusText = submitResult.Message;
                return;
            case "account.selectrecentusername":
                _accountFriends.SelectRecentUsername(GetString(payload, "username", string.Empty));
                return;
            case "friends.setnickname":
                _accountFriends.SetFriendNickname(GetString(payload, "value", string.Empty));
                return;
            case "mods.setsearch":
                _modsSearch = GetString(payload, "value", string.Empty);
                return;
            case "mods.selectcategory":
                _modsCategory = GetString(payload, "category", _modsCategory);
                return;
            case "main.launch":
                _statusText = "Р—Р°РїСѓСЃРє Minecraft РµС‰С‘ РїРµСЂРµРЅРѕСЃРёС‚СЃСЏ РІ cross-platform backend.";
                _progressText = "РЎР»РµРґСѓСЋС‰РёР№ СЌС‚Р°Рї: JavaDetector, VersionManager Рё MinecraftLauncher Р±РµР· WPF.";
                return;
            default:
                _statusText = "РљРѕРјР°РЅРґР° РїРѕРєР° РЅРµ РїРµСЂРµРЅРµСЃРµРЅР°: " + command;
                return;
        }
    }

    private void ApplyToggle(string field, bool value)
    {
        switch (field.ToLowerInvariant())
        {
            case "usesystemjava":
                _useSystemJava = value;
                return;
            case "showjvmargs":
                _showJvmArgs = value;
                return;
            case "autooptimizememory":
                _autoMemory = value;
                return;
        }
    }

    private void ApplyText(string field, string value)
    {
        switch (field.ToLowerInvariant())
        {
            case "javapath":
                _javaPath = value;
                _useSystemJava = string.IsNullOrWhiteSpace(value);
                return;
            case "extrajvmargs":
                _extraJvmArgs = value;
                return;
        }
    }

    private void ApplyOption(string field, string value)
    {
        if (field.Equals("javaruntimemode", StringComparison.OrdinalIgnoreCase))
        {
            _useSystemJava = !value.Equals("custom", StringComparison.OrdinalIgnoreCase);
        }
    }

    private static string ResolveAssetsRoot()
    {
        var candidates = new[]
        {
            Path.Combine(AppContext.BaseDirectory, "Assets"),
            Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "Assets"))
        };

        return candidates.FirstOrDefault(Directory.Exists) ?? AppContext.BaseDirectory;
    }

    private static string? FindFirstExisting(string directory, params string[] fileNames)
    {
        if (string.IsNullOrWhiteSpace(directory) || !Directory.Exists(directory))
        {
            return null;
        }

        return fileNames
            .Select(fileName => Path.Combine(directory, fileName))
            .FirstOrDefault(File.Exists);
    }

    private static string ToLauncherFileUrl(string? path)
    {
        return string.IsNullOrWhiteSpace(path) ? string.Empty : LocalStaticFileServer.BuildLauncherFileUrl(path);
    }

    private static string BuildAvatarPlaceholder(string value)
    {
        var text = string.IsNullOrWhiteSpace(value) ? "AV" : value.Trim();
        return text.Length <= 2 ? text.ToUpperInvariant() : text[..2].ToUpperInvariant();
    }

    private static string GetString(JsonElement payload, string propertyName, string fallback)
    {
        return payload.ValueKind == JsonValueKind.Object &&
               payload.TryGetProperty(propertyName, out var value) &&
               value.ValueKind == JsonValueKind.String
            ? value.GetString() ?? fallback
            : fallback;
    }

    private static int GetInt(JsonElement payload, string propertyName, int fallback)
    {
        return payload.ValueKind == JsonValueKind.Object &&
               payload.TryGetProperty(propertyName, out var value) &&
               value.TryGetInt32(out var result)
            ? result
            : fallback;
    }

    private static bool GetBool(JsonElement payload, string propertyName, bool fallback)
    {
        return payload.ValueKind == JsonValueKind.Object &&
               payload.TryGetProperty(propertyName, out var value) &&
               (value.ValueKind == JsonValueKind.True || value.ValueKind == JsonValueKind.False)
            ? value.GetBoolean()
            : fallback;
    }
}


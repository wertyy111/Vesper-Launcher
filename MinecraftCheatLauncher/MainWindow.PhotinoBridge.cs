using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Runtime.InteropServices;
using System.Text.Json;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Controls.Primitives;
using System.Windows.Interop;
using System.Windows.Media.Imaging;
using MinecraftCheatLauncher.Launcher;
using MinecraftCheatLauncher.PhotinoHost;

namespace MinecraftCheatLauncher;

public partial class MainWindow
{
    private static readonly IReadOnlyList<string> PhotinoModsCategories =
    [
        AllModsCategoryFilter,
        "Моды",
        "Шейдеры",
        "Ресурспаки",
        "Сборки"
    ];

    private static readonly IntPtr HwndBottom = new(1);
    private const double HiddenHostLeft = -32000;
    private const double HiddenHostTop = -32000;
    private const uint SwpNoSize = 0x0001;
    private const uint SwpNoMove = 0x0002;
    private const uint SwpNoActivate = 0x0010;
    private const uint SwpNoOwnerZOrder = 0x0200;
    private bool _isApplyingPhotinoHostHiddenState;
    private string? _photinoSkinPreviewCacheKey;
    private string? _photinoSkinPreviewDataUrl;

    public void PrepareForPhotinoHost()
    {
        ShowInTaskbar = false;
        ShowActivated = false;
        WindowStartupLocation = WindowStartupLocation.Manual;
        Width = 1100;
        Height = 720;
        MinWidth = 960;
        MinHeight = 620;
        Left = HiddenHostLeft;
        Top = HiddenHostTop;
        Opacity = 0;
        IsHitTestVisible = false;
        Loaded += (_, _) => HidePhotinoHostWindow();
        SourceInitialized += (_, _) => HidePhotinoHostWindow();
    }

    public void HidePhotinoHostWindow()
    {
        if (_isApplyingPhotinoHostHiddenState)
        {
            return;
        }

        try
        {
            _isApplyingPhotinoHostHiddenState = true;
            ShowInTaskbar = false;
            ShowActivated = false;
            IsHitTestVisible = false;
            Opacity = 0;

            if (WindowState != WindowState.Normal)
            {
                WindowState = WindowState.Normal;
            }

            Left = HiddenHostLeft;
            Top = HiddenHostTop;
            SendHostWindowBehindPhotino();
        }
        finally
        {
            _isApplyingPhotinoHostHiddenState = false;
        }
    }

    public void SyncPhotinoHostBounds(JsonElement payload)
    {
        var left = TryGetDouble(payload, "left");
        var top = TryGetDouble(payload, "top");
        var width = TryGetDouble(payload, "width");
        var height = TryGetDouble(payload, "height");
        var maximized = TryGetBool(payload, "maximized");

        _ = left;
        _ = top;
        _ = maximized;

        if (width is > 0)
        {
            Width = Math.Max(MinWidth, width.Value);
        }

        if (height is > 0)
        {
            Height = Math.Max(MinHeight, height.Value);
        }

        HidePhotinoHostWindow();
    }

    public object CreatePhotinoSnapshot()
    {
        var currentNickname = GetActiveNickname() ?? UsernameTextBox.Text?.Trim() ?? string.Empty;
        return new
        {
            activeSection = _activeSidePanelSection.ToString().ToLowerInvariant(),
            activeSettingsTab = _activeLauncherSettingsTabId,
            isBusy = _isBusy,
            isGameRunning = _gameProcess is not null && !_gameProcess.HasExited,
            canAccessFriends = CanAccessFriendsFeature(),
            notificationsCount = _incomingFriendRequests.Count,
            theme = BuildPhotinoThemeSnapshot(),
            main = BuildPhotinoMainSnapshot(currentNickname),
            account = BuildPhotinoAccountSnapshot(currentNickname),
            settings = BuildPhotinoSettingsSnapshot(),
            skin = BuildPhotinoSkinSnapshot(),
            background = BuildPhotinoBackgroundSnapshot(),
            mods = BuildPhotinoModsSnapshot(),
            friends = BuildPhotinoFriendsSnapshot(currentNickname)
        };
    }

    public Task ExecutePhotinoCommandAsync(string command, JsonElement payload)
    {
        return ExecutePhotinoCommandCoreAsync(command, payload);
    }

    private Task ExecutePhotinoCommandCoreAsync(string command, JsonElement payload)
    {
        var normalizedCommand = command?.Trim() ?? string.Empty;
        switch (normalizedCommand.ToLowerInvariant())
        {
            case "":
            case "bridge.requestsnapshot":
                return Task.CompletedTask;

            case "shell.opensection":
                ShowSidePanelSection(ParsePhotinoSidePanelSection(TryGetString(payload, "section")));
                var requestedTab = TryGetString(payload, "tabId");
                if (_activeSidePanelSection == SidePanelSection.Settings && !string.IsNullOrWhiteSpace(requestedTab))
                {
                    ShowLauncherSettingsTab(requestedTab);
                }
                return Task.CompletedTask;

            case "shell.closesection":
                HideSidePanel();
                return Task.CompletedTask;

            case "main.launch":
                LaunchButton_OnClick(LaunchButton, new RoutedEventArgs());
                return Task.CompletedTask;

            case "main.selectversionkey":
                SelectVersionChoiceByKey(TryGetString(payload, "key") ?? string.Empty);
                return Task.CompletedTask;

            case "main.selectversionid":
                SelectVersionByVersionId(TryGetString(payload, "versionId") ?? string.Empty);
                return Task.CompletedTask;

            case "main.selectsavedusername":
                ApplySavedUsernameSelection(TryGetString(payload, "username") ?? string.Empty);
                return Task.CompletedTask;

            case "main.savecurrentusername":
                if (TrySaveCurrentUsername())
                {
                    RefreshSavedUsernamesList(UsernameTextBox.Text.Trim());
                }
                return Task.CompletedTask;

            case "main.removesavedusername":
                RemoveSavedUsername(TryGetString(payload, "username"));
                return Task.CompletedTask;

            case "main.openprofilefolder":
                OpenProfileFolderButton_OnClick(OpenProfileFolderButton, new RoutedEventArgs());
                return Task.CompletedTask;

            case "account.setmode":
                ApplyPhotinoAccountMode(TryGetString(payload, "mode"));
                return Task.CompletedTask;

            case "account.submit":
                return SubmitPhotinoAccountAsync(payload);

            case "account.logout":
                return LogoutPhotinoAccountAsync();

            case "account.pickavatar":
                ChangeAvatarButton_OnClick(ChangeAvatarButton, new RoutedEventArgs());
                return Task.CompletedTask;

            case "account.selectrecentusername":
                ApplySavedUsernameSelection(TryGetString(payload, "username") ?? string.Empty);
                return Task.CompletedTask;

            case "settings.selecttab":
                ShowLauncherSettingsTab(TryGetString(payload, "tabId"));
                return Task.CompletedTask;

            case "settings.settoggle":
                ApplyPhotinoSettingsToggle(TryGetString(payload, "field"), TryGetBool(payload, "value") == true);
                return Task.CompletedTask;

            case "settings.settext":
                ApplyPhotinoSettingsText(TryGetString(payload, "field"), TryGetString(payload, "value"));
                return Task.CompletedTask;

            case "settings.setmemory":
                if (TryGetInt(payload, "value") is int memoryMb)
                {
                    MemorySlider.Value = NormalizeMemoryMb(memoryMb);
                    SetStatus($"Память: {ResolveDisplayedMemoryMb()} MB.");
                }
                return Task.CompletedTask;

            case "settings.setoption":
                ApplyPhotinoSettingsOption(TryGetString(payload, "field"), TryGetString(payload, "value"));
                return Task.CompletedTask;

            case "settings.opengamedirectory":
                LauncherOpenGameDirectoryButton_OnClick(LauncherOpenGameDirectoryButton, new RoutedEventArgs());
                return Task.CompletedTask;

            case "skin.selectfile":
                ApplyPhotinoSkinSelection(TryGetString(payload, "fileName"));
                return Task.CompletedTask;

            case "skin.setmodel":
                ApplyPhotinoSkinModel(TryGetString(payload, "modelId"));
                return Task.CompletedTask;

            case "skin.refresh":
                RefreshSkinFiles(_uiState.SelectedSkinFileName);
                return Task.CompletedTask;

            case "skin.clear":
                ClearSkinSelectionButton_OnClick(ClearSkinSelectionButton, new RoutedEventArgs());
                return Task.CompletedTask;

            case "skin.importdialog":
                ImportSkinButton_OnClick(ImportSkinButton, new RoutedEventArgs());
                return Task.CompletedTask;

            case "skin.openfolder":
                OpenSkinsFolderButton_OnClick(OpenSkinsFolderButton, new RoutedEventArgs());
                return Task.CompletedTask;

            case "background.reset":
                ApplyDefaultBackgroundButton_OnClick(ApplyDefaultBackgroundButton, new RoutedEventArgs());
                return Task.CompletedTask;

            case "background.openfolder":
                OpenBackgroundsFolderButton_OnClick(OpenBackgroundsFolderButton, new RoutedEventArgs());
                return Task.CompletedTask;

            case "mods.selectcategory":
                return SelectPhotinoModsCategoryAsync(TryGetString(payload, "category"));

            case "mods.setsearch":
                ModsSearchTextBox.Text = TryGetString(payload, "value") ?? string.Empty;
                ApplyRecommendedModsFilter();
                UpdateSelectedModsState();
                return Task.CompletedTask;

            case "mods.setselectedprojects":
                ApplyPhotinoModSelection(TryGetStringArray(payload, "projectIds"));
                return Task.CompletedTask;

            case "mods.refreshcatalog":
                return RefreshRecommendedModCatalogAsync(forceRefresh: true);

            case "mods.installselected":
                InstallSelectedModsButton_OnClick(InstallSelectedModsButton, new RoutedEventArgs());
                return Task.CompletedTask;

            case "mods.clearselection":
                ClearSelectedModsButton_OnClick(ClearSelectedModsButton, new RoutedEventArgs());
                return Task.CompletedTask;

            case "mods.togglefavorite":
                TogglePhotinoFavoriteMod(TryGetString(payload, "projectId"));
                return Task.CompletedTask;

            case "mods.toggleitem":
                TriggerPhotinoModPrimaryAction(TryGetString(payload, "projectId"));
                return Task.CompletedTask;

            case "mods.openfolder":
                OpenModsFolderButton_OnClick(OpenModsFolderButton, new RoutedEventArgs());
                return Task.CompletedTask;

            case "friends.setnickname":
                FriendNicknameTextBox.Text = TryGetString(payload, "value") ?? string.Empty;
                return Task.CompletedTask;

            case "friends.refresh":
                RefreshFriendsSection();
                return Task.CompletedTask;

            case "friends.add":
                return AddFriendButton_OnClickAsync();

            case "friends.remove":
                return RemoveFriendByUsernameAsync(TryGetString(payload, "username"));

            case "friends.respond":
                return RespondPhotinoFriendRequestAsync(payload);

            case "friends.connect":
                TriggerPhotinoFriendConnect(TryGetString(payload, "username"));
                return Task.CompletedTask;

            default:
                throw new InvalidOperationException($"Неизвестная команда Photino: {command}");
        }
    }

    private async Task SubmitPhotinoAccountAsync(JsonElement payload)
    {
        var mode = ParsePhotinoAccountEntryMode(TryGetString(payload, "mode"));
        var username = TryGetString(payload, "username");
        var password = TryGetString(payload, "password");

        ApplyPhotinoAccountMode(mode.ToString().ToLowerInvariant());
        if (!string.IsNullOrWhiteSpace(username))
        {
            AccountNicknameTextBox.Text = username;
        }

        if (password is not null)
        {
            AccountPasswordBox.Password = password;
        }

        await SubmitAccountActionForModeAsync(mode, username, password);
    }

    private async Task LogoutPhotinoAccountAsync()
    {
        if (HasIncognitoIdentity() && !HasAuthenticatedCloudSession())
        {
            ClearGuestIdentityState();
            UsernameTextBox.Text = HasRegisteredAccount()
                ? _accountState!.Username
                : _savedUsernames.FirstOrDefault() ?? string.Empty;
            ShowSidePanelSection(SidePanelSection.Account);
            ApplyAccountUiState();
            SetStatus("Инкогнито отключено.");
            return;
        }

        if (!HasAuthenticatedCloudSession())
        {
            return;
        }

        SetBusy(true, "Выход из аккаунта...");
        await TryLogoutCloudSessionAsync(_accountState);
        ResetAccountSessionState();
        ShowSidePanelSection(SidePanelSection.Account);
        SetBusy(false);
        SetStatus("Вы вышли из аккаунта. Можно войти снова.");
    }
    private void ApplyPhotinoAccountMode(string? requestedMode)
    {
        ShowSidePanelSection(SidePanelSection.Account);
        var normalizedMode = requestedMode?.Trim().ToLowerInvariant();
        if (string.Equals(normalizedMode, "guest", StringComparison.Ordinal))
        {
            _accountEntryMode = AccountEntryMode.Guest;
            _isEditingIncognitoNickname = true;
            AccountPasswordBox.Password = string.Empty;
            RefreshAccountSection();
            FocusAccountNicknameEditorIfNeeded();
            return;
        }

        SetAccountEntryMode(ParsePhotinoAccountEntryMode(normalizedMode));
        FocusAccountNicknameEditorIfNeeded();
    }

    private void RemoveSavedUsername(string? username)
    {
        if (string.IsNullOrWhiteSpace(username))
        {
            return;
        }

        _savedUsernames.RemoveAll(existing =>
            string.Equals(existing, username, StringComparison.OrdinalIgnoreCase));
        SaveSavedUsernamesToDisk();
        RefreshSavedUsernamesList();
        SetStatus($"Ник удален: {username}");
    }

    private void ApplyPhotinoSettingsToggle(string? field, bool value)
    {
        switch (field?.Trim().ToLowerInvariant())
        {
            case "usesystemjava":
                UseSystemJavaToggleButton.IsChecked = value;
                LauncherSettingsToggleButton_OnClick(UseSystemJavaToggleButton, new RoutedEventArgs());
                break;
            case "showjvmargs":
                ShowJvmArgsToggleButton.IsChecked = value;
                LauncherSettingsToggleButton_OnClick(ShowJvmArgsToggleButton, new RoutedEventArgs());
                break;
            case "autooptimizememory":
                AutoOptimizeMemoryToggleButton.IsChecked = value;
                LauncherSettingsToggleButton_OnClick(AutoOptimizeMemoryToggleButton, new RoutedEventArgs());
                break;
            case "autominimizeonlaunch":
                AutoMinimizeLauncherToggleButton.IsChecked = value;
                LauncherSettingsToggleButton_OnClick(AutoMinimizeLauncherToggleButton, new RoutedEventArgs());
                break;
            case "restorelauncheraftergameexit":
                RestoreLauncherAfterGameExitToggleButton.IsChecked = value;
                LauncherSettingsToggleButton_OnClick(RestoreLauncherAfterGameExitToggleButton, new RoutedEventArgs());
                break;
            case "clicksoundenabled":
                LauncherClickSoundToggleButton.IsChecked = value;
                LauncherSettingsToggleButton_OnClick(LauncherClickSoundToggleButton, new RoutedEventArgs());
                break;
        }
    }

    private void ApplyPhotinoSettingsText(string? field, string? value)
    {
        switch (field?.Trim().ToLowerInvariant())
        {
            case "javapath":
                JavaPathTextBox.Text = value ?? string.Empty;
                break;
            case "extrajvmargs":
                ExtraJvmArgsTextBox.Text = value ?? string.Empty;
                break;
            default:
                return;
        }

        ApplyLauncherSettingsUiState();
        PersistLauncherSettingsFromControls();
    }

    private void ApplyPhotinoSettingsOption(string? field, string? value)
    {
        switch (field?.Trim().ToLowerInvariant())
        {
            case "minecraftlanguagecode":
                MinecraftLanguageComboBox.SelectedItem = ResolveMinecraftLanguageOption(value);
                break;
            case "loginformplacementid":
                LauncherLoginFormPlacementComboBox.SelectedItem = ResolveLoginFormPlacementOption(value);
                break;
            case "launcherdirectoryviewid":
                LauncherDirectoryViewModeComboBox.SelectedItem = ResolveLauncherDirectoryViewOption(value);
                break;
            case "javaruntimemode":
                var useSystemJava = !string.Equals(value, "custom", StringComparison.OrdinalIgnoreCase);
                LauncherJavaRuntimeComboBox.SelectedItem = ResolveLauncherJavaRuntimeOption(useSystemJava);
                break;
            default:
                return;
        }

        ApplyLauncherSettingsUiState();
        PersistLauncherSettingsFromControls();
    }

    private void ApplyPhotinoSkinSelection(string? fileName)
    {
        if (string.IsNullOrWhiteSpace(fileName))
        {
            SkinFilesComboBox.SelectedItem = null;
            return;
        }

        var matchedEntry = _availableSkinFiles.FirstOrDefault(entry =>
            string.Equals(entry.FileName, fileName, StringComparison.OrdinalIgnoreCase));
        if (matchedEntry is not null)
        {
            SkinFilesComboBox.SelectedItem = matchedEntry;
            return;
        }

        RefreshSkinFiles(fileName);
    }

    private void ApplyPhotinoSkinModel(string? modelId)
    {
        var normalizedModelId = NormalizeSkinModelPreferenceId(modelId);
        var selectedOption = SkinModelOptions.FirstOrDefault(option =>
            string.Equals(option.Id, normalizedModelId, StringComparison.OrdinalIgnoreCase));
        if (selectedOption is not null)
        {
            SkinModelComboBox.SelectedItem = selectedOption;
        }
    }

    private Task SelectPhotinoModsCategoryAsync(string? category)
    {
        var normalizedCategory = PhotinoModsCategories.FirstOrDefault(option =>
            string.Equals(option, category, StringComparison.OrdinalIgnoreCase)) ?? AllModsCategoryFilter;
        _selectedModsCategoryFilter = normalizedCategory;
        UpdateRecommendedCatalogTargetFolderHint();
        return EnsureRecommendedModCatalogAsync();
    }

    private void ApplyPhotinoModSelection(IReadOnlyList<string> selectedProjectIds)
    {
        var selectedIds = new HashSet<string>(selectedProjectIds ?? Array.Empty<string>(), StringComparer.OrdinalIgnoreCase);
        RecommendedModsListBox.SelectedItems.Clear();
        foreach (var item in RecommendedModsListBox.Items.OfType<RecommendedModCatalogItem>())
        {
            if (selectedIds.Contains(item.ProjectId))
            {
                RecommendedModsListBox.SelectedItems.Add(item);
            }
        }

        UpdateSelectedModsState();
    }

    private void TogglePhotinoFavoriteMod(string? projectId)
    {
        var item = FindPhotinoCatalogItem(projectId);
        if (item is null)
        {
            return;
        }

        ToggleFavoriteModButton_OnClick(new Button { Tag = item }, new RoutedEventArgs());
    }

    private void TriggerPhotinoModPrimaryAction(string? projectId)
    {
        var item = FindPhotinoCatalogItem(projectId);
        if (item is null)
        {
            return;
        }

        InstallCatalogModButton_OnClick(new Button { Tag = item }, new RoutedEventArgs());
    }

    private async Task RespondPhotinoFriendRequestAsync(JsonElement payload)
    {
        var requestId = TryGetLong(payload, "requestId");
        var username = TryGetString(payload, "username");
        var action = TryGetString(payload, "action") ?? "accept";
        var request = _incomingFriendRequests.FirstOrDefault(entry =>
            (requestId.HasValue && entry.RequestId == requestId.Value) ||
            (!string.IsNullOrWhiteSpace(username) && string.Equals(entry.Username, username, StringComparison.OrdinalIgnoreCase)));
        if (request is null)
        {
            return;
        }

        await RespondToFriendRequestAsync(request, action);
    }

    private void TriggerPhotinoFriendConnect(string? username)
    {
        if (string.IsNullOrWhiteSpace(username))
        {
            return;
        }

        var friend = _friendEntries.FirstOrDefault(entry =>
            string.Equals(entry.Username, username, StringComparison.OrdinalIgnoreCase));
        if (friend is null)
        {
            return;
        }

        ConnectToFriendButton_OnClick(new Button { Tag = friend }, new RoutedEventArgs());
    }

    private RecommendedModCatalogItem? FindPhotinoCatalogItem(string? projectId)
    {
        if (string.IsNullOrWhiteSpace(projectId))
        {
            return null;
        }

        return _filteredRecommendedModCatalog.FirstOrDefault(item =>
                   string.Equals(item.ProjectId, projectId, StringComparison.OrdinalIgnoreCase))
               ?? _recommendedModCatalog.FirstOrDefault(item =>
                   string.Equals(item.ProjectId, projectId, StringComparison.OrdinalIgnoreCase));
    }

    private object BuildPhotinoThemeSnapshot()
    {
        var assetDirectories = new[]
        {
            Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "Assets")),
            Path.Combine(AppContext.BaseDirectory, "Assets")
        }
        .Where(Directory.Exists)
        .Distinct(StringComparer.OrdinalIgnoreCase)
        .ToArray();

        var logoPath = ResolveBestLogoPath(assetDirectories);
        var wordmarkPath = ResolveBestWordmarkPath(assetDirectories) ?? logoPath;
        var iconPath = assetDirectories
            .Select(directory => Path.Combine(directory, "vesper-app.ico"))
            .FirstOrDefault(File.Exists);

        return new
        {
            title = Title,
            iconUrl = ToLauncherFileUrl(iconPath),
            logoUrl = ToLauncherFileUrl(logoPath),
            wordmarkUrl = ToLauncherFileUrl(wordmarkPath),
            backgroundUrl = ToLauncherFileUrl(_appliedBackgroundImagePath),
            glassTone = (_appliedGlassThemeTone ?? GlassThemeTone.Light).ToString().ToLowerInvariant()
        };
    }

    private object BuildPhotinoMainSnapshot(string currentNickname)
    {
        var selectedChoice = _selectedVersionChoice;
        return new
        {
            nickname = currentNickname,
            usernameText = UsernameTextBox.Text?.Trim() ?? string.Empty,
            launchButtonText = LaunchButtonText.Text,
            statusText = StatusTextBlock.Text,
            progressText = ProgressLabelTextBlock.Text,
            progressOverlayText = DownloadProgressOverlayTextBlock?.Text ?? string.Empty,
            progressPercent = DownloadProgressBar.IsIndeterminate ? (double?)null : DownloadProgressBar.Value,
            isProgressIndeterminate = DownloadProgressBar.IsIndeterminate,
            selectedVersionKey = selectedChoice?.Key,
            selectedVersionId = _selectedVersion?.Id,
            selectedVersionLabel = selectedChoice?.DisplayName,
            inlineVersionLabel = InlineVersionLabel.Text,
            quickVersionHint = QuickVersionHintTextBlock.Text,
            canLaunch = !_isBusy && selectedChoice is not null,
            canOpenProfileFolder = true,
            hasLaunchIdentity = HasLaunchIdentity(),
            profilePath = ResolveCurrentProfileDirectory(),
            savedUsernames = _savedUsernames.ToArray(),
            displayedMemoryMb = ResolveDisplayedMemoryMb(),
            availableVersions = _availableVersionChoices.Select(choice => new
            {
                key = choice.Key,
                displayName = choice.DisplayName,
                baseVersionId = choice.BaseVersionId,
                versionId = choice.Version?.Id,
                availabilityNote = choice.AvailabilityNote,
                isSelected = string.Equals(choice.Key, selectedChoice?.Key, StringComparison.Ordinal),
                isInstalled = choice.Version is not null,
                loaders = GetSelectionEntryLoaders(choice).Select(GetLoaderDisplayName).ToArray(),
                subtitle = choice.Version is not null
                    ? choice.Version.Id
                    : $"{choice.BaseVersionId} + {BuildLoaderCombinationDisplayName(GetSelectionEntryLoaders(choice))}"
            }).ToArray()
        };
    }

    private object BuildPhotinoAccountSnapshot(string currentNickname)
    {
        var avatarPath = ResolveAccountAvatarPath(HasAuthenticatedCloudSession() ? _accountState : null);
        var avatarName = HasAuthenticatedCloudSession()
            ? _accountState?.Username
            : HasIncognitoIdentity()
                ? _guestIdentityState?.Username
                : currentNickname;

        return new
        {
            mode = ResolvePhotinoAccountMode(),
            hasAuthenticatedSession = HasAuthenticatedCloudSession(),
            hasStoredProfile = HasRegisteredAccount(),
            hasGuestIdentity = HasIncognitoIdentity(),
            isEditingGuest = _isEditingIncognitoNickname,
            accountStateText = AccountStateTextBlock.Text,
            nicknameInput = AccountNicknameTextBox.Text?.Trim() ?? string.Empty,
            currentNickname,
            avatarUrl = ToLauncherFileUrl(avatarPath),
            avatarPlaceholder = BuildAvatarPlaceholderText(avatarName),
            canLogout = LogoutAccountButton.IsEnabled,
            canChangeAvatar = ChangeAvatarButton.IsEnabled,
            canUseGuest = true,
            recentUsernames = _savedUsernames.Take(MaxAccountRecentUsernames).ToArray(),
            hasEarlyPlayersAchievement = HasEarlyPlayersAchievement()
        };
    }

    private object BuildPhotinoSettingsSnapshot()
    {
        return new
        {
            activeTab = _activeLauncherSettingsTabId,
            tabs = new[]
            {
                new { id = LauncherSettingsTabLaunch, label = "Запуск" },
                new { id = LauncherSettingsTabJava, label = "Java" },
                new { id = LauncherSettingsTabLanguage, label = "Язык" },
                new { id = LauncherSettingsTabLauncher, label = "Лаунчер" },
                new { id = LauncherSettingsTabVesper, label = "Vesper" },
                new { id = LauncherSettingsTabGlass, label = "Стекло" }
            },
            useSystemJava = ResolveUseSystemJavaPreference(),
            javaPath = ResolveStoredJavaExecutablePath(),
            effectiveJavaPath = ResolveEffectiveJavaExecutable(),
            memoryMb = ResolveStoredMemoryMb(),
            displayedMemoryMb = ResolveDisplayedMemoryMb(),
            minimumMemoryMb = MinimumLauncherMemoryMb,
            maximumMemoryMb = MaximumLauncherMemoryMb,
            showJvmArgs = ResolveShowJvmArgsPreference(),
            extraJvmArgs = ResolveStoredExtraJvmArgs(),
            autoOptimizeMemory = ResolveAutoOptimizeMemoryPreference(),
            autoMinimizeOnLaunch = IsAutoMinimizeLauncherEnabled(),
            restoreLauncherAfterGameExit = IsRestoreLauncherAfterGameExitEnabled(),
            clickSoundEnabled = IsLauncherClickSoundEnabled(),
            minecraftLanguageCode = ResolveSelectedMinecraftLanguageCode(),
            loginFormPlacementId = ResolveSelectedLoginFormPlacementId(),
            launcherDirectoryViewId = ResolveSelectedLauncherDirectoryViewId(),
            javaRuntimeMode = ResolveUseSystemJavaPreference() ? "system" : "custom",
            javaModeHint = JavaModeHintTextBlock?.Text ?? string.Empty,
            jvmArgsHint = JvmArgsHintTextBlock?.Text ?? string.Empty,
            autoMemoryHint = AutoMemoryHintTextBlock?.Text ?? string.Empty,
            autoMinimizeHint = AutoMinimizeLauncherHintTextBlock?.Text ?? string.Empty,
            restoreHint = RestoreLauncherAfterGameExitHintTextBlock?.Text ?? string.Empty,
            displayedGameDirectory = ResolveDisplayedLauncherGameDirectory(),
            languageOptions = MinecraftLanguageOptions.Select(option => new { id = option.Id, label = option.DisplayName }).ToArray(),
            loginPlacementOptions = LoginFormPlacementOptions.Select(option => new { id = option.Id, label = option.DisplayName }).ToArray(),
            directoryViewOptions = LauncherDirectoryViewOptions.Select(option => new { id = option.Id, label = option.DisplayName }).ToArray(),
            javaRuntimeOptions = LauncherJavaRuntimeOptions.Select(option => new
            {
                id = option.UseSystemJava ? "system" : "custom",
                label = option.DisplayName
            }).ToArray(),
            memoryPresets = new[] { 4096, 6144, 8192 }
        };
    }
    private object BuildPhotinoSkinSnapshot()
    {
        var skinsDirectory = Path.Combine(GetPreferredLauncherDataDirectory(), "skins");
        return new
        {
            selectedSkinFileName = _uiState.SelectedSkinFileName,
            selectedSkinUrl = ToLauncherFileUrl(_selectedSkinFilePath),
            selectedSkinPreviewUrl = BuildPhotinoSkinPreviewDataUrl(),
            selectedSkinLabel = SelectedSkinFileTextBlock.Text,
            selectedSkinIsSlim = _selectedSkinIsSlim,
            modelPreferenceId = ToSkinModelPreferenceId(_skinModelPreference),
            skinsDirectory,
            availableSkins = _availableSkinFiles.Select(entry => new
            {
                fileName = entry.FileName,
                isSelected = string.Equals(entry.FileName, _uiState.SelectedSkinFileName, StringComparison.OrdinalIgnoreCase),
                url = ToLauncherFileUrl(entry.FullPath)
            }).ToArray(),
            modelOptions = SkinModelOptions.Select(option => new
            {
                id = option.Id,
                label = option.DisplayName,
                isSelected = option.Preference == _skinModelPreference
            }).ToArray()
        };
    }

    private string? BuildPhotinoSkinPreviewDataUrl()
    {
        var selectedPath = _selectedSkinFilePath;
        var fileStamp = !string.IsNullOrWhiteSpace(selectedPath) && File.Exists(selectedPath)
            ? File.GetLastWriteTimeUtc(selectedPath).Ticks
            : 0L;
        var cacheKey = $"{selectedPath ?? string.Empty}|{fileStamp}|{ToSkinModelPreferenceId(_skinModelPreference)}";
        if (string.Equals(cacheKey, _photinoSkinPreviewCacheKey, StringComparison.Ordinal) &&
            !string.IsNullOrWhiteSpace(_photinoSkinPreviewDataUrl))
        {
            return _photinoSkinPreviewDataUrl;
        }

        try
        {
            var loadedSkinBitmap = LoadSkinBitmapForPreview(selectedPath);
            var skinBitmap = loadedSkinBitmap ?? CreateFallbackSkinBitmap();
            var sourceTextureWidth = Math.Max(1, skinBitmap.PixelWidth);
            var sourceTextureHeight = Math.Max(1, skinBitmap.PixelHeight);
            ResolveSkinScale(sourceTextureWidth, sourceTextureHeight, out var sourceScaleX, out var sourceScaleY);

            var isModernLayout = sourceTextureWidth == sourceTextureHeight;
            var autoSlimModel = loadedSkinBitmap is not null &&
                                isModernLayout &&
                                IsSlimSkinModel(skinBitmap, sourceScaleX, sourceScaleY);
            var isSlimModel = _skinModelPreference switch
            {
                SkinModelPreference.Slim => isModernLayout,
                SkinModelPreference.Classic => false,
                _ => autoSlimModel
            };

            var previewBitmap = RenderSoftwareSkinPreview(
                skinBitmap,
                isModernLayout,
                isSlimModel,
                sourceScaleX,
                sourceScaleY);

            _photinoSkinPreviewCacheKey = cacheKey;
            _photinoSkinPreviewDataUrl = ToPngDataUrl(previewBitmap);
            return _photinoSkinPreviewDataUrl;
        }
        catch (Exception ex)
        {
            _ = ex;
            _photinoSkinPreviewCacheKey = cacheKey;
            _photinoSkinPreviewDataUrl = null;
            return null;
        }
    }

    private static string ToPngDataUrl(BitmapSource bitmap)
    {
        var encoder = new PngBitmapEncoder();
        encoder.Frames.Add(BitmapFrame.Create(bitmap));

        using var stream = new MemoryStream();
        encoder.Save(stream);
        return $"data:image/png;base64,{Convert.ToBase64String(stream.ToArray())}";
    }

    private object BuildPhotinoBackgroundSnapshot()
    {
        var backgroundItems = GetBackgroundImageCandidates()
            .Select(file => new
            {
                fileName = file.Name,
                label = Path.GetFileNameWithoutExtension(file.Name),
                url = ToLauncherFileUrl(file.FullName),
                isActive = string.Equals(file.FullName, _appliedBackgroundImagePath, StringComparison.OrdinalIgnoreCase)
            })
            .ToArray();

        return new
        {
            currentPresetId = NormalizeBackgroundPresetId(_uiState.BackgroundPresetId),
            currentPresetLabel = BackgroundCurrentPresetTextBlock.Text,
            appliedBackgroundUrl = ToLauncherFileUrl(_appliedBackgroundImagePath),
            backgroundsDirectory = Path.Combine(GetAssetsDirectory(), "Backgrounds"),
            items = backgroundItems
        };
    }

    private object BuildPhotinoModsSnapshot()
    {
        var selectedProjectIds = RecommendedModsListBox.SelectedItems
            .OfType<RecommendedModCatalogItem>()
            .Select(item => item.ProjectId)
            .ToArray();
        var modsDirectory = Path.Combine(ResolveCurrentProfileDirectory(), "mods");
        Directory.CreateDirectory(modsDirectory);

        return new
        {
            summary = ModsSummaryTextBlock.Text,
            catalogSummary = ModCatalogSummaryTextBlock.Text,
            targetFolderHint = ModsTargetFolderTextBlock.Text,
            searchQuery = ModsSearchTextBox.Text ?? string.Empty,
            selectedCategory = _selectedModsCategoryFilter,
            categories = PhotinoModsCategories.ToArray(),
            isRefreshing = _isRefreshingRecommendedModCatalog,
            isCatalogLoading = _isRefreshingRecommendedModCatalog || ModsCatalogLoadingPanel.Visibility == Visibility.Visible,
            canInstallSelected = InstallSelectedModsButton.IsEnabled,
            installedModsCount = Directory.EnumerateFiles(modsDirectory, "*.jar", SearchOption.TopDirectoryOnly).Count(),
            selectedProjectIds,
            modsDirectory,
            items = _filteredRecommendedModCatalog.Select(item => new
            {
                projectId = item.ProjectId,
                displayName = item.DisplayName,
                description = item.Description,
                iconUrl = ResolvePhotinoModIconUrl(item),
                sourceIconUrl = ToLauncherFileUrl(item.SourceIconUrl),
                badgeText = item.BadgeText,
                badgeBackgroundHex = item.BadgeBackgroundHex,
                badgeForegroundHex = item.BadgeForegroundHex,
                packSummary = item.PackSummary,
                contentKind = item.ContentKind.ToString().ToLowerInvariant(),
                isFavorite = item.IsFavorite,
                isInstalled = item.IsInstalled,
                isSelected = selectedProjectIds.Contains(item.ProjectId, StringComparer.OrdinalIgnoreCase),
                actionText = item.IsInstalled ? "Удалить" : GetCatalogInstallActionText(item.ContentKind),
                installedFilePath = item.InstalledFilePath
            }).ToArray()
        };
    }

    private object BuildPhotinoFriendsSnapshot(string currentNickname)
    {
        var profileAvatarPath = ResolveAccountAvatarPath(HasAuthenticatedCloudSession() ? _accountState : null);
        var selectedRequest = IncomingFriendRequestsListBox.SelectedItem as CloudIncomingFriendRequestItem;
        return new
        {
            profileNickname = currentNickname,
            profileType = FriendsProfileTypeTextBlock.Text,
            cloudStatus = FriendsCloudStatusTextBlock.Text,
            vesperNetStatus = VesperNetStatusTextBlock.Text,
            profileAvatarUrl = ToLauncherFileUrl(profileAvatarPath),
            profileAvatarPlaceholder = BuildAvatarPlaceholderText(currentNickname),
            friendNicknameInput = FriendNicknameTextBox.Text?.Trim() ?? string.Empty,
            canManage = CanManageCloudFriends(),
            canAccess = CanAccessFriendsFeature(),
            outgoingRequestCount = _outgoingFriendRequests.Count,
            selectedRequestId = selectedRequest?.RequestId,
            friends = _friendEntries.Select(entry => new
            {
                username = entry.Username,
                avatarUrl = ToLauncherAvatarUrl(entry.AvatarFilePath),
                avatarPlaceholder = entry.AvatarPlaceholder,
                isOnline = entry.IsOnline,
                presenceText = entry.PresenceText,
                activityText = entry.ActivityText,
                versionText = entry.VersionText,
                joinAddressText = entry.JoinAddressText,
                hasJoinEndpoint = entry.HasJoinEndpoint,
                canConnect = entry.CanConnect,
                connectTooltip = entry.ConnectTooltip,
                preferredVersionId = entry.PreferredVersionId
            }).ToArray(),
            incomingRequests = _incomingFriendRequests.Select(entry => new
            {
                requestId = entry.RequestId,
                username = entry.Username,
                createdAtUtc = entry.CreatedAtUtc,
                avatarUrl = ToLauncherAvatarUrl(entry.AvatarFilePath),
                avatarPlaceholder = entry.AvatarPlaceholder,
                subtitleText = entry.SubtitleText
            }).ToArray()
        };
    }

    private string ResolvePhotinoAccountMode()
    {
        if (HasAuthenticatedCloudSession() || (HasIncognitoIdentity() && !_isEditingIncognitoNickname))
        {
            return "summary";
        }

        return _accountEntryMode switch
        {
            AccountEntryMode.Register => "register",
            AccountEntryMode.Guest => "guest",
            _ => "login"
        };
    }

    private static SidePanelSection ParsePhotinoSidePanelSection(string? section)
    {
        return section?.Trim().ToLowerInvariant() switch
        {
            "account" => SidePanelSection.Account,
            "settings" => SidePanelSection.Settings,
            "skin" => SidePanelSection.Skin,
            "background" => SidePanelSection.Background,
            "mods" => SidePanelSection.Mods,
            "friends" => SidePanelSection.Friends,
            _ => SidePanelSection.None
        };
    }

    private static AccountEntryMode ParsePhotinoAccountEntryMode(string? mode)
    {
        return mode?.Trim().ToLowerInvariant() switch
        {
            "register" => AccountEntryMode.Register,
            "guest" => AccountEntryMode.Guest,
            _ => AccountEntryMode.Login
        };
    }

    private string? ResolvePhotinoModIconUrl(RecommendedModCatalogItem item)
    {
        return ToLauncherImageUrl(item.IconUrl) ?? ToLauncherFileUrl(item.SourceIconUrl);
    }

    private static string? TryGetString(JsonElement payload, string propertyName)
    {
        if (payload.ValueKind != JsonValueKind.Object || !payload.TryGetProperty(propertyName, out var property))
        {
            return null;
        }

        return property.ValueKind switch
        {
            JsonValueKind.String => property.GetString(),
            JsonValueKind.Number => property.GetRawText(),
            JsonValueKind.True => bool.TrueString,
            JsonValueKind.False => bool.FalseString,
            _ => null
        };
    }

    private static IReadOnlyList<string> TryGetStringArray(JsonElement payload, string propertyName)
    {
        if (payload.ValueKind != JsonValueKind.Object ||
            !payload.TryGetProperty(propertyName, out var property) ||
            property.ValueKind != JsonValueKind.Array)
        {
            return Array.Empty<string>();
        }

        return property.EnumerateArray()
            .Select(item => item.ValueKind == JsonValueKind.String ? item.GetString() : item.GetRawText())
            .Where(item => !string.IsNullOrWhiteSpace(item))
            .Cast<string>()
            .ToArray();
    }

    private static bool? TryGetBool(JsonElement payload, string propertyName)
    {
        if (payload.ValueKind != JsonValueKind.Object || !payload.TryGetProperty(propertyName, out var property))
        {
            return null;
        }

        return property.ValueKind switch
        {
            JsonValueKind.True => true,
            JsonValueKind.False => false,
            JsonValueKind.String when bool.TryParse(property.GetString(), out var parsed) => parsed,
            JsonValueKind.Number when property.TryGetInt32(out var numeric) => numeric != 0,
            _ => null
        };
    }

    private static int? TryGetInt(JsonElement payload, string propertyName)
    {
        if (payload.ValueKind != JsonValueKind.Object || !payload.TryGetProperty(propertyName, out var property))
        {
            return null;
        }

        if (property.ValueKind == JsonValueKind.Number && property.TryGetInt32(out var value))
        {
            return value;
        }

        return property.ValueKind == JsonValueKind.String && int.TryParse(property.GetString(), out value)
            ? value
            : null;
    }

    private static long? TryGetLong(JsonElement payload, string propertyName)
    {
        if (payload.ValueKind != JsonValueKind.Object || !payload.TryGetProperty(propertyName, out var property))
        {
            return null;
        }

        if (property.ValueKind == JsonValueKind.Number && property.TryGetInt64(out var value))
        {
            return value;
        }

        return property.ValueKind == JsonValueKind.String && long.TryParse(property.GetString(), out value)
            ? value
            : null;
    }

    private static double? TryGetDouble(JsonElement payload, string propertyName)
    {
        if (payload.ValueKind != JsonValueKind.Object || !payload.TryGetProperty(propertyName, out var property))
        {
            return null;
        }

        if (property.ValueKind == JsonValueKind.Number && property.TryGetDouble(out var value))
        {
            return value;
        }

        return property.ValueKind == JsonValueKind.String && double.TryParse(property.GetString(), out value)
            ? value
            : null;
    }

    private static string? ToLauncherFileUrl(string? path)
    {
        if (string.IsNullOrWhiteSpace(path))
        {
            return null;
        }

        if (Uri.TryCreate(path, UriKind.Absolute, out var uri))
        {
            if (uri.IsFile)
            {
                return File.Exists(uri.LocalPath)
                    ? LocalStaticFileServer.BuildLauncherFileUrl(uri.LocalPath)
                    : null;
            }

            if (uri.Scheme == Uri.UriSchemeHttp || uri.Scheme == Uri.UriSchemeHttps || uri.Scheme == "data")
            {
                return uri.ToString();
            }
        }

        return File.Exists(path)
            ? LocalStaticFileServer.BuildLauncherFileUrl(path)
            : null;
    }

    private static string? ToLauncherImageUrl(string? path)
    {
        if (string.IsNullOrWhiteSpace(path))
        {
            return null;
        }

        if (Uri.TryCreate(path, UriKind.Absolute, out var uri) &&
            uri.IsFile &&
            File.Exists(uri.LocalPath))
        {
            return TryReadImageDataUrl(uri.LocalPath) ??
                   LocalStaticFileServer.BuildLauncherFileUrl(uri.LocalPath);
        }

        if (File.Exists(path))
        {
            return TryReadImageDataUrl(path) ??
                   LocalStaticFileServer.BuildLauncherFileUrl(path);
        }

        return ToLauncherFileUrl(path);
    }

    private static string? TryReadImageDataUrl(string path)
    {
        try
        {
            var fileInfo = new FileInfo(path);
            if (!fileInfo.Exists || fileInfo.Length is <= 0 or > 96 * 1024)
            {
                return null;
            }

            var contentType = Path.GetExtension(path).ToLowerInvariant() switch
            {
                ".jpg" or ".jpeg" => "image/jpeg",
                ".webp" => "image/webp",
                ".bmp" => "image/bmp",
                ".gif" => "image/gif",
                _ => "image/png"
            };
            var bytes = File.ReadAllBytes(path);
            return $"data:{contentType};base64,{Convert.ToBase64String(bytes)}";
        }
        catch
        {
            return null;
        }
    }

    private static string? ToLauncherAvatarUrl(string? path)
    {
        if (string.IsNullOrWhiteSpace(path) || !File.Exists(path))
        {
            return null;
        }

        try
        {
            var fileInfo = new FileInfo(path);
            if (fileInfo.Length is > 0 and <= 256 * 1024)
            {
                var contentType = Path.GetExtension(path).ToLowerInvariant() switch
                {
                    ".jpg" or ".jpeg" => "image/jpeg",
                    ".webp" => "image/webp",
                    ".bmp" => "image/bmp",
                    ".gif" => "image/gif",
                    _ => "image/png"
                };
                var bytes = File.ReadAllBytes(path);
                return $"data:{contentType};base64,{Convert.ToBase64String(bytes)}";
            }
        }
        catch
        {
            // Fall back to the local static server route below.
        }

        return LocalStaticFileServer.BuildLauncherFileUrl(path);
    }

    private void SendHostWindowBehindPhotino()
    {
        try
        {
            var handle = new WindowInteropHelper(this).EnsureHandle();
            if (handle == IntPtr.Zero)
            {
                return;
            }

            SetWindowPos(handle, HwndBottom, 0, 0, 0, 0, SwpNoMove | SwpNoSize | SwpNoActivate | SwpNoOwnerZOrder);
        }
        catch
        {
            // Keep the host window best-effort only.
        }
    }

    [DllImport("user32.dll", SetLastError = true)]
    private static extern bool SetWindowPos(
        IntPtr hWnd,
        IntPtr hWndInsertAfter,
        int x,
        int y,
        int cx,
        int cy,
        uint uFlags);
}

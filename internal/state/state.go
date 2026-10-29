package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"vesper-launcher/internal/models"
	"vesper-launcher/internal/platform"
)

const (
	DefaultWindowWidth  = 1405
	DefaultWindowHeight = 840
	MinWindowWidth      = 900
	MinWindowHeight     = 580
	MaxWindowWidth      = 3840
	MaxWindowHeight     = 2160
)

type LauncherState struct {
	mu sync.RWMutex

	ActiveSection           string
	ActiveSettingsTab       string
	IsBusy                  bool
	IsGameRunning           bool
	Nickname                string
	AvatarUrl               string
	HasAccount              bool
	AccountType             string
	SelectedVersion         string
	StatusText              string
	ProgressText            string
	ProgressOverlay         string
	ProgressPercent         float64
	IsIndeterminate         bool
	JavaMemoryMb            int
	JavaPath                string
	JvmArgs                 string
	LanguageCode            string
	SkinModel               string
	AutoClose               bool
	CurrentBgUrl            string
	CurrentSkinUrl          string
	SearchQuery             string
	SelectedCategory        string
	ModProvider             string
	IsCatalogLoading        bool
	LoginFormPlacementId    string
	LauncherDirectoryViewId string
	JavaRuntimeMode         string
	WindowWidth             int
	WindowHeight            int
	ModItems                []models.ModItem
	Versions                []models.VersionOption
	Friends                 []models.FriendEntry
	FriendRequests          []models.FriendRequest
	Updates                 models.UpdatesSnapshot
	InstalledProjects       map[string]string
}

func NewLauncherState() *LauncherState {
	s := &LauncherState{
		InstalledProjects:       make(map[string]string),
		ActiveSection:           "none",
		ActiveSettingsTab:       "launcher",
		Nickname:                "",
		StatusText:              "Готов к запуску",
		SelectedVersion:         "1.21.4",
		JavaMemoryMb:            4096,
		LanguageCode:            "ru_ru",
		CurrentSkinUrl:          "/assets/steve.png",
		SkinModel:               "auto",
		SelectedCategory:        "Моды",
		ModProvider:             "modrinth",
		LoginFormPlacementId:    "center",
		LauncherDirectoryViewId: "current",
		JavaRuntimeMode:         "system",
		WindowWidth:             DefaultWindowWidth,
		WindowHeight:            DefaultWindowHeight,
		Updates: models.UpdatesSnapshot{
			IsAvailable: false,
			Version:     "1.0.65",
		},
		Versions: []models.VersionOption{
			{Key: "1.21.4", DisplayName: "1.21.4 - ванильная", Category: "release", IsInstalled: false},
			{Key: "1.20.4", DisplayName: "1.20.4 - ванильная", Category: "release", IsInstalled: false},
			{Key: "1.16.5", DisplayName: "1.16.5 - ванильная", Category: "release", IsInstalled: false},
			{Key: "1.12.2", DisplayName: "1.12.2 - ванильная", Category: "release", IsInstalled: false},
		},
	}
	s.updateThemeByTime()
	s.Load()
	return s
}

func (s *LauncherState) isVersionInstalledLocked(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	for _, v := range s.Versions {
		if strings.ToLower(v.Key) == lower {
			if v.IsInstalled {
				return true
			}
		}
	}

	mcDir := platform.GetMinecraftDir()
	versionsDir := filepath.Join(mcDir, "versions")

	checkInstalled := func(dirName string) bool {
		jsonPath := filepath.Join(versionsDir, dirName, dirName+".json")
		if _, err := os.Stat(jsonPath); err != nil {
			return false
		}
		jarPath := filepath.Join(versionsDir, dirName, dirName+".jar")
		if _, err := os.Stat(jarPath); err == nil {
			return true
		}
		// Check if it inherits from another version
		if data, err := os.ReadFile(jsonPath); err == nil {
			var meta struct {
				InheritsFrom string `json:"inheritsFrom"`
				Jar          string `json:"jar"`
			}
			if errU := json.Unmarshal(data, &meta); errU == nil {
				parent := meta.Jar
				if parent == "" {
					parent = meta.InheritsFrom
				}
				if parent != "" {
					parentJar := filepath.Join(versionsDir, parent, parent+".jar")
					if _, errP := os.Stat(parentJar); errP == nil {
						return true
					}
				}
			}
		}
		return false
	}

	if checkInstalled(key) || checkInstalled(lower) {
		return true
	}

	if entries, err := os.ReadDir(versionsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := strings.ToLower(e.Name())
			if name == lower {
				if checkInstalled(e.Name()) {
					return true
				}
			}
			if strings.HasPrefix(lower, "fabric-") {
				mcVer := strings.TrimPrefix(lower, "fabric-")
				if (strings.Contains(name, "fabric") || strings.HasPrefix(name, "fabric-loader")) && strings.Contains(name, mcVer) {
					if checkInstalled(e.Name()) {
						return true
					}
				}
			}
			if strings.HasPrefix(lower, "forge-") {
				mcVer := strings.TrimPrefix(lower, "forge-")
				if strings.Contains(name, "forge") && strings.Contains(name, mcVer) {
					if checkInstalled(e.Name()) {
						return true
					}
				}
			}
			if strings.HasPrefix(lower, "neoforge-") {
				mcVer := strings.TrimPrefix(lower, "neoforge-")
				if strings.Contains(name, "neoforge") && strings.Contains(name, mcVer) {
					if checkInstalled(e.Name()) {
						return true
					}
				}
			}
			if strings.HasPrefix(lower, "optifine-") || strings.HasPrefix(lower, "optifine ") {
				mcVer := strings.TrimPrefix(lower, "optifine-")
				mcVer = strings.TrimPrefix(mcVer, "optifine ")
				if strings.Contains(mcVer, " ") {
					mcVer = strings.Fields(mcVer)[0]
				}
				if strings.Contains(name, "optifine") && strings.Contains(name, mcVer) {
					if checkInstalled(e.Name()) {
						return true
					}
				}
			}
		}
	}

	return false
}

func (s *LauncherState) IsVersionInstalled(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isVersionInstalledLocked(key)
}

func (s *LauncherState) updateThemeByTime() {
	hour := time.Now().Hour()
	if hour >= 6 && hour < 12 {
		s.CurrentBgUrl = "/assets/bg-sunrise.png"
	} else if hour >= 12 && hour < 18 {
		s.CurrentBgUrl = "/assets/bg-day.png"
	} else if hour >= 18 && hour < 23 {
		s.CurrentBgUrl = "/assets/bg-sunset.png"
	} else {
		s.CurrentBgUrl = "/assets/bg-night.png"
	}
}

func (s *LauncherState) GetSnapshot() *models.HostSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	isInstalled := s.isVersionInstalledLocked(s.SelectedVersion)

	launchText := "Играть"
	if s.IsBusy {
		if isInstalled {
			launchText = "Запуск..."
		} else {
			launchText = "Установка..."
		}
	} else if s.IsGameRunning {
		launchText = "Игра запущена"
	} else if !isInstalled {
		launchText = "Установить"
	}

	inlineLabel := s.SelectedVersion
	for _, v := range s.Versions {
		if v.Key == s.SelectedVersion {
			inlineLabel = v.DisplayName
			break
		}
	}

	updatePercent := 100.0
	return &models.HostSnapshot{
		Phase:        "ready",
		ErrorMessage: nil,
		Update: models.UpdateProgress{
			Message:         "Лаунчер готов",
			DetailMessage:   "",
			ProgressPercent: &updatePercent,
			IsIndeterminate: false,
			ProgressText:    "",
		},
		Launcher: &models.LauncherSnapshot{
			ActiveSection:      s.ActiveSection,
			ActiveSettingsTab:  s.ActiveSettingsTab,
			IsBusy:             s.IsBusy,
			IsGameRunning:      s.IsGameRunning,
			CanAccessFriends:   true,
			NotificationsCount: len(s.FriendRequests),
			Notifications:      []map[string]any{},
			Updates:            s.Updates,
			Theme: models.ThemeSnapshot{
				BackgroundUrl: s.CurrentBgUrl,
				LogoUrl:       "/assets/vesper-logo.png",
				WordmarkUrl:   "/assets/vesper-launcher-wordmark.png",
			},
			Main: models.MainSnapshot{
				Nickname:                 s.Nickname,
				StatusText:               s.StatusText,
				ProgressText:             s.ProgressText,
				ProgressOverlayText:      s.ProgressOverlay,
				ProgressPercent:          s.ProgressPercent,
				IsProgressIndeterminate:  s.IsIndeterminate,
				LaunchButtonText:         launchText,
				IsVersionOperationActive: s.IsBusy,
				SelectedVersionKey:       s.SelectedVersion,
				InlineVersionLabel:       inlineLabel,
				Versions:                 s.Versions,
				AvailableVersions:        s.Versions,
			},
			Account: func() models.AccountSnapshot {
				accountMode := "login"
				accountStateText := "Сессия не активна."
				if s.HasAccount {
					accountMode = "summary"
					accountStateText = "Облачная сессия активна."
				}
				avatarPlaceholder := s.buildAvatarPlaceholder()
				nickTrim := strings.TrimSpace(s.Nickname)
				var recentUsers []string
				if nickTrim != "" {
					recentUsers = []string{nickTrim}
				} else {
					recentUsers = []string{}
				}
				return models.AccountSnapshot{
					Mode:                    accountMode,
					AccountStateText:        accountStateText,
					CurrentNickname:         nickTrim,
					AvatarUrl:               s.AvatarUrl,
					AvatarPlaceholder:       avatarPlaceholder,
					HasAccount:              s.HasAccount,
					HasAuthenticatedSession: s.HasAccount,
					AccountType:             s.AccountType,
					RecentUsernames:         recentUsers,
					CanChangeAvatar:         s.HasAccount,
				}
			}(),
			Settings: models.SettingsSnapshot{
				ActiveTab:                    s.ActiveSettingsTab,
				MemoryMb:                     s.JavaMemoryMb,
				DisplayedMemoryMb:            s.JavaMemoryMb,
				MinimumMemoryMb:              1024,
				MaximumMemoryMb:              12288,
				JavaMemoryMb:                 s.JavaMemoryMb,
				JavaPath:                     s.JavaPath,
				EffectiveJavaPath:            s.JavaPath,
				JvmArgs:                      s.JvmArgs,
				ExtraJvmArgs:                 s.JvmArgs,
				ShowJvmArgs:                  true,
				AutoOptimizeMemory:           false,
				AutoMinimizeOnLaunch:         false,
				AutoCloseOnLaunch:            s.AutoClose,
				RestoreLauncherAfterGameExit: true,
				ClickSoundEnabled:            true,
				JavaRuntimeMode:              s.JavaRuntimeMode,
				JavaRuntimeOptions: []map[string]string{
					{"id": "auto", "label": "Автоматически"},
					{"id": "system", "label": "Системная Java"},
					{"id": "custom", "label": "Свой путь"},
				},
				LoginFormPlacementId: s.LoginFormPlacementId,
				LoginPlacementOptions: []map[string]string{
					{"id": "center", "label": "Центр"},
					{"id": "left", "label": "Слева"},
				},
				LauncherDirectoryViewId: s.LauncherDirectoryViewId,
				DirectoryViewOptions: []map[string]string{
					{"id": "current", "label": "Текущая"},
					{"id": "minecraft", "label": ".minecraft"},
				},
				JavaModeHint:           s.GetJavaModeHint(),
				AutoMemoryHint:         "Память задана вручную.",
				AutoMinimizeHint:       "Будет скрыт после запуска игры.",
				RestoreHint:            "Лаунчер восстановится после завершения игры.",
				JvmArgsHint:            "Поле открыто.",
				MinecraftLanguageCode:  s.LanguageCode,
				DisplayedGameDirectory: platform.GetMinecraftDir(),
				LanguageOptions: []map[string]string{
					{"id": "auto", "label": "Авто"},
					{"id": "ru_ru", "label": "Русский"},
					{"id": "en_us", "label": "English"},
				},
			},
			Skin: models.SkinSnapshot{
				CurrentSkinUrl:     s.CurrentSkinUrl,
				SelectedSkinUrl:    s.CurrentSkinUrl,
				SelectedSkinIsSlim: false,
				ModelPreferenceId:  s.SkinModel,
				SelectedSkinLabel:  "Стандартный скин",
			},
			Background: models.BackgroundSnapshot{
				CurrentBgUrl:         s.CurrentBgUrl,
				AppliedBackgroundUrl: s.CurrentBgUrl,
				Items: []models.BackgroundItem{
					{FileName: "bg-day.png", Label: "День", Url: "/assets/bg-day.png", IsActive: s.CurrentBgUrl == "/assets/bg-day.png"},
					{FileName: "bg-night.png", Label: "Ночь", Url: "/assets/bg-night.png", IsActive: s.CurrentBgUrl == "/assets/bg-night.png"},
					{FileName: "bg-sunrise.png", Label: "Рассвет", Url: "/assets/bg-sunrise.png", IsActive: s.CurrentBgUrl == "/assets/bg-sunrise.png"},
					{FileName: "bg-sunset.png", Label: "Закат", Url: "/assets/bg-sunset.png", IsActive: s.CurrentBgUrl == "/assets/bg-sunset.png"},
				},
			},
			Mods: models.ModsSnapshot{
				Provider:           s.ModProvider,
				Items:              s.getDecoratedModItemsLocked(s.ScanInstalledFiles()),
				SearchQuery:        s.SearchQuery,
				SelectedCategory:   s.SelectedCategory,
				IsCatalogLoading:   s.IsCatalogLoading,
				CatalogSummary:     "Популярные модификации для Minecraft",
				InstalledFileNames: s.ScanInstalledFiles(),
			},
			Friends: models.FriendsSnapshot{
				ProfileAvatarUrl:         s.AvatarUrl,
				ProfileAvatarPlaceholder: s.buildAvatarPlaceholder(),
				ProfileNickname:          s.Nickname,
				ProfileType:              "Тип: " + s.AccountType,
				CanManage:                s.HasAccount,
				Friends:                  s.Friends,
				IncomingRequests:         s.FriendRequests,
			},
		},
	}
}

func (s *LauncherState) buildAvatarPlaceholder() string {
	nickTrim := strings.TrimSpace(s.Nickname)
	if len(nickTrim) >= 2 {
		return strings.ToUpper(nickTrim[:2])
	} else if len(nickTrim) == 1 {
		return strings.ToUpper(nickTrim)
	}
	return "AV"
}

func (s *LauncherState) SetSection(section string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ActiveSection = section
}

func (s *LauncherState) SetSelectedVersion(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SelectedVersion = key
	s.Save()
}

func (s *LauncherState) SetNickname(nick string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if nick != "" {
		s.Nickname = nick
		s.Save()
	}
}

func (s *LauncherState) SetStatus(status, progress string, percent float64, indeterminate bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.StatusText = status
	s.ProgressText = progress
	s.ProgressPercent = percent
	s.IsIndeterminate = indeterminate
}

func (s *LauncherState) SetBusy(busy bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.IsBusy = busy
}

func (s *LauncherState) SetGameRunning(running bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.IsGameRunning = running
	if running {
		s.StatusText = "Игра запущена"
	} else {
		s.StatusText = "Готов к запуску"
	}
}

func (s *LauncherState) SetBackground(bgUrl string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CurrentBgUrl = bgUrl
}

type serializedState struct {
	Nickname                string            `json:"nickname"`
	SelectedVersion         string            `json:"selectedVersion"`
	JavaMemoryMb            int               `json:"javaMemoryMb"`
	JavaPath                string            `json:"javaPath"`
	JvmArgs                 string            `json:"jvmArgs"`
	LanguageCode            string            `json:"languageCode"`
	CurrentBgUrl            string            `json:"currentBgUrl"`
	LoginFormPlacementId    string            `json:"loginFormPlacementId"`
	LauncherDirectoryViewId string            `json:"launcherDirectoryViewId"`
	JavaRuntimeMode         string            `json:"javaRuntimeMode"`
	WindowWidth             int               `json:"windowWidth"`
	WindowHeight            int               `json:"windowHeight"`
	InstalledProjects       map[string]string `json:"installedProjects,omitempty"`
}

func (s *LauncherState) Save() {
	configDir := platform.GetVesperConfigDir()
	_ = os.MkdirAll(configDir, 0755)
	filePath := filepath.Join(configDir, "launcher-state.json")

	data := serializedState{
		Nickname:                s.Nickname,
		SelectedVersion:         s.SelectedVersion,
		JavaMemoryMb:            s.JavaMemoryMb,
		JavaPath:                s.JavaPath,
		JvmArgs:                 s.JvmArgs,
		LanguageCode:            s.LanguageCode,
		CurrentBgUrl:            s.CurrentBgUrl,
		LoginFormPlacementId:    s.LoginFormPlacementId,
		LauncherDirectoryViewId: s.LauncherDirectoryViewId,
		JavaRuntimeMode:         s.JavaRuntimeMode,
		WindowWidth:             s.WindowWidth,
		WindowHeight:            s.WindowHeight,
		InstalledProjects:       s.InstalledProjects,
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err == nil {
		_ = os.WriteFile(filePath, bytes, 0644)
	}
}

func (s *LauncherState) Load() {
	configDir := platform.GetVesperConfigDir()
	filePath := filepath.Join(configDir, "launcher-state.json")

	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	var data serializedState
	if err := json.Unmarshal(bytes, &data); err == nil {
		if data.Nickname != "" && data.Nickname != "Steve" {
			s.Nickname = data.Nickname
		}
		if data.SelectedVersion != "" {
			s.SelectedVersion = data.SelectedVersion
		}
		if data.JavaMemoryMb > 0 {
			s.JavaMemoryMb = data.JavaMemoryMb
		}
		if data.JavaPath != "" {
			s.JavaPath = data.JavaPath
		}
		if data.JvmArgs != "" {
			s.JvmArgs = data.JvmArgs
		}
		if data.LanguageCode != "" {
			s.LanguageCode = data.LanguageCode
		}
		if data.CurrentBgUrl != "" {
			s.CurrentBgUrl = data.CurrentBgUrl
		}
		if data.LoginFormPlacementId != "" {
			s.LoginFormPlacementId = data.LoginFormPlacementId
		}
		if data.LauncherDirectoryViewId != "" {
			s.LauncherDirectoryViewId = data.LauncherDirectoryViewId
		}
		if data.JavaRuntimeMode != "" {
			s.JavaRuntimeMode = data.JavaRuntimeMode
		}
		if data.WindowWidth >= MinWindowWidth && data.WindowWidth <= MaxWindowWidth {
			s.WindowWidth = data.WindowWidth
		} else {
			s.WindowWidth = DefaultWindowWidth
		}
		if data.WindowHeight >= MinWindowHeight && data.WindowHeight <= MaxWindowHeight {
			s.WindowHeight = data.WindowHeight
		} else {
			s.WindowHeight = DefaultWindowHeight
		}
		if data.InstalledProjects != nil {
			s.InstalledProjects = data.InstalledProjects
		}
	}
}

func (s *LauncherState) ScanInstalledFiles() []string {
	mcDir := platform.GetMinecraftDir()
	dirs := []string{
		filepath.Join(mcDir, "mods"),
		filepath.Join(mcDir, "resourcepacks"),
		filepath.Join(mcDir, "shaderpacks"),
		filepath.Join(mcDir, "modpacks"),
	}

	seen := make(map[string]bool)
	var files []string

	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				name := e.Name()
				if !seen[name] {
					seen[name] = true
					files = append(files, name)
				}
			}
		}
	}
	return files
}

func (s *LauncherState) getDecoratedModItemsLocked(installedFiles []string) []models.ModItem {
	lowerFiles := make([]string, len(installedFiles))
	for i, f := range installedFiles {
		lowerFiles[i] = strings.ToLower(f)
	}

	result := make([]models.ModItem, len(s.ModItems))
	for i, it := range s.ModItems {
		itemCopy := it
		isInstalled := false

		if s.InstalledProjects != nil {
			if f, ok := s.InstalledProjects[it.ProjectId]; ok && f != "" {
				isInstalled = true
			}
		}

		if !isInstalled && it.ProjectId != "" {
			pidLower := strings.ToLower(it.ProjectId)
			cleanPid := strings.TrimPrefix(pidLower, "cf-")
			for _, f := range lowerFiles {
				if strings.Contains(f, cleanPid) {
					isInstalled = true
					break
				}
			}
		}

		itemCopy.IsInstalled = isInstalled
		if isInstalled {
			itemCopy.ActionText = "Удалить"
		} else {
			itemCopy.ActionText = "Установить"
		}
		result[i] = itemCopy
	}
	return result
}

func (s *LauncherState) SetProjectInstalled(projectId, filename string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.InstalledProjects == nil {
		s.InstalledProjects = make(map[string]string)
	}
	s.InstalledProjects[projectId] = filename
	s.Save()
}

func (s *LauncherState) RemoveProjectInstalled(projectId string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.InstalledProjects != nil {
		delete(s.InstalledProjects, projectId)
	}
	s.Save()
}

func (s *LauncherState) IsProjectInstalled(projectId string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.InstalledProjects == nil {
		return false
	}
	_, ok := s.InstalledProjects[projectId]
	return ok
}

func (s *LauncherState) GetInstalledProjectFile(projectId string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.InstalledProjects == nil {
		return ""
	}
	return s.InstalledProjects[projectId]
}

func (s *LauncherState) SetVersionCatalog(versions []models.VersionOption) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Versions = versions
}

func (s *LauncherState) SetMods(items []models.ModItem, query, category, provider string, loading bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ModItems = items
	s.SearchQuery = query
	s.SelectedCategory = category
	if provider != "" {
		s.ModProvider = provider
	}
	s.IsCatalogLoading = loading
}

func (s *LauncherState) SetFriends(friends []models.FriendEntry, requests []models.FriendRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Friends = friends
	s.FriendRequests = requests
}

func (s *LauncherState) SetUpdates(up models.UpdatesSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Updates = up
}

func (s *LauncherState) SetAuth(hasAuth bool, accountType, nick, avatar string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.HasAccount = hasAuth
	s.AccountType = accountType
	if nick != "" {
		s.Nickname = nick
	} else if !hasAuth {
		s.Nickname = ""
	}
	s.AvatarUrl = avatar
	s.Save()
}

func (s *LauncherState) GetJavaModeHint() string {
	switch s.JavaRuntimeMode {
	case "auto":
		return "Автоматический выбор подходящей версии Java (Java 8 / 17 / 21) под версию игры."
	case "system":
		return "Использовать системную Java по умолчанию из PATH."
	case "custom":
		return "Использовать указанный вручную путь к исполняемому файлу Java."
	default:
		return "Лаунчер использует системную или указанную Java для запуска Minecraft."
	}
}

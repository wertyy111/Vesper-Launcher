package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"vesper-launcher/internal/models"
	"vesper-launcher/internal/platform"
)

type LauncherState struct {
	mu sync.RWMutex

	ActiveSection     string
	ActiveSettingsTab string
	IsBusy            bool
	IsGameRunning     bool
	Nickname          string
	AvatarUrl         string
	HasAccount        bool
	AccountType       string
	SelectedVersion   string
	StatusText        string
	ProgressText      string
	ProgressOverlay   string
	ProgressPercent   float64
	IsIndeterminate   bool
	JavaMemoryMb      int
	JavaPath          string
	JvmArgs           string
	LanguageCode      string
	AutoClose         bool
	CurrentBgUrl      string
	CurrentSkinUrl    string
	SearchQuery       string
	SelectedCategory  string
	ModItems          []models.ModItem
	Versions          []models.VersionOption
	Friends           []models.FriendEntry
	FriendRequests    []models.FriendRequest
}

func NewLauncherState() *LauncherState {
	s := &LauncherState{
		ActiveSection:     "none",
		ActiveSettingsTab: "common",
		Nickname:          "Steve",
		StatusText:        "Готов к запуску",
		SelectedVersion:   "1.21.4",
		JavaMemoryMb:      4096,
		LanguageCode:      "ru_ru",
		CurrentSkinUrl:    "/assets/steve.png",
		SelectedCategory:  "Моды",
		Versions: []models.VersionOption{
			{Key: "1.21.4", DisplayName: "1.21.4 - ванильная", Category: "release", IsInstalled: true},
			{Key: "1.20.4", DisplayName: "1.20.4 - ванильная", Category: "release", IsInstalled: false},
			{Key: "1.16.5", DisplayName: "1.16.5 - ForgeOptiFine", Category: "forge", IsInstalled: true},
			{Key: "1.12.2", DisplayName: "1.12.2 - ванильная", Category: "release", IsInstalled: false},
		},
	}
	s.updateThemeByTime()
	s.Load()
	return s
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

	launchText := "Играть"
	if s.IsBusy {
		launchText = "Загрузка..."
	} else if s.IsGameRunning {
		launchText = "Игра запущена"
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
			Updates: models.UpdatesSnapshot{
				IsAvailable: false,
				Version:     "1.0.65",
			},
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
			},
			Account: models.AccountSnapshot{
				CurrentNickname: s.Nickname,
				AvatarUrl:       s.AvatarUrl,
				HasAccount:      s.HasAccount,
				AccountType:     s.AccountType,
			},
			Settings: models.SettingsSnapshot{
				MinecraftLanguageCode: s.LanguageCode,
				JavaMemoryMb:          s.JavaMemoryMb,
				JavaPath:              s.JavaPath,
				JvmArgs:               s.JvmArgs,
				AutoCloseOnLaunch:     s.AutoClose,
			},
			Skin: models.SkinSnapshot{
				CurrentSkinUrl: s.CurrentSkinUrl,
			},
			Background: models.BackgroundSnapshot{
				CurrentBgUrl: s.CurrentBgUrl,
			},
			Mods: models.ModsSnapshot{
				Items:            s.ModItems,
				SearchQuery:      s.SearchQuery,
				SelectedCategory: s.SelectedCategory,
				IsCatalogLoading: false,
				CatalogSummary:   "Популярные модификации для Minecraft",
			},
			Friends: models.FriendsSnapshot{
				Friends:          s.Friends,
				IncomingRequests: s.FriendRequests,
			},
		},
	}
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
	Nickname        string `json:"nickname"`
	SelectedVersion string `json:"selectedVersion"`
	JavaMemoryMb    int    `json:"javaMemoryMb"`
	JavaPath        string `json:"javaPath"`
	JvmArgs         string `json:"jvmArgs"`
	LanguageCode    string `json:"languageCode"`
	CurrentBgUrl    string `json:"currentBgUrl"`
}

func (s *LauncherState) Save() {
	configDir := platform.GetVesperConfigDir()
	_ = os.MkdirAll(configDir, 0755)
	filePath := filepath.Join(configDir, "launcher-state.json")

	data := serializedState{
		Nickname:        s.Nickname,
		SelectedVersion: s.SelectedVersion,
		JavaMemoryMb:    s.JavaMemoryMb,
		JavaPath:        s.JavaPath,
		JvmArgs:         s.JvmArgs,
		LanguageCode:    s.LanguageCode,
		CurrentBgUrl:    s.CurrentBgUrl,
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
		if data.Nickname != "" {
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
	}
}

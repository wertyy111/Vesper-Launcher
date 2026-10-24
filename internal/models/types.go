package models

type HostEnvelope struct {
	Type    string        `json:"type"`
	Data    *HostSnapshot `json:"data,omitempty"`
	Message string        `json:"message,omitempty"`
}

type HostSnapshot struct {
	Phase        string            `json:"phase"`
	ErrorMessage *string           `json:"errorMessage"`
	Update       UpdateProgress    `json:"update"`
	Launcher     *LauncherSnapshot `json:"launcher,omitempty"`
}

type UpdateProgress struct {
	Message         string   `json:"message"`
	DetailMessage   string   `json:"detailMessage"`
	ProgressPercent *float64 `json:"progressPercent"`
	IsIndeterminate bool     `json:"isIndeterminate"`
	ProgressText    string   `json:"progressText"`
}

type LauncherSnapshot struct {
	ActiveSection      string             `json:"activeSection"`
	ActiveSettingsTab  string             `json:"activeSettingsTab"`
	IsBusy             bool               `json:"isBusy"`
	IsGameRunning      bool               `json:"isGameRunning"`
	CanAccessFriends   bool               `json:"canAccessFriends"`
	NotificationsCount int                `json:"notificationsCount"`
	Notifications      []map[string]any   `json:"notifications"`
	Updates            UpdatesSnapshot    `json:"updates"`
	Theme              ThemeSnapshot      `json:"theme"`
	Main               MainSnapshot       `json:"main"`
	Account            AccountSnapshot    `json:"account"`
	Settings           SettingsSnapshot   `json:"settings"`
	Skin               SkinSnapshot       `json:"skin"`
	Background         BackgroundSnapshot `json:"background"`
	Mods               ModsSnapshot       `json:"mods"`
	Friends            FriendsSnapshot    `json:"friends"`
}

type UpdatesSnapshot struct {
	IsAvailable      bool    `json:"isAvailable"`
	Version          string  `json:"version"`
	Title            string  `json:"title,omitempty"`
	ReleaseNotes     string  `json:"releaseNotes,omitempty"`
	UsesDelta        bool    `json:"usesDelta"`
	DeltaCount       int     `json:"deltaCount"`
	DownloadBytes    int64   `json:"downloadBytes"`
	FullPackageBytes int64   `json:"fullPackageBytes"`
	IsInstalling     bool    `json:"isInstalling"`
	ProgressPercent  int     `json:"progressPercent"`
	ErrorMessage     *string `json:"errorMessage"`
}

type ThemeSnapshot struct {
	BackgroundUrl string `json:"backgroundUrl"`
	LogoUrl       string `json:"logoUrl"`
	WordmarkUrl   string `json:"wordmarkUrl"`
}

type VersionOption struct {
	Key         string `json:"key"`
	DisplayName string `json:"displayName"`
	Category    string `json:"category"`
	IsInstalled bool   `json:"isInstalled"`
}

type MainSnapshot struct {
	Nickname                 string          `json:"nickname"`
	StatusText               string          `json:"statusText"`
	ProgressText             string          `json:"progressText"`
	ProgressOverlayText      string          `json:"progressOverlayText"`
	ProgressPercent          float64         `json:"progressPercent"`
	IsProgressIndeterminate  bool            `json:"isProgressIndeterminate"`
	LaunchButtonText         string          `json:"launchButtonText"`
	IsVersionOperationActive bool            `json:"isVersionOperationActive"`
	SelectedVersionKey       string          `json:"selectedVersionKey"`
	InlineVersionLabel       string          `json:"inlineVersionLabel"`
	Versions                 []VersionOption `json:"versions"`
}

type AccountSnapshot struct {
	Mode                    string   `json:"mode"`
	AccountStateText        string   `json:"accountStateText"`
	CurrentNickname         string   `json:"currentNickname"`
	AvatarUrl               string   `json:"avatarUrl"`
	AvatarPlaceholder       string   `json:"avatarPlaceholder"`
	HasAccount              bool     `json:"hasAccount"`
	HasAuthenticatedSession bool     `json:"hasAuthenticatedSession"`
	AccountType             string   `json:"accountType"`
	RecentUsernames         []string `json:"recentUsernames"`
	CanChangeAvatar         bool     `json:"canChangeAvatar"`
}

type SettingsSnapshot struct {
	ActiveTab                    string              `json:"activeTab"`
	MemoryMb                     int                 `json:"memoryMb"`
	DisplayedMemoryMb            int                 `json:"displayedMemoryMb"`
	MinimumMemoryMb              int                 `json:"minimumMemoryMb"`
	MaximumMemoryMb              int                 `json:"maximumMemoryMb"`
	JavaMemoryMb                 int                 `json:"javaMemoryMb"`
	JavaPath                     string              `json:"javaPath"`
	EffectiveJavaPath            string              `json:"effectiveJavaPath"`
	JvmArgs                      string              `json:"jvmArgs"`
	ExtraJvmArgs                 string              `json:"extraJvmArgs"`
	ShowJvmArgs                  bool                `json:"showJvmArgs"`
	AutoOptimizeMemory           bool                `json:"autoOptimizeMemory"`
	AutoMinimizeOnLaunch         bool                `json:"autoMinimizeOnLaunch"`
	AutoCloseOnLaunch            bool                `json:"autoCloseOnLaunch"`
	RestoreLauncherAfterGameExit bool                `json:"restoreLauncherAfterGameExit"`
	ClickSoundEnabled            bool                `json:"clickSoundEnabled"`
	JavaRuntimeMode              string              `json:"javaRuntimeMode"`
	JavaModeHint                 string              `json:"javaModeHint"`
	AutoMemoryHint               string              `json:"autoMemoryHint"`
	AutoMinimizeHint             string              `json:"autoMinimizeHint"`
	RestoreHint                  string              `json:"restoreHint"`
	JvmArgsHint                  string              `json:"jvmArgsHint"`
	MinecraftLanguageCode        string              `json:"minecraftLanguageCode"`
	DisplayedGameDirectory       string              `json:"displayedGameDirectory"`
	LanguageOptions              []map[string]string `json:"languageOptions"`
	LoginFormPlacementId         string              `json:"loginFormPlacementId"`
	LoginPlacementOptions        []map[string]string `json:"loginPlacementOptions"`
	LauncherDirectoryViewId      string              `json:"launcherDirectoryViewId"`
	DirectoryViewOptions         []map[string]string `json:"directoryViewOptions"`
	JavaRuntimeOptions           []map[string]string `json:"javaRuntimeOptions"`
}

type SkinSnapshot struct {
	CurrentSkinUrl     string `json:"currentSkinUrl"`
	SelectedSkinUrl    string `json:"selectedSkinUrl"`
	SelectedSkinIsSlim bool   `json:"selectedSkinIsSlim"`
	ModelPreferenceId  string `json:"modelPreferenceId"`
	SelectedSkinLabel  string `json:"selectedSkinLabel"`
}

type BackgroundItem struct {
	FileName string `json:"fileName"`
	Label    string `json:"label"`
	Url      string `json:"url"`
	IsActive bool   `json:"isActive"`
}

type BackgroundSnapshot struct {
	CurrentBgUrl         string           `json:"currentBgUrl"`
	AppliedBackgroundUrl string           `json:"appliedBackgroundUrl"`
	Items                []BackgroundItem `json:"items"`
}

type ModItem struct {
	ProjectId    string   `json:"projectId"`
	DisplayName  string   `json:"displayName"`
	FileName     string   `json:"fileName"`
	Version      string   `json:"version"`
	Author       string   `json:"author"`
	Description  string   `json:"description"`
	IconUrl      string   `json:"iconUrl"`
	Downloads    int64    `json:"downloads"`
	Followers    int64    `json:"followers"`
	ContentKind  string   `json:"contentKind"`
	Categories   []string `json:"categories"`
	GameVersions []string `json:"gameVersions"`
	Loaders      []string `json:"loaders"`
	IsEnabled    bool     `json:"isEnabled"`
	IsInstalled  bool     `json:"isInstalled"`
	CanInstall   bool     `json:"canInstall"`
	ActionText   string   `json:"actionText"`
}

type ModsSnapshot struct {
	Provider         string    `json:"provider"`
	Items            []ModItem `json:"items"`
	SearchQuery      string    `json:"searchQuery"`
	SelectedCategory string    `json:"selectedCategory"`
	IsCatalogLoading bool      `json:"isCatalogLoading"`
	CatalogSummary   string    `json:"catalogSummary"`
}

type FriendEntry struct {
	ID        string `json:"id"`
	Nickname  string `json:"nickname"`
	Username  string `json:"username"`
	AvatarUrl string `json:"avatarUrl"`
	Status    string `json:"status"`
	GameInfo  string `json:"gameInfo"`
	IsOnline  bool   `json:"isOnline"`
	Server    string `json:"server"`
}

type FriendRequest struct {
	ID        string `json:"id"`
	FromUser  string `json:"fromUser"`
	Sender    string `json:"sender"`
	Username  string `json:"username"`
	AvatarUrl string `json:"avatarUrl"`
	CreatedAt string `json:"createdAt"`
	SentTime  string `json:"sentTime"`
}

type FriendsSnapshot struct {
	ProfileAvatarUrl         string          `json:"profileAvatarUrl"`
	ProfileAvatarPlaceholder string          `json:"profileAvatarPlaceholder"`
	ProfileNickname          string          `json:"profileNickname"`
	ProfileType              string          `json:"profileType"`
	CanManage                bool            `json:"canManage"`
	Friends                  []FriendEntry   `json:"friends"`
	IncomingRequests         []FriendRequest `json:"incomingRequests"`
}

type WindowState struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pkg/browser"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"vesper-launcher/internal/account"
	"vesper-launcher/internal/auth"
	"vesper-launcher/internal/minecraft"
	"vesper-launcher/internal/models"
	"vesper-launcher/internal/mods"
	"vesper-launcher/internal/platform"
	"vesper-launcher/internal/state"
	"vesper-launcher/internal/updater"
)

type BridgeService struct {
	ctx        context.Context
	state      *state.LauncherState
	account    *account.VesperAccountService
	msaAuth    *auth.MicrosoftAuthService
	modsClient *mods.ModrinthClient
	updaterSvc *updater.UpdateService
}

func NewBridgeService(s *state.LauncherState) *BridgeService {
	b := &BridgeService{
		state:      s,
		msaAuth:    auth.NewMicrosoftAuthService(),
		modsClient: mods.NewModrinthClient(),
		updaterSvc: updater.NewUpdateService("1.0.65"),
	}

	b.account = account.NewVesperAccountService(func(nick, avatar string, friends []models.FriendEntry, requests []models.FriendRequest) {
		b.state.SetFriends(friends, requests)
		if nick != "" {
			b.state.SetAuth(true, "vesper", nick, avatar)
		}
		b.EmitSnapshot()
	})

	return b
}

func (b *BridgeService) SetContext(ctx context.Context) {
	b.ctx = ctx
	go b.account.StartBackgroundLoop(ctx)
	go b.initBackgroundTasks()
}

func (b *BridgeService) initBackgroundTasks() {
	// 1. Initial version catalog discovery
	b.refreshVersionCatalog()

	// 2. Initial mods search
	b.searchMods("", "Моды", "modrinth")

	// 3. Initial update check
	if up, err := b.updaterSvc.CheckUpdate(); err == nil && up != nil {
		b.state.SetUpdates(*up)
	}

	b.EmitSnapshot()
}

func (b *BridgeService) refreshVersionCatalog() {
	mcDir := platform.GetMinecraftDir()
	versionsDir := filepath.Join(mcDir, "versions")
	installedMap := make(map[string]bool)

	if entries, err := os.ReadDir(versionsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				jsonPath := filepath.Join(versionsDir, e.Name(), e.Name()+".json")
				if _, errJ := os.Stat(jsonPath); err == nil || errJ == nil {
					installedMap[strings.ToLower(e.Name())] = true
				}
			}
		}
	}

	// Always mark bundled as installed
	installedMap["forgeoptifine 1.16.5"] = true

	var catalog []models.VersionOption
	// Add bundled / popular presets
	catalog = append(catalog, models.VersionOption{
		Key:         "ForgeOptiFine 1.16.5",
		DisplayName: "1.16.5 - Forge + OptiFine (Встроенная)",
		Category:    "forge",
		IsInstalled: true,
	})

	// Add popular releases
	popularReleases := []string{"1.21.4", "1.21.1", "1.20.4", "1.20.1", "1.19.4", "1.18.2", "1.16.5", "1.12.2", "1.8.9"}
	for _, rel := range popularReleases {
		if rel == "1.16.5" {
			continue // covered by ForgeOptiFine
		}
		isInst := installedMap[strings.ToLower(rel)]
		catalog = append(catalog, models.VersionOption{
			Key:         rel,
			DisplayName: rel + " - Ванильная",
			Category:    "release",
			IsInstalled: isInst,
		})
	}

	// Also add any other installed custom versions
	if entries, err := os.ReadDir(versionsDir); err == nil {
		for _, e := range entries {
			name := e.Name()
			if strings.EqualFold(name, "ForgeOptiFine 1.16.5") {
				continue
			}
			isDup := false
			for _, c := range catalog {
				if strings.EqualFold(c.Key, name) {
					isDup = true
					break
				}
			}
			if !isDup {
				catalog = append(catalog, models.VersionOption{
					Key:         name,
					DisplayName: name + " (Локальная)",
					Category:    "custom",
					IsInstalled: true,
				})
			}
		}
	}

	b.state.SetVersionCatalog(catalog)
}

func (b *BridgeService) searchMods(query, category, provider string) {
	b.state.SetMods(b.state.ModItems, query, category, provider, true)
	b.EmitSnapshot()

	go func() {
		defer func() {
			b.state.SetMods(b.state.ModItems, query, category, provider, false)
			b.EmitSnapshot()
		}()

		items, err := b.modsClient.Search(query, category, b.state.SelectedVersion, "", 30)
		if err == nil && len(items) > 0 {
			b.state.SetMods(items, query, category, provider, false)
		}
	}()
}

func (b *BridgeService) EmitSnapshot() {
	if b.ctx == nil {
		return
	}
	snap := b.state.GetSnapshot()
	envelope := map[string]any{
		"type": "snapshot",
		"data": snap,
	}
	wailsRuntime.EventsEmit(b.ctx, "vesper:message", envelope)
	wailsRuntime.EventsEmit(b.ctx, "snapshot", snap)
}

// HandleCommand processes commands sent from the React UI.
func (b *BridgeService) HandleCommand(command string, payload map[string]any) error {
	norm := strings.TrimSpace(strings.ToLower(command))

	switch norm {
	case "host.requestsnapshot", "bridge.requestsnapshot":
		b.EmitSnapshot()

	case "host.windowsize":
		if b.ctx != nil {
			if wailsRuntime.WindowIsMaximised(b.ctx) || wailsRuntime.WindowIsFullscreen(b.ctx) {
				return nil
			}
		}
		w, _ := payload["width"].(float64)
		h, _ := payload["height"].(float64)
		intW := int(w)
		intH := int(h)
		if intW >= state.MinWindowWidth && intW <= state.MaxWindowWidth &&
			intH >= state.MinWindowHeight && intH <= state.MaxWindowHeight {
			b.state.WindowWidth = intW
			b.state.WindowHeight = intH
			b.state.Save()
		}

	case "main.selectversionkey":
		if key, ok := payload["key"].(string); ok && key != "" {
			b.state.SetSelectedVersion(key)
			b.EmitSnapshot()
		}

	case "main.openprofilefolder":
		mcDir := platform.GetMinecraftDir()
		openFolderInExplorer(mcDir)

	case "shell.openexternalurl":
		if url, ok := payload["url"].(string); ok && url != "" {
			_ = browser.OpenURL(url)
		}

	case "account.submit":
		username, _ := payload["username"].(string)
		password, _ := payload["password"].(string)
		mode, _ := payload["mode"].(string)

		if username == "" {
			return nil
		}

		if mode == "register" && password != "" {
			go func() {
				b.state.SetStatus("Регистрация в Vesper...", "", 0, true)
				b.EmitSnapshot()
				if err := b.account.Register(username, password); err != nil {
					b.state.SetStatus(err.Error(), "Проверьте введённые данные.", 0, false)
				} else {
					b.state.SetNickname(username)
					b.state.SetAuth(true, "vesper", username, "")
					b.state.SetSection("none")
					b.state.SetStatus("Аккаунт создан: "+username, "Аккаунт Vesper подключен.", 0, false)
				}
				b.EmitSnapshot()
			}()
		} else if mode == "login" && password != "" {
			go func() {
				b.state.SetStatus("Вход в аккаунт Vesper...", "", 0, true)
				b.EmitSnapshot()
				if err := b.account.Login(username, password); err != nil {
					b.state.SetStatus(err.Error(), "Проверьте ник, пароль или доступность сервера.", 0, false)
				} else {
					b.state.SetNickname(username)
					b.state.SetAuth(true, "vesper", username, "")
					b.state.SetSection("none")
					b.state.SetStatus("Вход выполнен: "+username, "Аккаунт Vesper подключен.", 0, false)
				}
				b.EmitSnapshot()
			}()
		} else {
			// Offline mode
			b.state.SetNickname(username)
			b.state.SetAuth(false, "offline", username, "")
			b.state.SetSection("none")
			b.state.SetStatus("Офлайн режим: "+username, "", 0, false)
			b.EmitSnapshot()
		}

	case "account.logout":
		b.account.Logout()
		b.state.SetNickname("")
		b.state.SetAuth(false, "offline", "", "")
		b.state.SetStatus("Вы вышли из аккаунта", "Сессия сброшена.", 0, false)
		b.EmitSnapshot()

	case "account.microsoft":
		go func() {
			dcr, err := b.msaAuth.RequestDeviceCode()
			if err != nil {
				b.state.SetStatus("Ошибка авторизации Microsoft: "+err.Error(), "", 0, false)
				b.EmitSnapshot()
				return
			}

			_ = browser.OpenURL(dcr.VerificationURI)
			b.state.SetStatus(fmt.Sprintf("Код: %s (открыт браузер)", dcr.UserCode), "", 0, true)
			b.EmitSnapshot()

			ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			sess, err := b.msaAuth.PollAndAuthenticate(ctxTimeout, dcr)
			if err != nil {
				b.state.SetStatus("Microsoft вход не удался: "+err.Error(), "", 0, false)
			} else {
				b.state.SetNickname(sess.Username)
				b.state.SetAuth(true, "microsoft", sess.Username, sess.SkinURL)
				b.state.SetSection("none")
				b.state.SetStatus(fmt.Sprintf("Добро пожаловать, %s!", sess.Username), "", 0, false)
			}
			b.EmitSnapshot()
		}()

	case "shell.opensection":
		if sec, ok := payload["section"].(string); ok && sec != "" {
			b.state.SetSection(sec)
		}
		if tab, ok := payload["tabId"].(string); ok && tab != "" {
			b.state.ActiveSettingsTab = tab
		}
		b.EmitSnapshot()

	case "shell.closesection":
		b.state.SetSection("none")
		b.EmitSnapshot()

	case "shell.navigate":
		if sec, ok := payload["section"].(string); ok {
			b.state.SetSection(sec)
			b.EmitSnapshot()
		}

	case "settings.selecttab":
		b.state.SetSection("settings")
		if tab, ok := payload["tabId"].(string); ok && tab != "" {
			b.state.ActiveSettingsTab = tab
		}
		b.EmitSnapshot()

	case "settings.setmemory":
		if val, ok := payload["value"].(float64); ok && val > 0 {
			b.state.JavaMemoryMb = int(val)
			b.state.Save()
			b.EmitSnapshot()
		}

	case "settings.settoggle":
		field, _ := payload["field"].(string)
		val, _ := payload["value"].(bool)
		switch field {
		case "autoMinimizeOnLaunch", "autoCloseOnLaunch":
			b.state.AutoClose = val
		}
		b.state.Save()
		b.EmitSnapshot()

	case "settings.settext":
		field, _ := payload["field"].(string)
		val, _ := payload["value"].(string)
		switch field {
		case "javaPath":
			b.state.JavaPath = val
		case "extraJvmArgs", "jvmArgs":
			b.state.JvmArgs = val
		}
		b.state.Save()
		b.EmitSnapshot()

	case "settings.setoption":
		field, _ := payload["field"].(string)
		val, _ := payload["value"].(string)
		switch field {
		case "minecraftLanguageCode":
			b.state.LanguageCode = val
		case "loginFormPlacementId":
			b.state.LoginFormPlacementId = val
		case "launcherDirectoryViewId":
			b.state.LauncherDirectoryViewId = val
		case "javaRuntimeMode":
			b.state.JavaRuntimeMode = val
		}
		b.state.Save()
		b.EmitSnapshot()

	case "settings.opengamedirectory":
		mcDir := platform.GetMinecraftDir()
		openFolderInExplorer(mcDir)

	case "skin.importdialog":
		go func() {
			if b.ctx == nil {
				return
			}
			filePath, err := wailsRuntime.OpenFileDialog(b.ctx, wailsRuntime.OpenDialogOptions{
				Title: "Выберите скин (.png)",
				Filters: []wailsRuntime.FileFilter{
					{DisplayName: "Изображение скина (*.png)", Pattern: "*.png"},
				},
			})
			if err == nil && filePath != "" {
				b.state.CurrentSkinUrl = filePath
				b.state.Save()
				b.EmitSnapshot()
			}
		}()

	case "skin.openfolder":
		skinsDir := filepath.Join(platform.GetMinecraftDir(), "skins")
		_ = os.MkdirAll(skinsDir, 0755)
		openFolderInExplorer(skinsDir)

	case "skin.clear":
		b.state.CurrentSkinUrl = "/assets/steve.png"
		b.state.Save()
		b.EmitSnapshot()

	case "skin.setmodel":
		if m, ok := payload["modelId"].(string); ok && m != "" {
			b.state.SkinModel = m
			b.state.Save()
			b.EmitSnapshot()
		}

	case "skin.selectfile":
		if fn, ok := payload["fileName"].(string); ok && fn != "" {
			b.state.CurrentSkinUrl = fn
			b.state.Save()
			b.EmitSnapshot()
		}

	case "background.openfolder":
		openFolderInExplorer(platform.GetMinecraftDir())

	case "background.reset":
		b.state.SetBackground("/assets/bg-day.png")
		b.EmitSnapshot()

	case "background.setpreset":
		if fn, ok := payload["fileName"].(string); ok && fn != "" {
			b.state.SetBackground("/assets/" + fn)
			b.EmitSnapshot()
		}

	case "account.setmode":
		b.state.SetSection("account")
		b.EmitSnapshot()

	case "account.selectrecentusername":
		if u, ok := payload["username"].(string); ok && u != "" {
			b.state.SetNickname(u)
			b.EmitSnapshot()
		}

	case "account.pickavatar":
		go func() {
			if b.ctx == nil {
				return
			}
			filePath, err := wailsRuntime.OpenFileDialog(b.ctx, wailsRuntime.OpenDialogOptions{
				Title: "Выберите аватар (*.png, *.jpg)",
				Filters: []wailsRuntime.FileFilter{
					{DisplayName: "Изображение (*.png;*.jpg;*.webp)", Pattern: "*.png;*.jpg;*.jpeg;*.webp"},
				},
			})
			if err == nil && filePath != "" {
				b.state.AvatarUrl = filePath
				b.state.Save()
				b.EmitSnapshot()
			}
		}()

	case "friends.setnickname":
		// Friend search/draft
		b.EmitSnapshot()

	case "friends.add":
		b.state.SetStatus("Запрос в друзья отправлен", "", 0, false)
		b.EmitSnapshot()

	case "friends.respond":
		b.EmitSnapshot()

	case "friends.remove":
		b.EmitSnapshot()

	case "mods.toggleitem":
		if pid, ok := payload["projectId"].(string); ok && pid != "" {
			for i, it := range b.state.ModItems {
				if it.ProjectId == pid {
					b.state.ModItems[i].IsInstalled = !it.IsInstalled
					if b.state.ModItems[i].IsInstalled {
						b.state.ModItems[i].ActionText = "Удалить"
					} else {
						b.state.ModItems[i].ActionText = "Установить"
					}
					break
				}
			}
			b.EmitSnapshot()
		}

	case "mods.togglefavorite":
		b.EmitSnapshot()

	case "mods.setselectedprojects":
		b.EmitSnapshot()

	case "client.uiready":
		b.EmitSnapshot()

	case "updates.install":
		_ = browser.OpenURL("https://github.com/wertyy111/Updates-Vesper/releases/latest")

	case "settings.update":
		if mem, ok := payload["javaMemoryMb"].(float64); ok && mem > 0 {
			b.state.JavaMemoryMb = int(mem)
		}
		if p, ok := payload["javaPath"].(string); ok {
			b.state.JavaPath = p
		}
		if args, ok := payload["jvmArgs"].(string); ok {
			b.state.JvmArgs = args
		}
		b.state.Save()
		b.EmitSnapshot()

	case "background.select":
		if bg, ok := payload["bgUrl"].(string); ok && bg != "" {
			b.state.SetBackground(bg)
			b.EmitSnapshot()
		}

	case "mods.setsearch":
		if q, ok := payload["value"].(string); ok {
			b.searchMods(q, b.state.SelectedCategory, b.state.ModProvider)
		}

	case "mods.selectcategory":
		if cat, ok := payload["category"].(string); ok {
			b.searchMods(b.state.SearchQuery, cat, b.state.ModProvider)
		}

	case "mods.setprovider":
		if prov, ok := payload["value"].(string); ok {
			b.searchMods(b.state.SearchQuery, b.state.SelectedCategory, prov)
		}

	case "updates.check":
		go func() {
			if up, err := b.updaterSvc.CheckUpdate(); err == nil && up != nil {
				b.state.SetUpdates(*up)
				b.EmitSnapshot()
			}
		}()

	case "main.launch":
		b.handleLaunch()

	case "host.close":
		if b.ctx != nil {
			wailsRuntime.Quit(b.ctx)
		}

	case "host.minimize":
		if b.ctx != nil {
			wailsRuntime.WindowMinimise(b.ctx)
		}

	default:
		// Unknown or benign command
	}

	return nil
}

func (b *BridgeService) handleLaunch() {
	b.state.SetBusy(true)
	b.state.SetStatus("Подготовка к запуску...", "", 10, true)
	b.EmitSnapshot()

	go func() {
		nick := strings.TrimSpace(b.state.Nickname)
		if nick == "" {
			nick = "Player"
		}
		opts := minecraft.LaunchOptions{
			VersionID:  b.state.SelectedVersion,
			Nickname:   nick,
			MemoryMb:   b.state.JavaMemoryMb,
			CustomJava: b.state.JavaPath,
			CustomJvm:  b.state.JvmArgs,
			OnProgress: func(percent float64, message string) {
				b.state.SetStatus("Загрузка...", message, percent, false)
				b.EmitSnapshot()
			},
			OnGameStart: func() {
				b.state.SetBusy(false)
				b.state.SetGameRunning(true)
				b.refreshVersionCatalog()
				b.EmitSnapshot()
			},
			OnGameFinish: func(exitCode int) {
				b.state.SetGameRunning(false)
				b.state.SetBusy(false)
				if exitCode != 0 {
					b.state.SetStatus(fmt.Sprintf("Игра завершилась с ошибкой (код %d)", exitCode), "", 0, false)
				} else {
					b.state.SetStatus("Готов к запуску", "", 0, false)
				}
				b.refreshVersionCatalog()
				b.EmitSnapshot()
			},
		}

		_, err := minecraft.LaunchGame(opts)
		if err != nil {
			b.state.SetBusy(false)
			b.state.SetStatus("Ошибка запуска: "+err.Error(), "", 0, false)
			b.EmitSnapshot()
		}
	}()
}

func openFolderInExplorer(path string) {
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("explorer", path).Start()
	case "darwin":
		_ = exec.Command("open", path).Start()
	default:
		_ = exec.Command("xdg-open", path).Start()
	}
}

// ExecuteRawCommand parses a JSON command message from the bridge.
func (b *BridgeService) ExecuteRawCommand(messageJson string) {
	var envelope struct {
		Type    string         `json:"type"`
		Command string         `json:"command"`
		Payload map[string]any `json:"payload"`
	}

	if err := json.Unmarshal([]byte(messageJson), &envelope); err == nil {
		_ = b.HandleCommand(envelope.Command, envelope.Payload)
	}
}

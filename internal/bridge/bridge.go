package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func (b *BridgeService) buildVersionCatalog(manifest *minecraft.VersionManifestV2) []models.VersionOption {
	mcDir := platform.GetMinecraftDir()
	versionsDir := filepath.Join(mcDir, "versions")
	installedMap := make(map[string]bool)

	if entries, err := os.ReadDir(versionsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				jsonPath := filepath.Join(versionsDir, e.Name(), e.Name()+".json")
				if _, errJ := os.Stat(jsonPath); errJ == nil {
					installedMap[strings.ToLower(e.Name())] = true
				}
			}
		}
	}

	var catalog []models.VersionOption
	seen := make(map[string]bool)

	addOption := func(opt models.VersionOption) {
		lower := strings.ToLower(opt.Key)
		if seen[lower] {
			return
		}
		seen[lower] = true

		isInstalled := false
		if installedMap[lower] {
			isInstalled = true
		} else if strings.HasPrefix(lower, "forge-") {
			mcVer := strings.TrimPrefix(lower, "forge-")
			for k := range installedMap {
				if strings.Contains(k, "forge") && strings.Contains(k, mcVer) {
					isInstalled = true
					break
				}
			}
		} else if strings.HasPrefix(lower, "fabric-") {
			mcVer := strings.TrimPrefix(lower, "fabric-")
			for k := range installedMap {
				if (strings.Contains(k, "fabric") || strings.HasPrefix(k, "fabric-loader")) && strings.Contains(k, mcVer) {
					isInstalled = true
					break
				}
			}
		} else if strings.HasPrefix(lower, "neoforge-") {
			mcVer := strings.TrimPrefix(lower, "neoforge-")
			for k := range installedMap {
				if strings.Contains(k, "neoforge") && strings.Contains(k, mcVer) {
					isInstalled = true
					break
				}
			}
		}
		opt.IsInstalled = isInstalled
		catalog = append(catalog, opt)
	}

	// 1. Mod loaders (Fabric, Forge, NeoForge, OptiFine) for popular releases
	type loaderSpec struct {
		prefix   string
		name     string
		category string
	}

	targetReleases := []struct {
		mcVersion string
		loaders   []loaderSpec
	}{
		{
			mcVersion: "1.21.4",
			loaders: []loaderSpec{
				{prefix: "fabric", name: "Fabric", category: "fabric"},
				{prefix: "neoforge", name: "NeoForge", category: "neoforge"},
			},
		},
		{
			mcVersion: "1.21.1",
			loaders: []loaderSpec{
				{prefix: "fabric", name: "Fabric", category: "fabric"},
				{prefix: "neoforge", name: "NeoForge", category: "neoforge"},
				{prefix: "forge", name: "Forge", category: "forge"},
			},
		},
		{
			mcVersion: "1.20.4",
			loaders: []loaderSpec{
				{prefix: "fabric", name: "Fabric", category: "fabric"},
				{prefix: "neoforge", name: "NeoForge", category: "neoforge"},
				{prefix: "forge", name: "Forge", category: "forge"},
				{prefix: "optifine", name: "OptiFine", category: "optifine"},
			},
		},
		{
			mcVersion: "1.20.1",
			loaders: []loaderSpec{
				{prefix: "fabric", name: "Fabric", category: "fabric"},
				{prefix: "forge", name: "Forge", category: "forge"},
				{prefix: "neoforge", name: "NeoForge", category: "neoforge"},
				{prefix: "optifine", name: "OptiFine", category: "optifine"},
			},
		},
		{
			mcVersion: "1.19.4",
			loaders: []loaderSpec{
				{prefix: "fabric", name: "Fabric", category: "fabric"},
				{prefix: "forge", name: "Forge", category: "forge"},
				{prefix: "optifine", name: "OptiFine", category: "optifine"},
			},
		},
		{
			mcVersion: "1.19.2",
			loaders: []loaderSpec{
				{prefix: "fabric", name: "Fabric", category: "fabric"},
				{prefix: "forge", name: "Forge", category: "forge"},
				{prefix: "optifine", name: "OptiFine", category: "optifine"},
			},
		},
		{
			mcVersion: "1.18.2",
			loaders: []loaderSpec{
				{prefix: "fabric", name: "Fabric", category: "fabric"},
				{prefix: "forge", name: "Forge", category: "forge"},
				{prefix: "optifine", name: "OptiFine", category: "optifine"},
			},
		},
		{
			mcVersion: "1.16.5",
			loaders: []loaderSpec{
				{prefix: "fabric", name: "Fabric", category: "fabric"},
				{prefix: "forge", name: "Forge", category: "forge"},
				{prefix: "optifine", name: "OptiFine", category: "optifine"},
			},
		},
		{
			mcVersion: "1.12.2",
			loaders: []loaderSpec{
				{prefix: "forge", name: "Forge", category: "forge"},
				{prefix: "optifine", name: "OptiFine", category: "optifine"},
			},
		},
		{
			mcVersion: "1.8.9",
			loaders: []loaderSpec{
				{prefix: "forge", name: "Forge", category: "forge"},
				{prefix: "optifine", name: "OptiFine", category: "optifine"},
			},
		},
		{
			mcVersion: "1.7.10",
			loaders: []loaderSpec{
				{prefix: "forge", name: "Forge", category: "forge"},
			},
		},
	}

	for _, tr := range targetReleases {
		// Vanilla release
		addOption(models.VersionOption{
			Key:         tr.mcVersion,
			DisplayName: tr.mcVersion + " - Ванильная",
			Category:    "release",
		})
		// Loaders for this release
		for _, l := range tr.loaders {
			key := fmt.Sprintf("%s-%s", l.prefix, tr.mcVersion)
			addOption(models.VersionOption{
				Key:         key,
				DisplayName: fmt.Sprintf("%s - %s", tr.mcVersion, l.name),
				Category:    l.category,
			})
		}
	}

	// 3. If we have a manifest, add other official releases
	if manifest != nil && len(manifest.Versions) > 0 {
		for _, v := range manifest.Versions {
			if v.Type != "release" {
				continue
			}
			addOption(models.VersionOption{
				Key:         v.ID,
				DisplayName: v.ID + " - Ванильная",
				Category:    "release",
			})
		}
	} else {
		// Popular releases fallback
		popularReleases := []string{"1.21.4", "1.21.3", "1.21.1", "1.20.4", "1.20.1", "1.19.4", "1.18.2", "1.16.5", "1.12.2", "1.8.9"}
		for _, rel := range popularReleases {
			addOption(models.VersionOption{
				Key:         rel,
				DisplayName: rel + " - Ванильная",
				Category:    "release",
			})
		}
	}

	// 4. Also add any other installed custom versions from ~/.minecraft/versions
	if entries, err := os.ReadDir(versionsDir); err == nil {
		for _, e := range entries {
			name := e.Name()
			lower := strings.ToLower(name)
			if !seen[lower] {
				seen[lower] = true
				catalog = append(catalog, models.VersionOption{
					Key:         name,
					DisplayName: name + " (Локальная)",
					Category:    "custom",
					IsInstalled: true,
				})
			}
		}
	}

	return catalog
}

func (b *BridgeService) refreshVersionCatalog() {
	// Try loading cached manifest immediately for instant rendering
	cachePath := filepath.Join(platform.GetMinecraftDir(), "version_manifest_v2.json")
	var initialManifest *minecraft.VersionManifestV2
	if data, err := os.ReadFile(cachePath); err == nil {
		var m minecraft.VersionManifestV2
		if err := json.Unmarshal(data, &m); err == nil && len(m.Versions) > 0 {
			initialManifest = &m
		}
	}

	catalog := b.buildVersionCatalog(initialManifest)
	b.state.SetVersionCatalog(catalog)

	// Fetch fresh manifest from Mojang in background if needed
	go func() {
		manifest, err := minecraft.FetchMojangManifest()
		if err == nil && manifest != nil && len(manifest.Versions) > 0 {
			updated := b.buildVersionCatalog(manifest)
			b.state.SetVersionCatalog(updated)
			b.EmitSnapshot()
		}
	}()
}

func (b *BridgeService) searchMods(query, category, provider string) {
	b.state.SetMods(b.state.ModItems, query, category, provider, true)
	b.EmitSnapshot()

	go func() {
		defer func() {
			b.state.SetMods(b.state.ModItems, query, category, provider, false)
			b.EmitSnapshot()
		}()

		if strings.EqualFold(provider, "curseforge") {
			items := mods.SearchCurseForge(query, category)
			b.state.SetMods(items, query, category, provider, false)
			return
		}

		items, err := b.modsClient.Search(query, category, b.state.SelectedVersion, "", 40)
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
			b.searchMods(b.state.SearchQuery, b.state.SelectedCategory, b.state.ModProvider)
			b.EmitSnapshot()
		}

	case "main.deleteversion":
		if key, ok := payload["key"].(string); ok && key != "" {
			go func() {
				b.deleteMinecraftVersion(key)
				b.refreshVersionCatalog()
				b.EmitSnapshot()
			}()
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

	case "mods.installversion":
		projectId, _ := payload["projectId"].(string)
		urlStr, _ := payload["url"].(string)
		filename, _ := payload["filename"].(string)
		contentKind, _ := payload["contentKind"].(string)

		if urlStr != "" && filename != "" {
			go func() {
				b.state.StatusText = fmt.Sprintf("Скачивание %s...", filename)
				b.EmitSnapshot()

				err := b.downloadModFile(urlStr, filename, contentKind)
				if err != nil {
					b.state.StatusText = fmt.Sprintf("Ошибка скачивания %s: %v", filename, err)
				} else {
					if projectId != "" {
						b.state.SetProjectInstalled(projectId, filename)
					}
					b.state.StatusText = fmt.Sprintf("%s успешно скачан", filename)
				}
				b.EmitSnapshot()
			}()
		}

	case "mods.toggleitem":
		if pid, ok := payload["projectId"].(string); ok && pid != "" {
			go func() {
				installedFile := b.state.GetInstalledProjectFile(pid)
				if installedFile != "" || b.state.IsProjectInstalled(pid) {
					b.state.StatusText = fmt.Sprintf("Удаление %s...", pid)
					b.EmitSnapshot()

					b.deleteModFile(pid, installedFile)
					b.state.RemoveProjectInstalled(pid)
					b.state.StatusText = fmt.Sprintf("%s удалён", pid)
					b.EmitSnapshot()
					return
				}

				// Not installed: resolve best version and download
				b.state.StatusText = fmt.Sprintf("Поиск версии для %s...", pid)
				b.EmitSnapshot()

				var fileUrl, fileName, contentKind string
				if strings.HasPrefix(pid, "cf-") {
					for _, it := range mods.CurseForgePreviewCatalog {
						if it.ProjectId == pid {
							contentKind = it.ContentKind
							switch pid {
							case "cf-jei":
								fileName = "jei-1.20.1-forge-15.2.0.27.jar"
								fileUrl = "https://edge.forgecdn.net/files/4712/100/jei-1.20.1-forge-15.2.0.27.jar"
							case "cf-journeymap":
								fileName = "journeymap-1.20.1-5.9.12-forge.jar"
								fileUrl = "https://edge.forgecdn.net/files/4652/389/journeymap-1.20.1-5.9.12-forge.jar"
							case "cf-bop":
								fileName = "BiomesOPlenty-1.20.1-18.0.0.598.jar"
								fileUrl = "https://edge.forgecdn.net/files/4688/542/BiomesOPlenty-1.20.1-18.0.0.598.jar"
							case "cf-tweaks":
								fileName = "MouseTweaks-2.25-mc1.20.1.jar"
								fileUrl = "https://edge.forgecdn.net/files/4574/819/MouseTweaks-2.25-mc1.20.1.jar"
							case "cf-faithful":
								fileName = "Faithful-32x-1.20.1.zip"
								fileUrl = "https://edge.forgecdn.net/files/4580/112/Faithful-32x-1.20.1.zip"
							case "cf-bsl":
								fileName = "BSL_v8.2.09.zip"
								fileUrl = "https://edge.forgecdn.net/files/4620/711/BSL_v8.2.09.zip"
							case "cf-complimentary":
								fileName = "ComplementaryReimagined_r5.0.1.zip"
								fileUrl = "https://edge.forgecdn.net/files/4650/980/ComplementaryReimagined_r5.0.1.zip"
							case "cf-rlcraft":
								fileName = "RLCraft-1.12.2-v2.9.3.zip"
								fileUrl = "https://edge.forgecdn.net/files/4610/500/RLCraft-1.12.2-v2.9.3.zip"
							}
							break
						}
					}
				} else {
					fName, fUrl, err := b.modsClient.GetProjectBestVersion(pid, b.state.SelectedVersion, "")
					if err == nil && fUrl != "" {
						fileName = fName
						fileUrl = fUrl
					}
					for _, it := range b.state.ModItems {
						if it.ProjectId == pid {
							contentKind = it.ContentKind
							break
						}
					}
				}

				if fileUrl == "" || fileName == "" {
					b.state.StatusText = fmt.Sprintf("Не удалось найти файл для установки %s", pid)
					b.EmitSnapshot()
					return
				}

				b.state.StatusText = fmt.Sprintf("Скачивание %s...", fileName)
				b.EmitSnapshot()

				if err := b.downloadModFile(fileUrl, fileName, contentKind); err != nil {
					b.state.StatusText = fmt.Sprintf("Ошибка скачивания: %v", err)
				} else {
					b.state.SetProjectInstalled(pid, fileName)
					b.state.StatusText = fmt.Sprintf("%s успешно установлен", fileName)
				}
				b.EmitSnapshot()
			}()
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
	isInstalled := b.state.IsVersionInstalled(b.state.SelectedVersion)
	if isInstalled {
		b.state.SetStatus("Подготовка к запуску...", "", 10, true)
	} else {
		b.state.SetStatus("Подготовка к установке...", "", 10, true)
	}
	b.EmitSnapshot()

	go func() {
		nick := strings.TrimSpace(b.state.Nickname)
		if nick == "" {
			nick = "Player"
		}
		opts := minecraft.LaunchOptions{
			VersionID:       b.state.SelectedVersion,
			Nickname:        nick,
			MemoryMb:        b.state.JavaMemoryMb,
			JavaRuntimeMode: b.state.JavaRuntimeMode,
			CustomJava:      b.state.JavaPath,
			CustomJvm:       b.state.JvmArgs,
			OnProgress: func(percent float64, message string) {
				if isInstalled {
					b.state.SetStatus("Загрузка...", message, percent, false)
				} else {
					b.state.SetStatus("Установка...", message, percent, false)
				}
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

func (b *BridgeService) getTargetDirectoryForContent(contentKind string) string {
	mcDir := platform.GetMinecraftDir()
	switch strings.ToLower(contentKind) {
	case "shader", "shaders", "шейдеры":
		return filepath.Join(mcDir, "shaderpacks")
	case "resourcepack", "resourcepacks", "ресурспаки", "ресурс паки":
		return filepath.Join(mcDir, "resourcepacks")
	case "modpack", "modpacks", "сборки":
		return filepath.Join(mcDir, "modpacks")
	default:
		return filepath.Join(mcDir, "mods")
	}
}

func (b *BridgeService) downloadModFile(fileUrl, fileName, contentKind string) error {
	destDir := b.getTargetDirectoryForContent(contentKind)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	cleanFileName := filepath.Base(fileName)
	destPath := filepath.Join(destDir, cleanFileName)

	req, err := http.NewRequest("GET", fileUrl, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "VesperLauncher/1.0 (contact@vesperlauncher.org)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error %s", resp.Status)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func (b *BridgeService) deleteModFile(projectId, fileName string) {
	mcDir := platform.GetMinecraftDir()
	dirs := []string{
		filepath.Join(mcDir, "mods"),
		filepath.Join(mcDir, "resourcepacks"),
		filepath.Join(mcDir, "shaderpacks"),
		filepath.Join(mcDir, "modpacks"),
	}

	cleanTarget := filepath.Base(fileName)
	for _, d := range dirs {
		if cleanTarget != "" && cleanTarget != "." {
			targetPath := filepath.Join(d, cleanTarget)
			if _, err := os.Stat(targetPath); err == nil {
				_ = os.Remove(targetPath)
				return
			}
		}

		if projectId != "" {
			entries, err := os.ReadDir(d)
			if err == nil {
				cleanPid := strings.TrimPrefix(strings.ToLower(projectId), "cf-")
				for _, e := range entries {
					if !e.IsDir() && strings.Contains(strings.ToLower(e.Name()), cleanPid) {
						_ = os.Remove(filepath.Join(d, e.Name()))
						return
					}
				}
			}
		}
	}
}

func (b *BridgeService) deleteMinecraftVersion(key string) {
	mcDir := platform.GetMinecraftDir()
	versionsDir := filepath.Join(mcDir, "versions")

	lowerKey := strings.ToLower(strings.TrimSpace(key))
	if lowerKey == "" {
		return
	}

	// 1. Direct folder match
	exactPath := filepath.Join(versionsDir, key)
	if _, err := os.Stat(exactPath); err == nil {
		_ = os.RemoveAll(exactPath)
	}

	// 2. Scan for case-insensitive matches and modloader naming variations
	entries, err := os.ReadDir(versionsDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			nameLower := strings.ToLower(e.Name())
			if nameLower == lowerKey {
				_ = os.RemoveAll(filepath.Join(versionsDir, e.Name()))
				continue
			}

			if strings.HasPrefix(lowerKey, "forge-") {
				mcVer := strings.TrimPrefix(lowerKey, "forge-")
				if strings.Contains(nameLower, "forge") && strings.Contains(nameLower, mcVer) {
					_ = os.RemoveAll(filepath.Join(versionsDir, e.Name()))
				}
			} else if strings.HasPrefix(lowerKey, "fabric-") {
				mcVer := strings.TrimPrefix(lowerKey, "fabric-")
				if (strings.Contains(nameLower, "fabric") || strings.HasPrefix(nameLower, "fabric-loader")) && strings.Contains(nameLower, mcVer) {
					_ = os.RemoveAll(filepath.Join(versionsDir, e.Name()))
				}
			} else if strings.HasPrefix(lowerKey, "neoforge-") {
				mcVer := strings.TrimPrefix(lowerKey, "neoforge-")
				if strings.Contains(nameLower, "neoforge") && strings.Contains(nameLower, mcVer) {
					_ = os.RemoveAll(filepath.Join(versionsDir, e.Name()))
				}
			} else if strings.HasPrefix(lowerKey, "optifine-") {
				mcVer := strings.TrimPrefix(lowerKey, "optifine-")
				if strings.Contains(nameLower, "optifine") && strings.Contains(nameLower, mcVer) {
					_ = os.RemoveAll(filepath.Join(versionsDir, e.Name()))
				}
			}
		}
	}

	b.state.StatusText = fmt.Sprintf("Версия %s удалена", key)
}

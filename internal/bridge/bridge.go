package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/pkg/browser"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"vesper-launcher/internal/minecraft"
	"vesper-launcher/internal/platform"
	"vesper-launcher/internal/state"
)

type BridgeService struct {
	ctx   context.Context
	state *state.LauncherState
}

func NewBridgeService(s *state.LauncherState) *BridgeService {
	return &BridgeService{
		state: s,
	}
}

func (b *BridgeService) SetContext(ctx context.Context) {
	b.ctx = ctx
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
		if user, ok := payload["username"].(string); ok && user != "" {
			b.state.SetNickname(user)
			b.state.SetSection("none")
			b.EmitSnapshot()
		}

	case "account.logout":
		b.state.SetNickname("Steve")
		b.EmitSnapshot()

	case "shell.navigate":
		if sec, ok := payload["section"].(string); ok {
			b.state.SetSection(sec)
			b.EmitSnapshot()
		}

	case "settings.update":
		// update settings if provided in payload
		b.EmitSnapshot()

	case "background.select":
		if bg, ok := payload["bgUrl"].(string); ok && bg != "" {
			b.state.SetBackground(bg)
			b.EmitSnapshot()
		}

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
		opts := minecraft.LaunchOptions{
			VersionID:  b.state.SelectedVersion,
			Nickname:   b.state.Nickname,
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

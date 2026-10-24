package account

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/pbkdf2"
	"vesper-launcher/internal/models"
	"vesper-launcher/internal/platform"
)

type AccountSyncConfig struct {
	RegisterUrl       string `json:"RegisterUrl"`
	LoginUrl          string `json:"LoginUrl"`
	CredentialInfoUrl string `json:"CredentialInfoUrl"`
	MeUrl             string `json:"MeUrl"`
	LogoutUrl         string `json:"LogoutUrl"`
}

type VesperAccountService struct {
	mu           sync.RWMutex
	client       *http.Client
	config       AccountSyncConfig
	sessionToken string
	nickname     string
	avatarUrl    string
	hasAuth      bool
	onSyncUpdate func(nickname, avatarUrl string, friends []models.FriendEntry, requests []models.FriendRequest)
}

func NewVesperAccountService(onSyncUpdate func(string, string, []models.FriendEntry, []models.FriendRequest)) *VesperAccountService {
	s := &VesperAccountService{
		client:       &http.Client{Timeout: 12 * time.Second},
		onSyncUpdate: onSyncUpdate,
	}
	s.loadConfig()
	s.loadState()
	return s
}

func (s *VesperAccountService) loadConfig() {
	b, err := os.ReadFile("account-sync.json")
	if err == nil {
		_ = json.Unmarshal(b, &s.config)
	}
	if s.config.LoginUrl == "" {
		s.config = AccountSyncConfig{
			RegisterUrl:       "https://vesper-account-api-3516.vesperlauncher3516.workers.dev/api/v1/auth/register",
			LoginUrl:          "https://vesper-account-api-3516.vesperlauncher3516.workers.dev/api/v1/auth/login",
			CredentialInfoUrl: "https://vesper-account-api-3516.vesperlauncher3516.workers.dev/api/v1/auth/credential-info",
			MeUrl:             "https://vesper-account-api-3516.vesperlauncher3516.workers.dev/api/v1/auth/me",
			LogoutUrl:         "https://vesper-account-api-3516.vesperlauncher3516.workers.dev/api/v1/auth/logout",
		}
	}
}

func (s *VesperAccountService) loadState() {
	p := filepath.Join(platform.GetVesperConfigDir(), "vesper-account-state.json")
	b, err := os.ReadFile(p)
	if err != nil {
		return
	}
	var data struct {
		Nickname     string `json:"nickname"`
		SessionToken string `json:"sessionToken"`
		HasAuth      bool   `json:"hasAuth"`
	}
	if err := json.Unmarshal(b, &data); err == nil && data.SessionToken != "" {
		if data.Nickname == "Steve" {
			data.Nickname = ""
		}
		s.nickname = data.Nickname
		s.sessionToken = data.SessionToken
		s.hasAuth = data.HasAuth
	}
}

func (s *VesperAccountService) saveState() {
	p := filepath.Join(platform.GetVesperConfigDir(), "vesper-account-state.json")
	data := map[string]any{
		"nickname":     s.nickname,
		"sessionToken": s.sessionToken,
		"hasAuth":      s.hasAuth,
	}
	b, _ := json.MarshalIndent(data, "", "  ")
	_ = os.WriteFile(p, b, 0600)
}

func (s *VesperAccountService) IsAuthenticated() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hasAuth && s.sessionToken != ""
}

func (s *VesperAccountService) GetNickname() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nickname
}

func (s *VesperAccountService) GetAvatarUrl() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.avatarUrl
}

func extractToken(jsonBytes []byte) string {
	var root map[string]any
	if err := json.Unmarshal(jsonBytes, &root); err != nil {
		return ""
	}
	candidates := []string{"accessToken", "token", "access_token", "sessionToken"}
	for _, k := range candidates {
		if val, ok := root[k].(string); ok && strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val)
		}
	}
	subContainers := []string{"session", "data", "auth", "result", "user"}
	for _, sub := range subContainers {
		if subObj, ok := root[sub].(map[string]any); ok {
			for _, k := range candidates {
				if val, ok := subObj[k].(string); ok && strings.TrimSpace(val) != "" {
					return strings.TrimSpace(val)
				}
			}
		}
	}
	return ""
}

func extractErrorMessage(jsonBytes []byte) string {
	var root map[string]any
	if err := json.Unmarshal(jsonBytes, &root); err != nil {
		return ""
	}
	candidates := []string{"message", "error", "detail"}
	for _, k := range candidates {
		if val, ok := root[k].(string); ok && strings.TrimSpace(val) != "" {
			msg := strings.TrimSpace(val)
			lower := strings.ToLower(msg)
			switch {
			case strings.Contains(lower, "account not found"):
				return "Аккаунт не найден"
			case strings.Contains(lower, "invalid credentials"):
				return "Неверный логин или пароль"
			case strings.Contains(lower, "username and password are required"):
				return "Введите ник и пароль"
			case strings.Contains(lower, "already taken") || strings.Contains(lower, "already exists"):
				return "Этот никнейм уже занят"
			default:
				return msg
			}
		}
	}
	return ""
}

func (s *VesperAccountService) Login(username, password string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("введите ник")
	}
	if password == "" {
		return fmt.Errorf("введите пароль")
	}

	// 1. Fetch credential info (salt and iterations)
	infoUrl := fmt.Sprintf("%s?username=%s", s.config.CredentialInfoUrl, url.QueryEscape(username))
	resp, err := s.client.Get(infoUrl)
	if err != nil {
		return fmt.Errorf("сервер авторизации недоступен")
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("аккаунт '%s' не найден", username)
	}
	if resp.StatusCode != http.StatusOK {
		errMsg := extractErrorMessage(bodyBytes)
		if errMsg == "" {
			errMsg = fmt.Sprintf("код %d", resp.StatusCode)
		}
		return fmt.Errorf("%s", errMsg)
	}

	var credInfo struct {
		PasswordSalt       string `json:"passwordSalt"`
		PasswordIterations int    `json:"passwordIterations"`
	}
	if err := json.Unmarshal(bodyBytes, &credInfo); err != nil || credInfo.PasswordSalt == "" {
		return fmt.Errorf("неверный ответ сервера авторизации")
	}

	saltBytes, err := hex.DecodeString(credInfo.PasswordSalt)
	if err != nil {
		return fmt.Errorf("ошибка формата данных авторизации")
	}

	iters := credInfo.PasswordIterations
	if iters <= 0 {
		iters = 120000
	}

	// Compute PBKDF2 hash
	dk := pbkdf2.Key([]byte(password), saltBytes, iters, 32, sha256.New)
	hashHex := hex.EncodeToString(dk)

	// 2. Submit login
	loginReq := map[string]string{
		"username":     username,
		"passwordHash": hashHex,
	}
	loginBytes, _ := json.Marshal(loginReq)
	loginResp, err := s.client.Post(s.config.LoginUrl, "application/json", bytes.NewReader(loginBytes))
	if err != nil {
		return fmt.Errorf("не удалось отправить запрос на вход")
	}
	defer loginResp.Body.Close()

	loginBody, _ := io.ReadAll(loginResp.Body)
	if loginResp.StatusCode != http.StatusOK {
		errMsg := extractErrorMessage(loginBody)
		if errMsg == "" {
			errMsg = fmt.Sprintf("ошибка входа (код %d)", loginResp.StatusCode)
		}
		return fmt.Errorf("%s", errMsg)
	}

	token := extractToken(loginBody)
	if token == "" {
		return fmt.Errorf("сервер не вернул токен сессии")
	}

	s.mu.Lock()
	s.nickname = username
	s.sessionToken = token
	s.hasAuth = true
	s.saveState()
	s.mu.Unlock()

	go s.RefreshSync(context.Background())
	return nil
}

func (s *VesperAccountService) Register(username, password string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("введите ник")
	}
	if password == "" {
		return fmt.Errorf("введите пароль")
	}

	saltBytes := make([]byte, 16)
	_, _ = rand.Read(saltBytes)
	saltHex := hex.EncodeToString(saltBytes)
	iters := 120000

	dk := pbkdf2.Key([]byte(password), saltBytes, iters, 32, sha256.New)
	hashHex := hex.EncodeToString(dk)

	regReq := map[string]any{
		"username":           username,
		"passwordHash":       hashHex,
		"passwordSalt":       saltHex,
		"passwordIterations": iters,
	}
	regBytes, _ := json.Marshal(regReq)
	regResp, err := s.client.Post(s.config.RegisterUrl, "application/json", bytes.NewReader(regBytes))
	if err != nil {
		return fmt.Errorf("не удалось отправить запрос на регистрацию")
	}
	defer regResp.Body.Close()

	regBody, _ := io.ReadAll(regResp.Body)
	if regResp.StatusCode != http.StatusOK && regResp.StatusCode != http.StatusCreated {
		errMsg := extractErrorMessage(regBody)
		if errMsg == "" {
			errMsg = fmt.Sprintf("ошибка регистрации (код %d)", regResp.StatusCode)
		}
		return fmt.Errorf("%s", errMsg)
	}

	token := extractToken(regBody)
	if token == "" {
		return fmt.Errorf("сервер не вернул токен сессии")
	}

	s.mu.Lock()
	s.nickname = username
	s.sessionToken = token
	s.hasAuth = true
	s.saveState()
	s.mu.Unlock()

	go s.RefreshSync(context.Background())
	return nil
}

func (s *VesperAccountService) Logout() {
	s.mu.Lock()
	token := s.sessionToken
	s.sessionToken = ""
	s.nickname = ""
	s.avatarUrl = ""
	s.hasAuth = false
	s.saveState()
	s.mu.Unlock()

	if token != "" && s.config.LogoutUrl != "" {
		req, _ := http.NewRequest("POST", s.config.LogoutUrl, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		_, _ = s.client.Do(req)
	}
}

func extractAvatarUrl(avatar any) string {
	if avatar == nil {
		return ""
	}
	if str, ok := avatar.(string); ok {
		return strings.TrimSpace(str)
	}
	m, ok := avatar.(map[string]any)
	if !ok {
		return ""
	}
	for _, k := range []string{"url", "imageUrl"} {
		if u, ok := m[k].(string); ok && strings.TrimSpace(u) != "" {
			return strings.TrimSpace(u)
		}
	}
	if b64, ok := m["imageBase64"].(string); ok && strings.TrimSpace(b64) != "" {
		ct := "image/png"
		if c, ok := m["contentType"].(string); ok && strings.TrimSpace(c) != "" {
			ct = strings.TrimSpace(c)
		}
		return fmt.Sprintf("data:%s;base64,%s", ct, strings.TrimSpace(b64))
	}
	return ""
}

func buildInitials(name string) string {
	t := strings.TrimSpace(name)
	if t == "" {
		return "AV"
	}
	if len(t) >= 2 {
		return strings.ToUpper(t[:2])
	}
	return strings.ToUpper(t)
}

func (s *VesperAccountService) RefreshSync(ctx context.Context) {
	s.mu.RLock()
	token := s.sessionToken
	s.mu.RUnlock()

	if token == "" || s.config.MeUrl == "" {
		return
	}

	// 1. Fetch user profile and avatar
	req, err := http.NewRequestWithContext(ctx, "GET", s.config.MeUrl, nil)
	if err == nil {
		req.Header.Set("Authorization", "Bearer "+token)
		resp, errDo := s.client.Do(req)
		if errDo == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				var profileData struct {
					User struct {
						Username string `json:"username"`
					} `json:"user"`
					Avatar any `json:"avatar"`
				}
				if json.Unmarshal(body, &profileData) == nil {
					avatarStr := extractAvatarUrl(profileData.Avatar)
					s.mu.Lock()
					if profileData.User.Username != "" {
						s.nickname = profileData.User.Username
					}
					s.avatarUrl = avatarStr
					s.mu.Unlock()
				}
			}
		}
	}

	// 2. Fetch friends list
	baseMe, errBase := url.Parse(s.config.MeUrl)
	var friendEntries []models.FriendEntry
	var reqEntries []models.FriendRequest
	if errBase == nil {
		baseMe.Path = "/api/v1/friends"
		friendsUrl := baseMe.String()
		fReq, errFReq := http.NewRequestWithContext(ctx, "GET", friendsUrl, nil)
		if errFReq == nil {
			fReq.Header.Set("Authorization", "Bearer "+token)
			fResp, errFDo := s.client.Do(fReq)
			if errFDo == nil {
				defer fResp.Body.Close()
				if fResp.StatusCode == http.StatusOK {
					fBody, _ := io.ReadAll(fResp.Body)
					var fData struct {
						Friends []struct {
							ID           any    `json:"id"`
							Username     string `json:"username"`
							IsOnline     bool   `json:"isOnline"`
							ActivityKind string `json:"activityKind"`
							ActivityName string `json:"activityName"`
							VersionID    string `json:"versionId"`
							Server       string `json:"server"`
							JoinHost     string `json:"joinHost"`
							JoinPort     any    `json:"joinPort"`
							RelayRoomID  string `json:"relayRoomId"`
							Avatar       any    `json:"avatar"`
						} `json:"friends"`
						IncomingRequests []struct {
							ID   any `json:"id"`
							User struct {
								Username string `json:"username"`
								Avatar   any    `json:"avatar"`
							} `json:"user"`
							CreatedAtUtc string `json:"createdAtUtc"`
						} `json:"incomingRequests"`
					}
					if json.Unmarshal(fBody, &fData) == nil {
						for _, f := range fData.Friends {
							u := strings.TrimSpace(f.Username)
							if u == "" {
								u = "Unknown"
							}
							avatarUrl := extractAvatarUrl(f.Avatar)
							placeholder := buildInitials(u)

							presenceText := "Офлайн"
							activityText := ""
							joinAddr := ""
							if f.JoinHost != "" {
								if f.JoinPort != nil {
									joinAddr = fmt.Sprintf("%s:%v", f.JoinHost, f.JoinPort)
								} else {
									joinAddr = f.JoinHost
								}
							}
							if f.IsOnline {
								presenceText = "В сети"
								if f.ActivityKind == "game" {
									presenceText = "В игре"
									if f.ActivityName != "" {
										activityText = "Играет в " + f.ActivityName
									} else {
										activityText = "Играет в Minecraft"
									}
								}
							}

							friendEntries = append(friendEntries, models.FriendEntry{
								ID:                fmt.Sprintf("%v", f.ID),
								Nickname:          u,
								Username:          u,
								AvatarUrl:         avatarUrl,
								AvatarPlaceholder: placeholder,
								IsOnline:          f.IsOnline,
								PresenceText:      presenceText,
								ActivityText:      activityText,
								VersionText:       f.VersionID,
								JoinAddressText:   joinAddr,
								CanConnect:        f.IsOnline && f.ActivityKind == "game",
								Server:            f.Server,
								RelayRoomId:       f.RelayRoomID,
							})
						}
						for _, r := range fData.IncomingRequests {
							u := strings.TrimSpace(r.User.Username)
							if u == "" {
								u = "Unknown"
							}
							avatarUrl := extractAvatarUrl(r.User.Avatar)
							placeholder := buildInitials(u)
							reqEntries = append(reqEntries, models.FriendRequest{
								ID:                fmt.Sprintf("%v", r.ID),
								Sender:            u,
								Username:          u,
								FromUser:          u,
								AvatarUrl:         avatarUrl,
								AvatarPlaceholder: placeholder,
								SentTime:          r.CreatedAtUtc,
								CreatedAt:         r.CreatedAtUtc,
							})
						}
					}
				}
			}
		}
	}

	if s.onSyncUpdate != nil {
		s.mu.RLock()
		nick := s.nickname
		avatar := s.avatarUrl
		s.mu.RUnlock()
		s.onSyncUpdate(nick, avatar, friendEntries, reqEntries)
	}
}

// StartBackgroundLoop runs the periodic profile and friend sync every 15 seconds.
func (s *VesperAccountService) StartBackgroundLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.IsAuthenticated() {
				s.RefreshSync(ctx)
			}
		}
	}
}

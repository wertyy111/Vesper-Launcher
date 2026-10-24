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
	// Look for account-sync.json
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

func (s *VesperAccountService) Login(username, password string) error {
	// 1. Fetch credential info (salt and iterations)
	infoUrl := fmt.Sprintf("%s?username=%s", s.config.CredentialInfoUrl, url.QueryEscape(username))
	resp, err := s.client.Get(infoUrl)
	if err != nil {
		return fmt.Errorf("failed to reach account server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("account not found or server error (%s): %s", resp.Status, string(b))
	}

	var credInfo struct {
		PasswordSalt       string `json:"passwordSalt"`
		PasswordIterations int    `json:"passwordIterations"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&credInfo); err != nil || credInfo.PasswordSalt == "" {
		return fmt.Errorf("invalid credential info response from server")
	}

	saltBytes, err := hex.DecodeString(credInfo.PasswordSalt)
	if err != nil {
		return fmt.Errorf("invalid salt format: %w", err)
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
		return fmt.Errorf("login request failed: %w", err)
	}
	defer loginResp.Body.Close()

	bodyBytes, _ := io.ReadAll(loginResp.Body)
	if loginResp.StatusCode != http.StatusOK {
		var errObj struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		_ = json.Unmarshal(bodyBytes, &errObj)
		msg := errObj.Message
		if msg == "" {
			msg = errObj.Error
		}
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", loginResp.StatusCode)
		}
		return fmt.Errorf("login rejected: %s", msg)
	}

	var tokenObj struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(bodyBytes, &tokenObj)
	if tokenObj.Token == "" {
		return fmt.Errorf("no session token returned")
	}

	s.mu.Lock()
	s.nickname = username
	s.sessionToken = tokenObj.Token
	s.hasAuth = true
	s.saveState()
	s.mu.Unlock()

	go s.RefreshSync(context.Background())
	return nil
}

func (s *VesperAccountService) Register(username, password string) error {
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
		return fmt.Errorf("register request failed: %w", err)
	}
	defer regResp.Body.Close()

	bodyBytes, _ := io.ReadAll(regResp.Body)
	if regResp.StatusCode != http.StatusOK && regResp.StatusCode != http.StatusCreated {
		var errObj struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		_ = json.Unmarshal(bodyBytes, &errObj)
		msg := errObj.Message
		if msg == "" {
			msg = errObj.Error
		}
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", regResp.StatusCode)
		}
		return fmt.Errorf("registration failed: %s", msg)
	}

	var tokenObj struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(bodyBytes, &tokenObj)
	if tokenObj.Token == "" {
		return fmt.Errorf("no session token returned after registration")
	}

	s.mu.Lock()
	s.nickname = username
	s.sessionToken = tokenObj.Token
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
	s.hasAuth = false
	s.saveState()
	s.mu.Unlock()

	if token != "" && s.config.LogoutUrl != "" {
		req, _ := http.NewRequest("POST", s.config.LogoutUrl, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		_, _ = s.client.Do(req)
	}
}

func (s *VesperAccountService) RefreshSync(ctx context.Context) {
	s.mu.RLock()
	token := s.sessionToken
	s.mu.RUnlock()

	if token == "" || s.config.MeUrl == "" {
		return
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", s.config.MeUrl, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var meResp struct {
		User struct {
			Username  string `json:"username"`
			AvatarUrl string `json:"avatarUrl"`
		} `json:"user"`
		Friends []struct {
			ID       string `json:"id"`
			Username string `json:"username"`
			IsOnline bool   `json:"isOnline"`
			Server   string `json:"server"`
		} `json:"friends"`
		FriendRequests []struct {
			ID       string `json:"id"`
			Sender   string `json:"sender"`
			SentTime string `json:"sentTime"`
		} `json:"friendRequests"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&meResp); err != nil {
		return
	}

	s.mu.Lock()
	if meResp.User.AvatarUrl != "" {
		s.avatarUrl = meResp.User.AvatarUrl
	}
	s.mu.Unlock()

	var friendEntries []models.FriendEntry
	for _, f := range meResp.Friends {
		friendEntries = append(friendEntries, models.FriendEntry{
			ID:       f.ID,
			Nickname: f.Username,
			IsOnline: f.IsOnline,
			Server:   f.Server,
		})
	}

	var reqEntries []models.FriendRequest
	for _, r := range meResp.FriendRequests {
		reqEntries = append(reqEntries, models.FriendRequest{
			ID:       r.ID,
			Sender:   r.Sender,
			SentTime: r.SentTime,
		})
	}

	if s.onSyncUpdate != nil {
		s.onSyncUpdate(s.nickname, s.avatarUrl, friendEntries, reqEntries)
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

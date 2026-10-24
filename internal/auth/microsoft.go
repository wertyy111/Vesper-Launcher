package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"vesper-launcher/internal/platform"
)

const (
	AzureClientID = "c36a9fb6-4f2a-41ff-90bd-ae7cc92031eb"
	AzureTenantID = "consumers"
	MsaScope      = "XboxLive.signin offline_access"
)

type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	Message         string `json:"message"`
}

type MicrosoftSession struct {
	Username     string `json:"username"`
	UUID         string `json:"uuid"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	SkinURL      string `json:"skinUrl,omitempty"`
}

type MicrosoftAuthService struct {
	client *http.Client
}

func NewMicrosoftAuthService() *MicrosoftAuthService {
	return &MicrosoftAuthService{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// RequestDeviceCode initiates the Microsoft device authorization flow.
func (s *MicrosoftAuthService) RequestDeviceCode() (*DeviceCodeResponse, error) {
	endpoint := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/devicecode", AzureTenantID)
	data := url.Values{}
	data.Set("client_id", AzureClientID)
	data.Set("scope", MsaScope)

	resp, err := s.client.PostForm(endpoint, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device code request failed: %s (%s)", resp.Status, string(b))
	}

	var dcr DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dcr); err != nil {
		return nil, err
	}

	return &dcr, nil
}

// PollAndAuthenticate waits for the user to enter the code at verification_uri and completes full Xbox/Mojang exchange.
func (s *MicrosoftAuthService) PollAndAuthenticate(ctx context.Context, dcr *DeviceCodeResponse) (*MicrosoftSession, error) {
	interval := time.Duration(dcr.Interval) * time.Second
	if interval < 2*time.Second {
		interval = 5 * time.Second
	}

	tokenEndpoint := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", AzureTenantID)
	data := url.Values{}
	data.Set("client_id", AzureClientID)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	data.Set("device_code", dcr.DeviceCode)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var msaAccessToken, msaRefreshToken string

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			resp, err := s.client.PostForm(tokenEndpoint, data)
			if err != nil {
				continue
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				var tokenResp struct {
					AccessToken  string `json:"access_token"`
					RefreshToken string `json:"refresh_token"`
				}
				if err := json.Unmarshal(body, &tokenResp); err == nil {
					msaAccessToken = tokenResp.AccessToken
					msaRefreshToken = tokenResp.RefreshToken
					break
				}
			}

			var errResp struct {
				Error string `json:"error"`
			}
			_ = json.Unmarshal(body, &errResp)
			if errResp.Error == "authorization_pending" {
				continue
			} else if errResp.Error == "slow_down" {
				time.Sleep(5 * time.Second)
				continue
			} else if errResp.Error != "" {
				return nil, fmt.Errorf("device auth error: %s", errResp.Error)
			}
		}

		if msaAccessToken != "" {
			break
		}
	}

	// Exchange MSA Token for Minecraft Session
	session, err := s.exchangeForMinecraft(msaAccessToken, msaRefreshToken)
	if err != nil {
		return nil, err
	}

	s.SaveSession(session)
	return session, nil
}

func (s *MicrosoftAuthService) exchangeForMinecraft(msaToken, refreshToken string) (*MicrosoftSession, error) {
	// 1. Xbox Live User Authentication
	xblReq := map[string]any{
		"Properties": map[string]any{
			"AuthMethod": "RPS",
			"SiteName":   "user.auth.xboxlive.com",
			"RpsTicket":  "d=" + msaToken,
		},
		"RelyingParty": "http://auth.xboxlive.com",
		"TokenType":    "JWT",
	}
	xblBytes, _ := json.Marshal(xblReq)
	xblResp, err := s.client.Post("https://user.auth.xboxlive.com/user/authenticate", "application/json", bytes.NewReader(xblBytes))
	if err != nil {
		return nil, fmt.Errorf("xbox auth network error: %w", err)
	}
	defer xblResp.Body.Close()

	var xblResult struct {
		Token         string `json:"Token"`
		DisplayClaims struct {
			Xui []struct {
				Uhs string `json:"uhs"`
			} `json:"xui"`
		} `json:"DisplayClaims"`
	}
	if err := json.NewDecoder(xblResp.Body).Decode(&xblResult); err != nil || len(xblResult.DisplayClaims.Xui) == 0 {
		return nil, fmt.Errorf("failed to parse xbox user token")
	}

	uhs := xblResult.DisplayClaims.Xui[0].Uhs
	xblToken := xblResult.Token

	// 2. XSTS Token
	xstsReq := map[string]any{
		"Properties": map[string]any{
			"SandboxId":  "RETAIL",
			"UserTokens": []string{xblToken},
		},
		"RelyingParty": "rp://api.minecraftservices.com/",
		"TokenType":    "JWT",
	}
	xstsBytes, _ := json.Marshal(xstsReq)
	xstsResp, err := s.client.Post("https://xsts.auth.xboxlive.com/xsts/authorize", "application/json", bytes.NewReader(xstsBytes))
	if err != nil {
		return nil, fmt.Errorf("xsts auth network error: %w", err)
	}
	defer xstsResp.Body.Close()

	var xstsResult struct {
		Token string `json:"Token"`
	}
	if err := json.NewDecoder(xstsResp.Body).Decode(&xstsResult); err != nil || xstsResult.Token == "" {
		return nil, fmt.Errorf("failed to obtain xsts token")
	}

	// 3. Minecraft Services Login
	mcLoginReq := map[string]any{
		"identityToken": fmt.Sprintf("XBL3.0 x=%s;%s", uhs, xstsResult.Token),
	}
	mcLoginBytes, _ := json.Marshal(mcLoginReq)
	mcLoginResp, err := s.client.Post("https://api.minecraftservices.com/authentication/login_with_xbox", "application/json", bytes.NewReader(mcLoginBytes))
	if err != nil {
		return nil, fmt.Errorf("mojang services login error: %w", err)
	}
	defer mcLoginResp.Body.Close()

	var mcLoginResult struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(mcLoginResp.Body).Decode(&mcLoginResult); err != nil || mcLoginResult.AccessToken == "" {
		return nil, fmt.Errorf("failed to obtain minecraft access token")
	}

	// 4. Fetch Minecraft Profile
	profileReq, _ := http.NewRequest("GET", "https://api.minecraftservices.com/minecraft/profile", nil)
	profileReq.Header.Set("Authorization", "Bearer "+mcLoginResult.AccessToken)

	profileResp, err := s.client.Do(profileReq)
	if err != nil {
		return nil, fmt.Errorf("profile fetch network error: %w", err)
	}
	defer profileResp.Body.Close()

	var profileResult struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Skins []struct {
			URL   string `json:"url"`
			State string `json:"state"`
		} `json:"skins"`
	}
	if err := json.NewDecoder(profileResp.Body).Decode(&profileResult); err != nil || profileResult.Name == "" {
		return nil, fmt.Errorf("failed to fetch minecraft profile (does this account own minecraft?)")
	}

	skinUrl := ""
	if len(profileResult.Skins) > 0 {
		skinUrl = profileResult.Skins[0].URL
	}

	return &MicrosoftSession{
		Username:     profileResult.Name,
		UUID:         profileResult.ID,
		AccessToken:  mcLoginResult.AccessToken,
		RefreshToken: refreshToken,
		SkinURL:      skinUrl,
	}, nil
}

func (s *MicrosoftAuthService) SaveSession(sess *MicrosoftSession) {
	configDir := platform.GetVesperConfigDir()
	_ = os.MkdirAll(configDir, 0755)
	p := filepath.Join(configDir, "msa-session.json")
	b, err := json.MarshalIndent(sess, "", "  ")
	if err == nil {
		_ = os.WriteFile(p, b, 0600)
	}
}

func (s *MicrosoftAuthService) LoadSession() *MicrosoftSession {
	configDir := platform.GetVesperConfigDir()
	p := filepath.Join(configDir, "msa-session.json")
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var sess MicrosoftSession
	if err := json.Unmarshal(b, &sess); err == nil && sess.Username != "" {
		return &sess
	}
	return nil
}

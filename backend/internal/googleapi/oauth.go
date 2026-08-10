// Package googleapi membungkus interaksi dengan Google OAuth2 dan Google
// Drive. Layer di atasnya (repository/service) tidak pernah menyentuh SDK
// Google secara langsung.
package googleapi

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/oauth2"
	googleoauth "golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// Scope yang diminta saat onboarding. drive.file membatasi akses hanya pada
// berkas yang dibuat aplikasi ini, sehingga aplikasi tidak bisa membaca isi
// Drive pengguna yang lain. Data terstruktur tersimpan di PostgreSQL, jadi
// scope spreadsheet tidak lagi diperlukan.
var Scopes = []string{
	"https://www.googleapis.com/auth/drive.file",
	"openid",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
}

// OAuthManager mengelola alur OAuth2 Google beserta state anti-CSRF.
type OAuthManager struct {
	cfg    *oauth2.Config
	states sync.Map // state -> time.Time (kedaluwarsa)
}

// NewOAuthManager membuat manager dengan kredensial aplikasi.
func NewOAuthManager(clientID, clientSecret, redirectURL string) *OAuthManager {
	return &OAuthManager{
		cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       Scopes,
			Endpoint:     googleoauth.Endpoint,
		},
	}
}

// AuthCodeURL membuat URL consent screen sekaligus mendaftarkan state.
// access_type=offline + prompt=consent memastikan refresh token diterbitkan.
func (m *OAuthManager) AuthCodeURL() (string, string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("membuat state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(buf)
	m.states.Store(state, time.Now().Add(10*time.Minute))
	m.gcStates()

	url := m.cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
		oauth2.SetAuthURLParam("include_granted_scopes", "true"),
	)
	return url, state, nil
}

// ConsumeState memvalidasi state sekali pakai dari callback.
func (m *OAuthManager) ConsumeState(state string) bool {
	if state == "" {
		return false
	}
	v, ok := m.states.LoadAndDelete(state)
	if !ok {
		return false
	}
	exp, ok := v.(time.Time)
	return ok && time.Now().Before(exp)
}

func (m *OAuthManager) gcStates() {
	now := time.Now()
	m.states.Range(func(key, value any) bool {
		if exp, ok := value.(time.Time); ok && now.After(exp) {
			m.states.Delete(key)
		}
		return true
	})
}

// Exchange menukar authorization code menjadi token.
func (m *OAuthManager) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	tok, err := m.cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("menukar authorization code: %w", err)
	}
	return tok, nil
}

// TokenSource membangun token source yang otomatis me-refresh access token
// dari refresh token milik tenant.
func (m *OAuthManager) TokenSource(ctx context.Context, refreshToken string) oauth2.TokenSource {
	return m.cfg.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
}

// UserInfo adalah profil dasar pemilik akun Google.
type UserInfo struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// FetchUserInfo mengambil profil pengguna memakai access token.
func (m *OAuthManager) FetchUserInfo(ctx context.Context, tok *oauth2.Token) (*UserInfo, error) {
	client := m.cfg.Client(ctx, tok)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openidconnect.googleapis.com/v1/userinfo", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mengambil profil Google: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("profil Google membalas status %d", resp.StatusCode)
	}
	var info UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("membaca profil Google: %w", err)
	}
	return &info, nil
}

// ExchangeUser menukar authorization code lalu langsung mengambil profil
// pengguna. Mengembalikan refresh token yang harus disimpan terenkripsi.
func (m *OAuthManager) ExchangeUser(ctx context.Context, code string) (string, *UserInfo, error) {
	tok, err := m.Exchange(ctx, code)
	if err != nil {
		return "", nil, err
	}
	info, err := m.FetchUserInfo(ctx, tok)
	if err != nil {
		return "", nil, err
	}
	return tok.RefreshToken, info, nil
}

// NewDriveService membuat klien Google Drive untuk satu refresh token tenant.
func (m *OAuthManager) NewDriveService(ctx context.Context, refreshToken string) (*drive.Service, error) {
	driveSvc, err := drive.NewService(ctx, option.WithTokenSource(m.TokenSource(ctx, refreshToken)))
	if err != nil {
		return nil, fmt.Errorf("membuat klien Drive: %w", err)
	}
	return driveSvc, nil
}

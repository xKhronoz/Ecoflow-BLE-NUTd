package ecoflow

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/config"
)

type LoginClient struct {
	HTTPClient      *http.Client
	BaseURLByRegion map[string]string
}

type loginPayload struct {
	Scene      string     `json:"scene"`
	AppVersion string     `json:"appVersion"`
	Password   string     `json:"password"`
	OAuth      loginOAuth `json:"oauth"`
	UserType   string     `json:"userType"`
	Email      string     `json:"email,omitempty"`
	Phone      string     `json:"phone,omitempty"`
}

type loginOAuth struct {
	BundleID string `json:"bundleId"`
}

type loginResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    struct {
		User struct {
			UserID string `json:"userId"`
		} `json:"user"`
	} `json:"data"`
}

func (c *LoginClient) ResolveUserID(ctx context.Context, auth config.ProviderAuthConfig) (string, error) {
	if auth.UserID != "" {
		return auth.UserID, nil
	}
	identifier := strings.TrimSpace(auth.Email)
	if identifier == "" || auth.Password == "" {
		return "", fmt.Errorf("provider.auth.user_id or provider.auth.email/password is required")
	}
	isPhone := isPhoneIdentifier(identifier)
	region, err := normalizeRegion(auth.Region, isPhone)
	if err != nil {
		return "", err
	}
	baseURL, err := c.baseURL(region)
	if err != nil {
		return "", err
	}
	payload := loginPayload{
		Scene:      "IOT_APP",
		AppVersion: "1.0.0",
		Password:   base64.StdEncoding.EncodeToString([]byte(auth.Password)),
		OAuth:      loginOAuth{BundleID: "com.ef.EcoFlow"},
		UserType:   "ECOFLOW",
	}
	if isPhone && region == "api-cn" {
		payload.Phone = strings.TrimPrefix(identifier, "+86")
	} else {
		payload.Email = identifier
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/auth/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("login failed with status code %d: %s", resp.StatusCode, resp.Status)
	}
	var parsed loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if parsed.Code != "0" {
		return "", fmt.Errorf("login failed: %q", parsed.Message)
	}
	if parsed.Data.User.UserID == "" {
		return "", fmt.Errorf("login succeeded but user_id was empty")
	}
	return parsed.Data.User.UserID, nil
}

func (c *LoginClient) baseURL(region string) (string, error) {
	if c != nil && c.BaseURLByRegion != nil {
		if baseURL, ok := c.BaseURLByRegion[region]; ok {
			return strings.TrimRight(baseURL, "/"), nil
		}
	}
	switch region {
	case "api", "api-e", "api-a", "api-j", "api-r", "api-cn":
		return "https://" + region + ".ecoflow.com", nil
	default:
		return "", fmt.Errorf("unsupported EcoFlow region %q", region)
	}
}

func normalizeRegion(region string, isPhone bool) (string, error) {
	region = strings.ToLower(strings.TrimSpace(region))
	if region == "" || region == "auto" {
		if isPhone {
			return "api-cn", nil
		}
		return "api", nil
	}
	if region == "api-cn" && !isPhone {
		return "", fmt.Errorf("api-cn requires phone number, not email")
	}
	switch region {
	case "api", "api-e", "api-a", "api-j", "api-r", "api-cn":
		return region, nil
	default:
		return "", fmt.Errorf("unsupported EcoFlow region %q", region)
	}
}

func isPhoneIdentifier(identifier string) bool {
	digits := strings.ReplaceAll(strings.TrimPrefix(identifier, "+"), " ", "")
	if len(digits) < 6 || len(digits) > 15 {
		return false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

package config

import (
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Listen   string      `json:"listen" yaml:"listen"`
	Auth     AuthConfig  `json:"auth" yaml:"auth"`
	Web      WebConfig   `json:"web" yaml:"web"`
	Devices  []Device    `json:"devices" yaml:"devices"`
	Provider ProviderCfg `json:"provider" yaml:"provider"`
}

type AuthConfig struct {
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
}

type WebConfig struct {
	Enable bool   `json:"enable" yaml:"enable"`
	Listen string `json:"listen" yaml:"listen"`
}

type Device struct {
	Name                string `json:"name" yaml:"name"`
	Description         string `json:"description" yaml:"description"`
	Model               string `json:"model" yaml:"model"`
	Serial              string `json:"serial" yaml:"serial"`
	MAC                 string `json:"mac" yaml:"mac"`
	LowBatteryPercent   int    `json:"low_battery_percent" yaml:"low_battery_percent"`
	LowRuntimeSeconds   int    `json:"low_runtime_seconds" yaml:"low_runtime_seconds"`
	StaleTimeoutSeconds int    `json:"stale_timeout_seconds" yaml:"stale_timeout_seconds"`
}

type ProviderCfg struct {
	Type                  string             `json:"type" yaml:"type"` // mock, json-dir, eco-ble
	JSONDir               string             `json:"json_dir" yaml:"json_dir"`
	Adapter               string             `json:"adapter" yaml:"adapter"`
	ScanTimeoutSeconds    int                `json:"scan_timeout_seconds" yaml:"scan_timeout_seconds"`
	ConnectTimeoutSeconds int                `json:"connect_timeout_seconds" yaml:"connect_timeout_seconds"`
	ReconnectDelaySeconds int                `json:"reconnect_delay_seconds" yaml:"reconnect_delay_seconds"`
	PollSeconds           int                `json:"poll_seconds" yaml:"poll_seconds"`
	Auth                  ProviderAuthConfig `json:"auth" yaml:"auth"`
}

type ProviderAuthConfig struct {
	UserID   string `json:"user_id" yaml:"user_id"`
	Email    string `json:"email" yaml:"email"`
	Password string `json:"password" yaml:"password"`
	Region   string `json:"region" yaml:"region"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		if err := json.Unmarshal(b, &c); err != nil {
			return nil, err
		}
	}
	if c.Listen == "" {
		c.Listen = "127.0.0.1:3493"
	}
	if c.Web.Enable && c.Web.Listen == "" {
		c.Web.Listen = "127.0.0.1:8080"
	}
	if c.Provider.Type == "" {
		c.Provider.Type = "mock"
	}
	if c.Provider.Adapter == "" {
		c.Provider.Adapter = "hci0"
	}
	if c.Provider.ScanTimeoutSeconds <= 0 {
		c.Provider.ScanTimeoutSeconds = 8
	}
	if c.Provider.ConnectTimeoutSeconds <= 0 {
		c.Provider.ConnectTimeoutSeconds = 20
	}
	if c.Provider.ReconnectDelaySeconds <= 0 {
		c.Provider.ReconnectDelaySeconds = 10
	}
	if c.Provider.PollSeconds <= 0 {
		c.Provider.PollSeconds = 15
	}
	if c.Provider.Auth.Region == "" {
		c.Provider.Auth.Region = "auto"
	}
	if len(c.Devices) == 0 {
		return nil, fmt.Errorf("at least one device is required")
	}
	switch c.Provider.Type {
	case "mock", "json-dir", "eco-ble":
	default:
		return nil, fmt.Errorf("unsupported provider type %q", c.Provider.Type)
	}
	for i := range c.Devices {
		if c.Devices[i].Name == "" {
			return nil, fmt.Errorf("device %d name is required", i)
		}
		if c.Provider.Type == "eco-ble" && c.Devices[i].MAC == "" {
			return nil, fmt.Errorf("device %d mac is required for provider eco-ble", i)
		}
		if c.Devices[i].LowBatteryPercent <= 0 {
			c.Devices[i].LowBatteryPercent = 20
		}
		if c.Devices[i].LowRuntimeSeconds <= 0 {
			c.Devices[i].LowRuntimeSeconds = 300
		}
		if c.Devices[i].StaleTimeoutSeconds <= 0 {
			c.Devices[i].StaleTimeoutSeconds = 60
		}
	}
	if c.Provider.Type == "eco-ble" {
		if c.Provider.Auth.UserID == "" {
			if c.Provider.Auth.Email == "" || c.Provider.Auth.Password == "" {
				return nil, fmt.Errorf("provider.auth.user_id or provider.auth.email/password is required for provider eco-ble")
			}
		}
	}
	return &c, nil
}

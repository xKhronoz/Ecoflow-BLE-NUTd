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
	Devices  []Device    `json:"devices" yaml:"devices"`
	Provider ProviderCfg `json:"provider" yaml:"provider"`
}

type AuthConfig struct {
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
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
	Type        string `json:"type" yaml:"type"` // mock, json-dir, ble-todo
	JSONDir     string `json:"json_dir" yaml:"json_dir"`
	PollSeconds int    `json:"poll_seconds" yaml:"poll_seconds"`
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
	if c.Provider.Type == "" {
		c.Provider.Type = "mock"
	}
	if c.Provider.PollSeconds <= 0 {
		c.Provider.PollSeconds = 10
	}
	if len(c.Devices) == 0 {
		return nil, fmt.Errorf("at least one device is required")
	}
	for i := range c.Devices {
		if c.Devices[i].Name == "" {
			return nil, fmt.Errorf("device %d name is required", i)
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
	return &c, nil
}

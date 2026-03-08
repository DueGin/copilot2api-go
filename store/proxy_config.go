package store

import (
	"encoding/json"
	"os"
)

type ProxyConfig struct {
	ProxyURL string `json:"proxyURL"`
}

func GetProxyConfig() (ProxyConfig, error) {
	data, err := os.ReadFile(ProxyConfigFile())
	if err != nil {
		if os.IsNotExist(err) {
			return ProxyConfig{}, nil
		}
		return ProxyConfig{}, err
	}
	var cfg ProxyConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ProxyConfig{}, nil
	}
	return cfg, nil
}

func UpdateProxyConfig(cfg ProxyConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ProxyConfigFile(), data, 0644)
}

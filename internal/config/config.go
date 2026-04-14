package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Provider struct {
	Name       string   `json:"name"`
	OpenAIBase string   `json:"openai_base"`
	Models     []string `json:"models"`
}

type Config struct {
	Providers      []Provider `json:"providers"`
	ActiveProvider int        `json:"active_provider"`
	mu             sync.RWMutex
}

var (
	cfg     *Config
	cfgPath string
	once    sync.Once

	ErrNoActiveProvider      = errors.New("请先添加并选择第三方服务商")
	ErrProviderTargetsOpenAI = errors.New("服务商地址不能是 api.openai.com")
	ErrProviderModelsEmpty   = errors.New("请至少为当前服务商配置一个模型 ID")
)

func GetConfigPath() string {
	once.Do(func() {
		exePath, err := os.Executable()
		if err != nil {
			exePath = os.Args[0]
		}
		dir := filepath.Dir(exePath)
		cfgPath = filepath.Join(dir, "config.json")
	})
	return cfgPath
}

func Load() (*Config, error) {
	cfg = &Config{
		Providers:      []Provider{},
		ActiveProvider: 0,
	}

	path := GetConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Save() error {
	path := GetConfigPath()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func GetProviders() []Provider {
	if cfg == nil {
		return []Provider{}
	}
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	return cfg.Providers
}

func GetActiveProvider() *Provider {
	if cfg == nil {
		return nil
	}
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	if cfg.ActiveProvider >= 0 && cfg.ActiveProvider < len(cfg.Providers) {
		return &cfg.Providers[cfg.ActiveProvider]
	}
	return nil
}

func SetActiveProvider(index int) error {
	if cfg == nil {
		return nil
	}
	cfg.mu.Lock()
	defer cfg.mu.Unlock()
	if index >= 0 && index < len(cfg.Providers) {
		cfg.ActiveProvider = index
		return cfg.Save()
	}
	return nil
}

func AddProvider(provider Provider) error {
	if cfg == nil {
		return nil
	}
	if _, err := validateProviderTarget(provider); err != nil {
		return err
	}
	cfg.mu.Lock()
	defer cfg.mu.Unlock()
	cfg.Providers = append(cfg.Providers, provider)
	return cfg.Save()
}

func UpdateProvider(index int, provider Provider) error {
	if cfg == nil {
		return nil
	}
	if _, err := validateProviderTarget(provider); err != nil {
		return err
	}
	cfg.mu.Lock()
	defer cfg.mu.Unlock()
	if index >= 0 && index < len(cfg.Providers) {
		cfg.Providers[index] = provider
		return cfg.Save()
	}
	return nil
}

func DeleteProvider(index int) error {
	if cfg == nil {
		return nil
	}
	cfg.mu.Lock()
	defer cfg.mu.Unlock()
	if index >= 0 && index < len(cfg.Providers) {
		cfg.Providers = append(cfg.Providers[:index], cfg.Providers[index+1:]...)
		if cfg.ActiveProvider >= len(cfg.Providers) {
			cfg.ActiveProvider = 0
		}
		return cfg.Save()
	}
	return nil
}

func GetActiveProviderIndex() int {
	if cfg == nil {
		return 0
	}
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	return cfg.ActiveProvider
}

func GetModels() []string {
	provider := GetActiveProvider()
	if provider == nil {
		return []string{}
	}
	return provider.Models
}

func ValidateActiveProviderTarget() (*url.URL, error) {
	provider := GetActiveProvider()
	if provider == nil {
		return nil, ErrNoActiveProvider
	}

	return validateProviderTarget(*provider)
}

func validateProviderTarget(provider Provider) (*url.URL, error) {
	if strings.TrimSpace(provider.OpenAIBase) == "" {
		return nil, ErrNoActiveProvider
	}

	targetURL, err := url.Parse(strings.TrimSpace(provider.OpenAIBase))
	if err != nil {
		return nil, fmt.Errorf("服务商地址无效：%w", err)
	}

	if targetURL.Scheme == "" || targetURL.Host == "" {
		return nil, fmt.Errorf("服务商地址必须是完整 URL")
	}

	if strings.EqualFold(targetURL.Hostname(), "api.openai.com") {
		return nil, ErrProviderTargetsOpenAI
	}

	hasModel := false
	for _, model := range provider.Models {
		if strings.TrimSpace(model) != "" {
			hasModel = true
			break
		}
	}
	if !hasModel {
		return nil, ErrProviderModelsEmpty
	}

	return targetURL, nil
}

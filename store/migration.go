package store

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
)

// oldPoolConfig mirrors the legacy single-pool configuration format.
type oldPoolConfig struct {
	Enabled      bool   `json:"enabled"`
	Strategy     string `json:"strategy"`
	ApiKey       string `json:"apiKey"`
	RateLimitRPM int    `json:"rateLimitRPM,omitempty"`
}

// MigratePoolConfig migrates the legacy pool-config.json to the new multi-pool pools.json format.
// It is idempotent: if pools.json already has data, it does nothing.
func MigratePoolConfig() error {
	// Check if pools.json already has data.
	pools, err := readPools()
	if err == nil && len(pools) > 0 {
		return nil // Already migrated.
	}

	// Read the old pool-config.json.
	data, err := os.ReadFile(PoolConfigFile())
	if err != nil {
		// No old config, nothing to migrate.
		return nil
	}

	var oldCfg oldPoolConfig
	if err := json.Unmarshal(data, &oldCfg); err != nil {
		return nil
	}

	if !oldCfg.Enabled || oldCfg.ApiKey == "" {
		// Old pool was not enabled, nothing to migrate.
		return nil
	}

	// Read global proxy URL for assignment to the migrated pool.
	proxyURL := ""
	if proxyCfg, err := GetProxyConfig(); err == nil && proxyCfg.ProxyURL != "" {
		proxyURL = proxyCfg.ProxyURL
	}

	// Collect all enabled account IDs.
	accounts, err := GetEnabledAccounts()
	if err != nil {
		accounts = nil
	}
	var accountIDs []string
	for _, a := range accounts {
		accountIDs = append(accountIDs, a.ID)
	}
	if accountIDs == nil {
		accountIDs = []string{}
	}

	strategy := oldCfg.Strategy
	if strategy == "" {
		strategy = "round-robin"
	}

	pool := Pool{
		ID:           uuid.New().String(),
		Name:         "Default Pool",
		ApiKey:       oldCfg.ApiKey,
		Strategy:     strategy,
		RateLimitRPM: oldCfg.RateLimitRPM,
		ProxyURL:     proxyURL,
		AccountIDs:   accountIDs,
		Enabled:      true,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	poolsMu.Lock()
	defer poolsMu.Unlock()
	if err := writePools([]Pool{pool}); err != nil {
		return err
	}

	log.Printf("Migrated legacy pool config to pools.json: pool '%s' with %d accounts", pool.Name, len(accountIDs))
	return nil
}

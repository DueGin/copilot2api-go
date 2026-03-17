package store

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Pool represents a named group of accounts with shared configuration.
type Pool struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	ApiKey       string   `json:"apiKey"`
	Strategy     string   `json:"strategy"`
	RateLimitRPM int      `json:"rateLimitRPM,omitempty"`
	ProxyURL     string   `json:"proxyURL,omitempty"`
	AccountIDs   []string `json:"accountIds"`
	Enabled      bool     `json:"enabled"`
	CreatedAt    string   `json:"createdAt"`
}

type poolStore struct {
	Pools []Pool `json:"pools"`
}

var poolsMu sync.RWMutex

func readPools() ([]Pool, error) {
	data, err := os.ReadFile(PoolsFile())
	if err != nil {
		return []Pool{}, nil
	}
	if len(data) == 0 || string(data) == "{}" {
		return []Pool{}, nil
	}
	var s poolStore
	if err := json.Unmarshal(data, &s); err != nil {
		return []Pool{}, nil
	}
	return s.Pools, nil
}

func writePools(pools []Pool) error {
	s := poolStore{Pools: pools}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(PoolsFile(), data, 0644)
}

// GetPools returns all pools.
func GetPools() ([]Pool, error) {
	poolsMu.RLock()
	defer poolsMu.RUnlock()
	return readPools()
}

// GetPool returns a pool by ID.
func GetPool(id string) (*Pool, error) {
	pools, err := GetPools()
	if err != nil {
		return nil, err
	}
	for _, p := range pools {
		if p.ID == id {
			return &p, nil
		}
	}
	return nil, nil
}

// GetPoolByApiKey returns a pool matching the given API key.
func GetPoolByApiKey(apiKey string) (*Pool, error) {
	pools, err := GetPools()
	if err != nil {
		return nil, err
	}
	for _, p := range pools {
		if p.ApiKey == apiKey {
			return &p, nil
		}
	}
	return nil, nil
}

// AddPool creates a new pool with the given name, strategy, and optional proxy URL.
func AddPool(name, strategy, proxyURL string) (*Pool, error) {
	poolsMu.Lock()
	defer poolsMu.Unlock()

	pools, err := readPools()
	if err != nil {
		return nil, err
	}

	if strategy == "" {
		strategy = "round-robin"
	}

	pool := Pool{
		ID:         uuid.New().String(),
		Name:       name,
		ApiKey:     "sk-pool-" + uuid.New().String(),
		Strategy:   strategy,
		ProxyURL:   proxyURL,
		AccountIDs: []string{},
		Enabled:    true,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	pools = append(pools, pool)
	if err := writePools(pools); err != nil {
		return nil, err
	}
	return &pool, nil
}

// UpdatePool updates a pool by ID with the provided fields.
func UpdatePool(id string, updates map[string]interface{}) (*Pool, error) {
	poolsMu.Lock()
	defer poolsMu.Unlock()

	pools, err := readPools()
	if err != nil {
		return nil, err
	}

	for i, p := range pools {
		if p.ID == id {
			if v, ok := updates["name"].(string); ok {
				pools[i].Name = v
			}
			if v, ok := updates["strategy"].(string); ok && v != "" {
				pools[i].Strategy = v
			}
			if v, ok := updates["proxyURL"].(string); ok {
				pools[i].ProxyURL = v
			}
			if v, ok := updates["enabled"].(bool); ok {
				pools[i].Enabled = v
			}
			if v, ok := updates["rateLimitRPM"]; ok {
				switch rv := v.(type) {
				case float64:
					pools[i].RateLimitRPM = int(rv)
				case int:
					pools[i].RateLimitRPM = rv
				}
			}
			if err := writePools(pools); err != nil {
				return nil, err
			}
			return &pools[i], nil
		}
	}
	return nil, nil
}

// DeletePool removes a pool by ID.
func DeletePool(id string) error {
	poolsMu.Lock()
	defer poolsMu.Unlock()

	pools, err := readPools()
	if err != nil {
		return err
	}

	var filtered []Pool
	for _, p := range pools {
		if p.ID != id {
			filtered = append(filtered, p)
		}
	}
	return writePools(filtered)
}

// RegeneratePoolKey generates a new API key for a pool.
func RegeneratePoolKey(id string) (string, error) {
	poolsMu.Lock()
	defer poolsMu.Unlock()

	pools, err := readPools()
	if err != nil {
		return "", err
	}

	for i, p := range pools {
		if p.ID == id {
			newKey := "sk-pool-" + uuid.New().String()
			pools[i].ApiKey = newKey
			if err := writePools(pools); err != nil {
				return "", err
			}
			return newKey, nil
		}
	}
	return "", nil
}

// AddAccountToPool adds an account to a pool.
// Returns an error if the account already belongs to another pool.
func AddAccountToPool(poolID, accountID string) error {
	poolsMu.Lock()
	defer poolsMu.Unlock()

	pools, err := readPools()
	if err != nil {
		return err
	}

	// Check if account already belongs to another pool.
	for _, p := range pools {
		if p.ID == poolID {
			continue
		}
		for _, aid := range p.AccountIDs {
			if aid == accountID {
				return &AccountAlreadyInPoolError{PoolName: p.Name}
			}
		}
	}

	for i, p := range pools {
		if p.ID == poolID {
			// Check if already in this pool.
			for _, aid := range p.AccountIDs {
				if aid == accountID {
					return nil
				}
			}
			pools[i].AccountIDs = append(pools[i].AccountIDs, accountID)
			return writePools(pools)
		}
	}
	return nil
}

// RemoveAccountFromPool removes an account from a pool.
func RemoveAccountFromPool(poolID, accountID string) error {
	poolsMu.Lock()
	defer poolsMu.Unlock()

	pools, err := readPools()
	if err != nil {
		return err
	}

	for i, p := range pools {
		if p.ID == poolID {
			var filtered []string
			for _, aid := range p.AccountIDs {
				if aid != accountID {
					filtered = append(filtered, aid)
				}
			}
			pools[i].AccountIDs = filtered
			return writePools(pools)
		}
	}
	return nil
}

// GetPoolForAccount returns the pool that contains the given account, or nil.
func GetPoolForAccount(accountID string) (*Pool, error) {
	pools, err := GetPools()
	if err != nil {
		return nil, err
	}
	for _, p := range pools {
		for _, aid := range p.AccountIDs {
			if aid == accountID {
				return &p, nil
			}
		}
	}
	return nil, nil
}

// RemoveAccountFromAllPools removes an account from all pools it belongs to.
func RemoveAccountFromAllPools(accountID string) error {
	poolsMu.Lock()
	defer poolsMu.Unlock()

	pools, err := readPools()
	if err != nil {
		return err
	}

	changed := false
	for i, p := range pools {
		var filtered []string
		for _, aid := range p.AccountIDs {
			if aid != accountID {
				filtered = append(filtered, aid)
			}
		}
		if len(filtered) != len(p.AccountIDs) {
			pools[i].AccountIDs = filtered
			changed = true
		}
	}

	if changed {
		return writePools(pools)
	}
	return nil
}

// AccountAlreadyInPoolError is returned when trying to add an account to a pool
// but the account already belongs to another pool.
type AccountAlreadyInPoolError struct {
	PoolName string
}

func (e *AccountAlreadyInPoolError) Error() string {
	return "account already belongs to pool: " + e.PoolName
}

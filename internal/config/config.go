package config

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Addr string `yaml:"addr"`
	} `yaml:"server"`
	DB struct {
		DSN string `yaml:"dsn"`
	} `yaml:"db"`
	Wallet struct {
		XPub string `yaml:"xpub"`
	} `yaml:"wallet"`
	Chain struct {
		ChainID      string   `yaml:"chain_id"`
		RPCEndpoints []string `yaml:"rpc_endpoints"`
		WSEndpoints  []string `yaml:"ws_endpoints"`
		Denom        string   `yaml:"denom"`
		Decimals     int      `yaml:"decimals"`
		Bech32Prefix string   `yaml:"bech32_prefix"`
		ConfirmDepth int      `yaml:"confirm_depth"`
	} `yaml:"chain"`
	Orders struct {
		MinCredit  int64 `yaml:"min_credit"`
		TTLMinutes int   `yaml:"ttl_minutes"`
	} `yaml:"orders"`
	Worker struct {
		StartHeight          int64 `yaml:"start_height"`
		RewindBlocks         int64 `yaml:"rewind_blocks"`
		MaxBlocksPerTick     int64 `yaml:"max_blocks_per_tick"`
		IntervalSeconds      int64 `yaml:"interval_seconds"`
		PerPage              int   `yaml:"per_page"`
		WSBackfillBlocks     int64 `yaml:"ws_backfill_blocks"`
		RPCFailoverThreshold int   `yaml:"rpc_failover_threshold"`
		WSFailoverThreshold  int   `yaml:"ws_failover_threshold"`
	} `yaml:"worker"`
	Pricing struct {
		FixedCreditPerDora int64 `yaml:"fixed_credit_per_dora"`
	} `yaml:"pricing"`
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = os.Getenv("CONFIG_PATH")
	}
	if path == "" {
		path = "configs/config.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	applyEnvOverrides(&cfg)

	if cfg.Server.Addr == "" {
		return nil, errors.New("server.addr is required")
	}
	if cfg.DB.DSN == "" {
		return nil, errors.New("db.dsn is required")
	}
	if cfg.Chain.ChainID == "" || len(cfg.Chain.RPCEndpoints) == 0 || cfg.Chain.Denom == "" {
		return nil, errors.New("chain config is incomplete")
	}
	return &cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("SERVER_ADDR"); v != "" {
		cfg.Server.Addr = v
	}
	if v := os.Getenv("DB_DSN"); v != "" {
		cfg.DB.DSN = v
	}
	if v := os.Getenv("WALLET_XPUB"); v != "" {
		cfg.Wallet.XPub = v
	}
	if v := os.Getenv("CHAIN_ID"); v != "" {
		cfg.Chain.ChainID = v
	}
	if v := os.Getenv("RPC_ENDPOINTS"); v != "" {
		cfg.Chain.RPCEndpoints = splitCommaList(v)
	}
	if v := os.Getenv("WS_ENDPOINTS"); v != "" {
		cfg.Chain.WSEndpoints = splitCommaList(v)
	}
	if v := os.Getenv("DENOM"); v != "" {
		cfg.Chain.Denom = v
	}
	if v := os.Getenv("BECH32_PREFIX"); v != "" {
		cfg.Chain.Bech32Prefix = v
	}
	if v := os.Getenv("CONFIRM_DEPTH"); v != "" {
		cfg.Chain.ConfirmDepth = atoiOr(cfg.Chain.ConfirmDepth, v)
	}
	if v := os.Getenv("MIN_CREDIT"); v != "" {
		cfg.Orders.MinCredit = atoi64Or(cfg.Orders.MinCredit, v)
	}
	if v := os.Getenv("ORDER_TTL_MINUTES"); v != "" {
		cfg.Orders.TTLMinutes = atoiOr(cfg.Orders.TTLMinutes, v)
	}
	if v := os.Getenv("WORKER_START_HEIGHT"); v != "" {
		cfg.Worker.StartHeight = atoi64Or(cfg.Worker.StartHeight, v)
	}
	if v := os.Getenv("WORKER_REWIND_BLOCKS"); v != "" {
		cfg.Worker.RewindBlocks = atoi64Or(cfg.Worker.RewindBlocks, v)
	}
	if v := os.Getenv("WORKER_MAX_BLOCKS_PER_TICK"); v != "" {
		cfg.Worker.MaxBlocksPerTick = atoi64Or(cfg.Worker.MaxBlocksPerTick, v)
	}
	if v := os.Getenv("WORKER_INTERVAL_SECONDS"); v != "" {
		cfg.Worker.IntervalSeconds = atoi64Or(cfg.Worker.IntervalSeconds, v)
	}
	if v := os.Getenv("WORKER_PER_PAGE"); v != "" {
		cfg.Worker.PerPage = atoiOr(cfg.Worker.PerPage, v)
	}
	if v := os.Getenv("WORKER_WS_BACKFILL_BLOCKS"); v != "" {
		cfg.Worker.WSBackfillBlocks = atoi64Or(cfg.Worker.WSBackfillBlocks, v)
	}
	if v := os.Getenv("WORKER_RPC_FAILOVER_THRESHOLD"); v != "" {
		cfg.Worker.RPCFailoverThreshold = atoiOr(cfg.Worker.RPCFailoverThreshold, v)
	}
	if v := os.Getenv("WORKER_WS_FAILOVER_THRESHOLD"); v != "" {
		cfg.Worker.WSFailoverThreshold = atoiOr(cfg.Worker.WSFailoverThreshold, v)
	}
	if v := os.Getenv("FIXED_CREDIT_PER_DORA"); v != "" {
		cfg.Pricing.FixedCreditPerDora = atoi64Or(cfg.Pricing.FixedCreditPerDora, v)
	}
}

func splitCommaList(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func atoiOr(fallback int, v string) int {
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}

func atoi64Or(fallback int64, v string) int64 {
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return i
}

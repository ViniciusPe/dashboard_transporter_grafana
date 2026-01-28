package grafana

import (
	"fmt"

	"dashboard-transporter/internal/config"
)

// NewClientFromEnv cria um client do Grafana baseado no environment carregado no cfg.
// Ex: NewClientFromEnv(cfg, "dev") / "hml" / "prd"
func NewClientFromEnv(cfg *config.Config, envID string) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	env := cfg.GetEnvironment(envID)
	if env == nil {
		return nil, fmt.Errorf("environment not found: %s", envID)
	}

	if env.URL == "" {
		return nil, fmt.Errorf("environment %s missing URL", envID)
	}
	if env.User == "" {
		return nil, fmt.Errorf("environment %s missing USER", envID)
	}
	if env.Password == "" {
		return nil, fmt.Errorf("environment %s missing PASSWORD", envID)
	}

	return NewClient(env.URL, env.User, env.Password), nil
}

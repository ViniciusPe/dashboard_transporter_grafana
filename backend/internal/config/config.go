package config

import (
	"log"
	"os"
	"strings"
)

type Environment struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	User     string `json:"-"`
	Password string `json:"-"`
}

type Config struct {
	Environments []Environment
}

func Load() *Config {
	envs := []Environment{}

	// DEV
	if e := buildEnv("DEV", "Grafana DEV"); e != nil {
		envs = append(envs, *e)
	}

	// HML
	if e := buildEnv("HML", "Grafana HML"); e != nil {
		envs = append(envs, *e)
	}

	// PRD
	if e := buildEnv("PRD", "Grafana PRD"); e != nil {
		envs = append(envs, *e)
	}

	if len(envs) == 0 {
		log.Printf("[CONFIG] WARNING: Nenhum ambiente configurado via env vars (GRAFANA_*_URL)")
	}

	for _, e := range envs {
		log.Printf("[CONFIG] %s - URL: %s, User: %s", strings.ToUpper(e.ID), e.URL, e.User)
	}

	return &Config{
		Environments: envs,
	}
}

// buildEnv lê:
// - GRAFANA_<SUFFIX>_URL
// - GRAFANA_<SUFFIX>_USER
// - GRAFANA_<SUFFIX>_PASS (preferencial)
// - GRAFANA_<SUFFIX>_PASSWORD (fallback p/ compatibilidade)
func buildEnv(suffix string, displayName string) *Environment {
	url := strings.TrimSpace(os.Getenv("GRAFANA_" + suffix + "_URL"))
	if url == "" {
		return nil
	}

	user := strings.TrimSpace(os.Getenv("GRAFANA_" + suffix + "_USER"))

	pass := os.Getenv("GRAFANA_" + suffix + "_PASS")
	if pass == "" {
		// compatibilidade com o teu código antigo
		pass = os.Getenv("GRAFANA_" + suffix + "_PASSWORD")
	}

	return &Environment{
		ID:       strings.ToLower(suffix),
		Name:     displayName,
		URL:      url,
		User:     user,
		Password: pass,
	}
}

/*
Novo padrão (1 retorno)
*/
func (c *Config) GetEnvironment(id string) *Environment {
	for i := range c.Environments {
		if c.Environments[i].ID == id {
			return &c.Environments[i]
		}
	}
	return nil
}

/*
Compatibilidade TOTAL com código antigo (2 retornos)
*/
func (c *Config) FindEnvironment(id string) (*Environment, bool) {
	env := c.GetEnvironment(id)
	if env == nil {
		return nil, false
	}
	return env, true
}

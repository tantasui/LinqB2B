package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

var validate = validator.New()

func Load(path string) (*Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	expandedContent := expandEnvWithDefault(string(content))

	v := viper.New()
	v.SetConfigType("yaml")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadConfig(strings.NewReader(expandedContent)); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg, func(c *mapstructure.DecoderConfig) {
		c.TagName = "yaml"
	}); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// apply defaults
	if err := cfg.Chains.ApplyDefaults(cfg.Defaults); err != nil {
		return nil, err
	}

	// validate
	if err := validate.Struct(&cfg); err != nil {
		return nil, fmt.Errorf("struct validation failed: %w", err)
	}

	for name, chain := range cfg.Chains {
		// apply name to struct name
		chain.Name = strings.ToUpper(name)
		chain.NativeDenom = strings.TrimSpace(chain.NativeDenom)
		if err := validate.Struct(chain); err != nil {
			return nil, fmt.Errorf("chain %s validation failed: %w", name, err)
		}
		if err := validateChainConfig(chain); err != nil {
			return nil, fmt.Errorf("chain %s validation failed: %w", name, err)
		}
		cfg.Chains[name] = chain
	}

	return &cfg, nil
}

// expandEnvWithDefault replaces ${VAR} or ${VAR:-default} in the string.
func expandEnvWithDefault(s string) string {
	return os.Expand(s, func(k string) string {
		parts := strings.SplitN(k, ":-", 2)
		val := os.Getenv(parts[0])
		if val == "" && len(parts) > 1 {
			return parts[1]
		}
		return val
	})
}

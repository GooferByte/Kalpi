package config

import "github.com/spf13/viper"

// Config holds all application-level configuration loaded from environment variables or .env file.
type Config struct {
	Port            int    `mapstructure:"PORT"`
	Env             string `mapstructure:"ENV"`
	LogLevel        string `mapstructure:"LOG_LEVEL"`
	SessionTTLHours int    `mapstructure:"SESSION_TTL_HOURS"`
	MockMode        bool   `mapstructure:"MOCK_MODE"`
}

// Load reads configuration from .env file and environment variables.
// Environment variables take precedence over .env file values.
func Load() *Config {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	viper.SetDefault("PORT", 8080)
	viper.SetDefault("ENV", "development")
	viper.SetDefault("LOG_LEVEL", "info")
	viper.SetDefault("SESSION_TTL_HOURS", 24)
	viper.SetDefault("MOCK_MODE", false)

	// Ignore error — env vars will fill in if file is absent
	_ = viper.ReadInConfig()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		panic("failed to unmarshal config: " + err.Error())
	}
	return &cfg
}

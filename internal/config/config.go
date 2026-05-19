package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App          AppConfig          `mapstructure:"app"`
	HTTP         HTTPConfig         `mapstructure:"http"`
	Database     DatabaseConfig     `mapstructure:"database"`
	Security     SecurityConfig     `mapstructure:"security"`
	Admin        AdminConfig        `mapstructure:"admin"`
	Notification NotificationConfig `mapstructure:"notification"`
}

type AppConfig struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`
}

type HTTPConfig struct {
	Addr string `mapstructure:"addr"`
}

type DatabaseConfig struct {
	Driver string `mapstructure:"driver"`
	DSN    string `mapstructure:"dsn"`
}

type SecurityConfig struct {
	JWTSecret       string        `mapstructure:"jwt_secret"`
	AccessTokenTTL  time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL time.Duration `mapstructure:"refresh_token_ttl"`
	SecretKey       string        `mapstructure:"secret_key"`
}

type AdminConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Email    string `mapstructure:"email"`
}

type NotificationConfig struct {
	TemplateVersion string `mapstructure:"template_version"`
	PublicBaseURL   string `mapstructure:"public_base_url"`
}

func Load() (*Config, error) {
	v := newViper()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func newViper() *viper.Viper {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./configs")
	v.AddConfigPath(".")
	v.SetEnvPrefix("AEGISOPS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("app.name", "aegisops")
	v.SetDefault("app.env", "dev")
	v.SetDefault("http.addr", ":8080")
	v.SetDefault("database.driver", "postgres")
	v.SetDefault("database.dsn", "postgres://aegisops:aegisops@127.0.0.1:5432/aegisops?sslmode=disable")
	v.SetDefault("security.access_token_ttl", "2h")
	v.SetDefault("security.refresh_token_ttl", "168h")
	v.SetDefault("notification.template_version", "v2")
	v.SetDefault("notification.public_base_url", "http://localhost:4173")

	return v
}

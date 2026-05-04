package config

import (
	"fmt"
	"log/slog"

	"github.com/spf13/viper"
)

func NewConfig() (*ConfigModel, error) {
	var cfg ConfigModel
	v := viper.New()
	v.AddConfigPath("/etc/github.com/poshagator/content-service")
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	err := v.ReadInConfig()
	if err != nil {
		slog.Error("fail to read config", err)
		return &cfg, err
	}
	err = v.Unmarshal(&cfg)
	if err != nil {
		slog.Error("", fmt.Errorf("unable to decode config into struct, %w", err))
		return &cfg, err
	}
	return &cfg, nil
}

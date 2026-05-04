package config

import (
	"fmt"
	"log/slog"

	"github.com/spf13/viper"
)

func NewConfig() (*ConfigModel, error) {
	var cfg ConfigModel
	v := viper.New()
	v.AddConfigPath("/etc/content-service")
	v.AddConfigPath("./config")
	v.AddConfigPath(".")
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	v.AutomaticEnv()

	err := v.ReadInConfig()
	if err != nil {
		slog.Error("fail to read config", "error", err)
		return &cfg, err
	}
	err = v.Unmarshal(&cfg)
	if err != nil {
		slog.Error("unable to decode config into struct", "error", err)
		return &cfg, err
	}
	return &cfg, nil
}

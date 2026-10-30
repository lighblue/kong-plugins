package common

import (
	"fmt"
	"log"
	"strings"

	"github.com/spf13/viper"
)

func LoadConfig(path string) (*viper.Viper, error) {
	viper.SetConfigFile(path)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Error reading config file, %v", err)
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	log.Printf("__ttl: %d, interval: %d", viper.GetInt("auth_cache.ttl"), viper.GetInt("auth_cache.cleanup_interval"))

	return viper.GetViper(), nil
}

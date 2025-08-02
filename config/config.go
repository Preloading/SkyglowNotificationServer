// This is not where you configure the application, this is just so we can read the config file

package config

import (
	"crypto/tls"
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	TCPPort          int      `mapstructure:"TCP_PORT"`
	ServerAddress    string   `mapstructure:"SERVER_ADDRESS"`
	WhitelistedUUIDs []string `mapstructure:"WHITELISTED_UUIDS"`
	BlacklistUUIDs   []string `mapstructure:"BLACKLISTED_UUIDS"`
	WhitelistOn      bool     `mapstructure:"WHITELIST_ON"`
	DB_TYPE          string   `mapstructure:"DB_TYPE"`
	DB_DSN           string   `mapstructure:"DB_DSN"`
	KEY_PATH         string   `mapstructure:"KEY_PATH"`
}

type CryptoKeys struct {
	ServerPublicKeyString *string
	ServerTLSCert         *tls.Certificate
}

// LoadConfig reads configuration from config.yaml file
func LoadConfig() (config Config, err error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	viper.SetEnvPrefix("SGN")

	viper.ReadInConfig()
	// if err != nil {
	// 	return config, fmt.Errorf("error reading config file: %w", err)
	// }

	err = viper.Unmarshal(&config)
	if err != nil {
		return config, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return config, nil
}

func LoadCryptoKeys(keyPath string) (keys *CryptoKeys, err error) {
	cert, err := tls.LoadX509KeyPair(fmt.Sprintf("%s/server_public_key.pem", keyPath), fmt.Sprintf("%s/server_private_key.pem", keyPath))
	if err != nil {
		return nil, err
	}

	serverPubKeyBytes, err := os.ReadFile(fmt.Sprintf("%s/server_public_key.pem", keyPath))
	if err != nil {
		return keys, fmt.Errorf("error reading server public key: %w", err)
	}
	serverPubKeyString := string(serverPubKeyBytes)

	return &CryptoKeys{
		ServerTLSCert:         &cert,
		ServerPublicKeyString: &serverPubKeyString,
	}, nil
}

func IsWhitelisted(uuid string, _config Config) bool {
	for _, allowed := range _config.WhitelistedUUIDs {
		if allowed == uuid {
			return true
		}
	}
	return false
}

func IsBlacklisted(uuid string, _config Config) bool {
	for _, allowed := range _config.BlacklistUUIDs {
		if allowed == uuid {
			return true
		}
	}
	return false
}

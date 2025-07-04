// This is not where you configure the application, this is just so we can read the config file

package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
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
	DB_DSN           string   `mapstructure:"DB_DSN"`
}

type CryptoKeys struct {
	ServerPrivateKey *rsa.PrivateKey
	ServerPublicKey  *rsa.PublicKey
}

// LoadConfig reads configuration from config.yaml file
func LoadConfig() (config Config, err error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	err = viper.ReadInConfig()
	if err != nil {
		return config, fmt.Errorf("error reading config file: %w", err)
	}

	err = viper.Unmarshal(&config)
	if err != nil {
		return config, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return config, nil
}

func LoadCryptoKeys() (keys CryptoKeys, err error) {
	// Load server public key
	serverPubKeyBytes, err := os.ReadFile("keys/server_public_key.pem")
	if err != nil {
		return keys, fmt.Errorf("error reading server public key: %w", err)
	}
	serverPubKeyBlock, _ := pem.Decode(serverPubKeyBytes)
	if serverPubKeyBlock == nil {
		return keys, fmt.Errorf("failed to decode server public key PEM")
	}
	genericPubKey, err := x509.ParsePKIXPublicKey(serverPubKeyBlock.Bytes)
	if err != nil {
		return keys, fmt.Errorf("error parsing server public key: %w", err)
	}
	serverPubKey, ok := genericPubKey.(*rsa.PublicKey)
	if !ok {
		return keys, fmt.Errorf("server public key is not an RSA key")
	}
	keys.ServerPublicKey = serverPubKey

	// Load server private key
	serverPrivKeyBytes, err := os.ReadFile("keys/server_private_key.pem")
	if err != nil {
		return keys, fmt.Errorf("error reading server private key: %w", err)
	}
	serverPrivKeyBlock, _ := pem.Decode(serverPrivKeyBytes)
	if serverPrivKeyBlock == nil {
		return keys, fmt.Errorf("failed to decode server private key PEM")
	}
	serverPrivKey, err := x509.ParsePKCS1PrivateKey(serverPrivKeyBlock.Bytes)
	if err != nil {
		return keys, fmt.Errorf("error parsing server private key: %w", err)
	}
	keys.ServerPrivateKey = serverPrivKey

	return keys, nil
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

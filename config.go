package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"golang.org/x/crypto/blake2s"
)

const (
	DefaultConfigPath = "./setting.conf"
	DefaultMaxWorkers = 100
	DefaultBufferSize = 1500
	DefaultPoolSize   = 1000
)

type Config struct {
	Server     ServerConfig     `toml:"server"`
	KeyPairs   []KeyPairConfig  `toml:"keypairs"`
	BufferPool BufferPoolConfig `toml:"buffer_pool"`
	WorkerPool WorkerPoolConfig `toml:"worker_pool"`
}

type ServerConfig struct {
	ListenAddress  string        `toml:"listen_address"`
	Port           int           `toml:"port"`
	LogLevel       string        `toml:"log_level"`
	PeerExpiration time.Duration `toml:"peer_expiration"`
}

type KeyPairConfig struct {
	Key1 string `toml:"key1"`
	Key2 string `toml:"key2"`
}

type BufferPoolConfig struct {
	PoolSize   int `toml:"pool_size"`
	BufferSize int `toml:"buffer_size"`
}

type WorkerPoolConfig struct {
	MaxWorkers int `toml:"max_workers"`
}

func LoadConfig() (*Config, error) {
	config := &Config{
		Server: ServerConfig{
			ListenAddress:  "0.0.0.0",
			Port:           52820,
			LogLevel:       "info",
			PeerExpiration: 3 * time.Minute,
		},
		BufferPool: BufferPoolConfig{
			PoolSize:   DefaultPoolSize,
			BufferSize: DefaultBufferSize,
		},
		WorkerPool: WorkerPoolConfig{
			MaxWorkers: DefaultMaxWorkers,
		},
	}

	configFilePath := os.Getenv("WG_KNOT_CONFIG_FILE")
	if configFilePath == "" {
		configFilePath = DefaultConfigPath
	}

	configFileFlag := flag.String("configfile", configFilePath, "Path to configuration file")
	listenAddressFlag := flag.String("listen", "", "IP address to listen on")
	portFlag := flag.Int("port", 0, "Port to listen on")
	logLevelFlag := flag.String("loglevel", "", "Log level (debug, info, warning, error)")
	peerExpirationFlag := flag.Duration("peerexpiration", 0, "Peer expiration duration (e.g. 3m, 1h)")
	poolSizeFlag := flag.Int("poolsize", 0, "Buffer pool size")
	bufferSizeFlag := flag.Int("buffersize", 0, "Buffer size")
	maxWorkersFlag := flag.Int("maxworkers", 0, "Maximum number of worker goroutines")

	flag.Parse()

	configFilePath = *configFileFlag

	fileExists := true
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		fileExists = false
		if configFilePath == DefaultConfigPath {
			fmt.Println("Default configuration file not found. Please specify configuration using environment variables or command line arguments.")
		} else {
			return nil, fmt.Errorf("specified configuration file %s not found", configFilePath)
		}
	}

	if fileExists {
		_, err := toml.DecodeFile(configFilePath, config)
		if err != nil {
			return nil, fmt.Errorf("failed to load configuration file: %v", err)
		}
	}

	loadFromEnvironment(config)

	if *listenAddressFlag != "" {
		config.Server.ListenAddress = *listenAddressFlag
	}

	if *portFlag != 0 {
		config.Server.Port = *portFlag
	}

	if *logLevelFlag != "" {
		config.Server.LogLevel = *logLevelFlag
	}

	if *poolSizeFlag != 0 {
		config.BufferPool.PoolSize = *poolSizeFlag
	}

	if *bufferSizeFlag != 0 {
		config.BufferPool.BufferSize = *bufferSizeFlag
	}

	if *maxWorkersFlag != 0 {
		config.WorkerPool.MaxWorkers = *maxWorkersFlag
	}

	if *peerExpirationFlag != 0 {
		config.Server.PeerExpiration = *peerExpirationFlag
	}

	return config, nil
}

func getEnvInt(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}

	intVal, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}

	return intVal
}

func getEnvString(key string, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return val
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}

	duration, err := time.ParseDuration(val)
	if err != nil {
		return defaultVal
	}

	return duration
}

func loadFromEnvironment(config *Config) {
	config.Server.ListenAddress = getEnvString("WG_KNOT_LISTEN_ADDRESS", config.Server.ListenAddress)
	config.Server.Port = getEnvInt("WG_KNOT_PORT", config.Server.Port)

	config.Server.LogLevel = getEnvString("WG_KNOT_LOG_LEVEL", config.Server.LogLevel)
	config.Server.PeerExpiration = getEnvDuration("WG_KNOT_PEER_EXPIRATION", config.Server.PeerExpiration)

	config.BufferPool.PoolSize = getEnvInt("WG_KNOT_POOL_SIZE", config.BufferPool.PoolSize)
	config.BufferPool.BufferSize = getEnvInt("WG_KNOT_BUFFER_SIZE", config.BufferPool.BufferSize)

	config.WorkerPool.MaxWorkers = getEnvInt("WG_KNOT_MAX_WORKERS", config.WorkerPool.MaxWorkers)

	if val := os.Getenv("WG_KNOT_KEY_PAIRS"); val != "" {
		pairs := strings.Split(val, ",")
		for _, pair := range pairs {
			keyParts := strings.Split(strings.TrimSpace(pair), ":")
			if len(keyParts) == 2 {
				config.KeyPairs = append(config.KeyPairs, KeyPairConfig{
					Key1: strings.TrimSpace(keyParts[0]),
					Key2: strings.TrimSpace(keyParts[1]),
				})
			}
		}
	}
}

func GetLogLevel(level string) int {
	switch level {
	case "debug":
		return LogLevelDebug
	case "info":
		return LogLevelInfo
	case "warning":
		return LogLevelWarning
	case "error":
		return LogLevelError
	default:
		return LogLevelInfo
	}
}

func DecodePublicKeyWithError(publicKeyBase64 string) (PublicKey, error) {
	var publicKey PublicKey
	decoded, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil {
		return publicKey, NewInvalidPublicKeyError("invalid base64 encoding")
	}

	if len(decoded) != blake2s.Size {
		return publicKey, NewInvalidPublicKeyError("incorrect key size")
	}

	copy(publicKey[:], decoded)
	return publicKey, nil
}

func LoadPublicKeyPairsFromConfig(keyPairs []KeyPairConfig) ([]PublicKeyPair, error) {
	var publicKeyPairList []PublicKeyPair
	var invalidKeys []string

	for _, kp := range keyPairs {
		publicKey1, err1 := DecodePublicKeyWithError(kp.Key1)
		publicKey2, err2 := DecodePublicKeyWithError(kp.Key2)

		if err1 != nil {
			invalidKeys = append(invalidKeys, kp.Key1)
			continue
		}

		if err2 != nil {
			invalidKeys = append(invalidKeys, kp.Key2)
			continue
		}

		publicKeyPairList = append(publicKeyPairList, PublicKeyPair{
			PublicKey1: publicKey1,
			PublicKey2: publicKey2,
		})
	}

	if len(invalidKeys) > 0 {
		return publicKeyPairList, NewInvalidPublicKeyError(fmt.Sprintf("invalid keys: %v", invalidKeys))
	}

	return publicKeyPairList, nil
}

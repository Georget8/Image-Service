package config

import (
    "os"
    "strconv"
    "strings"
)

type Config struct {
    Port           string
    RedisURL       string
    RedisPassword  string
    AllowedDomains []string
    CacheTTL       int
    MaxImageSize   int64
    RateLimit      int
}

func Load() *Config {
    return &Config{
        Port:           getEnv("PORT", "3000"),
        RedisURL:       getEnv("REDIS_URL", "localhost:6379"),
        RedisPassword:  getEnv("REDIS_PASSWORD", ""),
        AllowedDomains: strings.Split(getEnv("ALLOWED_DOMAINS", ""), ","),
        CacheTTL:       getEnvInt("CACHE_TTL", 86400),
        MaxImageSize:   int64(getEnvInt("MAX_IMAGE_SIZE", 10*1024*1024)),
        RateLimit:      getEnvInt("RATE_LIMIT", 100),
    }
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
    if value := os.Getenv(key); value != "" {
        if intVal, err := strconv.Atoi(value); err == nil {
            return intVal
        }
    }
    return defaultValue
}
package redis

import (
	"os"

	log "github.com/sirupsen/logrus"
)

var (
	redisHost    = ""
	redisPort    = ""
	redisPass    = ""
	redisFullUrl = ""
)

// InitRedisConfig checks env vars and add default values in case we need
func InitRedisConfig() *RedisClient {
	// redis config
	redisHost = GetRedisHost()
	redisPort = GetRedisPort()
	redisPass = GetRedisPass()
	redisFullUrl = GetRedisFullURL()

	log.Info("Redis host to connect: ", redisFullUrl)

	return NewRedisClient(redisFullUrl, redisPass, 0)
}

// GetRedisHost returns the redis host to connect
func GetRedisHost() string {
	redisHost = os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}
	return redisHost
}

// GetRedisPort returns the redis port
func GetRedisPort() string {
	redisPort = os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}
	return redisPort
}

// GetRedisFullURL returns the full url
func GetRedisFullURL() string {
	return redisHost + ":" + redisPort
}

// GetRedisPass returns the redis pass
func GetRedisPass() string {
	redisPass = os.Getenv("REDIS_PASS")
	if redisPass == "" {
		redisPass = ""
	}
	return redisPass
}

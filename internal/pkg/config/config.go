package config

import (
	"os"
)

// Config is a function that acts like a map (key-value pair)
type Config func(key string) string

// Load loads the env variables in .env value into a Config.
func Load() (Config, error) {
	// Since we use "docker-compose", it automatically loads env variables in .env file
	// And so will be available through "os.Getenv". Otherwise, we can use "godotenv".

	// err := godotenv.Load()
	// if err != nil {
	// 	return nil, err
	// }

	return os.Getenv, nil
}

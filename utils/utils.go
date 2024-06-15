package utils

import (
	"fmt"
	"log"
	"os"
)

// GetEnvOrFallback returns the env variable value or the fallback value if the env var is not set
func GetEnvOrFallback(varName, fallback string) string {
	if varValue, ok := os.LookupEnv(varName); ok {
		return varValue
	}

	log.Println(fmt.Sprintf("env variable %s not set, fallback to %s", varName, fallback))
	return fallback
}

// GetEnvOrErr returns the env variable value or an error if the env var is not set
func GetEnvOrErr(varName string) (string, error) {
	if varValue, ok := os.LookupEnv(varName); ok && varValue != "" {
		return varValue, nil
	}

	err := fmt.Errorf("required env variable %s not set or empty", varName)
	log.Println(err)
	return "", err
}

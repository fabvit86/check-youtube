package utils

import (
	"checkYoutube/logging"
	"fmt"
	"log/slog"
	"os"
)

// GetEnvOrFallback returns the env variable value or the fallback value if the env var is not set
func GetEnvOrFallback(varName, fallback string) string {
	const funcName = "GetEnvOrFallback"

	if varValue, ok := os.LookupEnv(varName); ok {
		return varValue
	}

	slog.Warn(fmt.Sprintf("env variable %s not set, fallback to %s", varName, fallback),
		logging.FuncNameAttr(funcName))
	return fallback
}

// GetEnvOrErr returns the env variable value or an error if the env var is not set
func GetEnvOrErr(varName string) (string, error) {
	const funcName = "GetEnvOrErr"

	if varValue, ok := os.LookupEnv(varName); ok && varValue != "" {
		return varValue, nil
	}

	err := fmt.Errorf("required env variable %s not set or empty", varName)
	slog.Error(err.Error(), logging.FuncNameAttr(funcName))
	return "", err
}

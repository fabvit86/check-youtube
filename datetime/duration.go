package datetime

import (
	"checkYoutube/logging"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
)

// FormatISO8601Duration formats an ISO 8601 duration to a human-readable format
func FormatISO8601Duration(duration, username string) (string, error) {
	const funcName = "FormatISO8601Duration"
	regex := regexp.MustCompile(`P(?:(\d+)Y)?(?:(\d+)M)?(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?`)
	matches := regex.FindStringSubmatch(duration)
	errorMsg := fmt.Sprintf("invalid ISO 8601 duration: %s", duration)

	if len(matches) < 1 {
		slog.Warn(errorMsg, logging.FuncNameAttr(funcName), logging.UserAttr(username))
		return "", fmt.Errorf(errorMsg)
	}

	var parsedValues []string
	for i := range regex.SubexpNames() {
		number := matches[i]
		if i == 0 || number == "" {
			continue
		}

		_, err := strconv.Atoi(number)
		if err != nil {
			slog.Warn(errorMsg, logging.FuncNameAttr(funcName), logging.UserAttr(username))
			return "", fmt.Errorf(errorMsg)
		}

		if len(number) == 1 {
			number = "0" + number
		}

		parsedValues = append(parsedValues, number)
	}

	result := strings.Join(parsedValues, ":")
	lastChar := duration[len(duration)-1]
	switch lastChar {
	case 'S':
		if len(result) == 2 {
			result = "00:" + result
		}
	case 'M':
		result = result + ":00"
	case 'H':
		result = result + ":00:00"
	default:
		result = result + string(lastChar)
	}

	return result, nil
}

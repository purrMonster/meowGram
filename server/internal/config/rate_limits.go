package config

import (
	"fmt"
	"os"
	"strconv"
)

type RateLimits struct{ APIPerMinute, APIBurst, MessagePerMinute, MessageBurst, GlobalPerMinute, GlobalBurst int }

func loadRateLimits() (RateLimits, error) {
	result := RateLimits{120, 30, 60, 10, 6000, 200}
	for _, setting := range []struct {
		name  string
		value *int
	}{
		{"API_RATE_PER_MINUTE", &result.APIPerMinute}, {"API_RATE_BURST", &result.APIBurst},
		{"MESSAGE_RATE_PER_MINUTE", &result.MessagePerMinute}, {"MESSAGE_RATE_BURST", &result.MessageBurst},
		{"GLOBAL_RATE_PER_MINUTE", &result.GlobalPerMinute}, {"GLOBAL_RATE_BURST", &result.GlobalBurst},
	} {
		if raw := os.Getenv(setting.name); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || value > 1000000 {
				return result, fmt.Errorf("%s must be an integer between 1 and 1000000", setting.name)
			}
			*setting.value = value
		}
	}
	return result, nil
}

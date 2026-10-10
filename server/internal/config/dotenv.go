package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// loadDotenv searches for a .env file across standard relative directories
// and populates unset OS environment variables.
func loadDotenv() {
	candidates := []string{
		".env",
		"../.env",
		"deploy/.env",
		"../deploy/.env",
		"../../deploy/.env",
		"server/.env",
	}

	for _, relPath := range candidates {
		absPath, err := filepath.Abs(relPath)
		if err != nil {
			continue
		}

		info, err := os.Stat(absPath)
		if err != nil || info.IsDir() {
			continue
		}

		file, err := os.Open(absPath)
		if err != nil {
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			eqIdx := strings.Index(line, "=")
			if eqIdx <= 0 {
				continue
			}

			key := strings.TrimSpace(line[:eqIdx])
			val := strings.TrimSpace(line[eqIdx+1:])

			// Strip surrounding quotes
			if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
				(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
				if len(val) >= 2 {
					val = val[1 : len(val)-1]
				}
			}

			// Presence, not non-empty value, determines precedence. This lets callers
			// deliberately provide an empty value and keeps tests isolated from local
			// ignored .env files.
			if _, exists := os.LookupEnv(key); !exists && key != "" {
				_ = os.Setenv(key, val)
			}
		}

		// Stop after the first valid .env file is processed
		break
	}
}

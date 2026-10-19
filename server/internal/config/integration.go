package config

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

type Integration struct {
	SendPath, ReviewPath, ReviewURL, ReviewKeyFile string
	ReviewActors                                   map[string]bool
}

func LoadIntegration() (Integration, error) {
	c := Integration{SendPath: os.Getenv("MESSAGE_SEND_ENDPOINT"), ReviewPath: os.Getenv("REVIEW_ACTION_ENDPOINT"), ReviewURL: os.Getenv("REVIEW_CALLBACK_URL"), ReviewKeyFile: os.Getenv("REVIEW_SIGNING_KEY_FILE"), ReviewActors: map[string]bool{}}
	for _, path := range []string{c.SendPath, c.ReviewPath} {
		if path != "" && (!strings.HasPrefix(path, "/") || strings.ContainsAny(path, " {}?#\r\n\t") || strings.Contains(path, "//") || strings.HasSuffix(path, "/") || strings.Contains(path, "..")) {
			return c, errors.New("invalid integration endpoint path")
		}
	}
	if c.ReviewPath != "" {
		u, err := url.Parse(c.ReviewURL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path == "" || c.ReviewKeyFile == "" {
			return c, errors.New("review bridge requires explicit HTTPS callback URL and signing key file")
		}
		for _, value := range strings.Split(os.Getenv("REVIEW_ALLOWED_SUBJECTS"), ",") {
			value = strings.TrimSpace(value)
			if value == "" || len(value) > 100 || strings.ContainsAny(value, "\r\n") {
				return c, errors.New("explicit review subjects required")
			}
			c.ReviewActors[value] = true
		}
		if c.ReviewPath == c.SendPath {
			return c, errors.New("integration endpoints must differ")
		}
	}
	return c, nil
}

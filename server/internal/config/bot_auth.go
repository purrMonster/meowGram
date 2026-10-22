package config

import (
	"errors"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"meowgram/server/internal/botauth"
)

// BotAuth is consumed by the message API integration, not global OIDC middleware.
type BotAuth struct {
	Enabled        bool
	Directory      botauth.Options
	TrustedProxies []netip.Prefix
}

// LoadBotAuth loads server-only configuration without reading credentials or
// contacting LDAP. NewDirectory validates enabled directory configuration.
func LoadBotAuth() (BotAuth, error) {
	c := BotAuth{}
	raw := os.Getenv("BOT_BASIC_AUTH_ENABLED")
	if raw != "" {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			return c, errors.New("BOT_BASIC_AUTH_ENABLED must be a boolean")
		}
		c.Enabled = enabled
	}
	if !c.Enabled {
		return c, nil
	}
	c.Directory = botauth.Options{
		URL: os.Getenv("BOT_LDAP_URL"), BaseDN: os.Getenv("BOT_LDAP_BASE_DN"),
		ReaderDN: os.Getenv("BOT_LDAP_READER_DN"), ReaderPasswordFile: os.Getenv("BOT_LDAP_READER_PASSWORD_FILE"),
		AllowedGroup: os.Getenv("BOT_LDAP_ALLOWED_GROUP"), Namespace: os.Getenv("BOT_LDAP_DIRECTORY_NAMESPACE"),
		CAFile: os.Getenv("BOT_LDAP_CA_FILE"), Timeout: 5 * time.Second, MaxConcurrent: 8,
	}
	if raw = os.Getenv("BOT_LDAP_TIMEOUT"); raw != "" {
		timeout, err := time.ParseDuration(raw)
		if err != nil || timeout <= 0 || timeout > 30*time.Second {
			return c, errors.New("BOT_LDAP_TIMEOUT must be greater than zero and at most 30s")
		}
		c.Directory.Timeout = timeout
	}
	if raw = os.Getenv("BOT_LDAP_MAX_CONCURRENT"); raw != "" {
		count, err := strconv.Atoi(raw)
		if err != nil || count < 1 || count > 128 {
			return c, errors.New("BOT_LDAP_MAX_CONCURRENT must be between 1 and 128")
		}
		c.Directory.MaxConcurrent = count
	}
	if raw = os.Getenv("BOT_TRUSTED_PROXY_CIDRS"); raw != "" {
		for _, value := range strings.Split(raw, ",") {
			prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
			if err != nil || prefix.Bits() == 0 {
				return c, errors.New("BOT_TRUSTED_PROXY_CIDRS requires explicit non-universal CIDRs")
			}
			c.TrustedProxies = append(c.TrustedProxies, prefix.Masked())
		}
	}
	return c, nil
}

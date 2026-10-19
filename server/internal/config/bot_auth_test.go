package config

import (
	"testing"
	"time"
)

func TestBotAuthConfiguration(t *testing.T) {
	for _, name := range []string{"BOT_BASIC_AUTH_ENABLED", "BOT_LDAP_TIMEOUT", "BOT_LDAP_MAX_CONCURRENT", "BOT_TRUSTED_PROXY_CIDRS"} {
		t.Setenv(name, "")
	}
	c, err := LoadBotAuth()
	if err != nil || c.Enabled {
		t.Fatal("default enabled")
	}
	t.Setenv("BOT_BASIC_AUTH_ENABLED", "true")
	c, err = LoadBotAuth()
	if err != nil || c.Directory.Timeout != 5*time.Second || c.Directory.MaxConcurrent != 8 || len(c.TrustedProxies) != 0 {
		t.Fatal("unsafe defaults")
	}
	for _, pair := range [][2]string{{"BOT_BASIC_AUTH_ENABLED", "yes"}, {"BOT_LDAP_TIMEOUT", "0s"}, {"BOT_LDAP_TIMEOUT", "31s"}, {"BOT_LDAP_MAX_CONCURRENT", "129"}, {"BOT_LDAP_MAX_CONCURRENT", "0"}, {"BOT_TRUSTED_PROXY_CIDRS", "0.0.0.0/0"}, {"BOT_TRUSTED_PROXY_CIDRS", "bad"}} {
		t.Run(pair[0]+pair[1], func(t *testing.T) {
			t.Setenv(pair[0], pair[1])
			if _, err := LoadBotAuth(); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
	t.Setenv("BOT_TRUSTED_PROXY_CIDRS", "192.0.2.1/32, 2001:db8::1/128")
	c, err = LoadBotAuth()
	if err != nil || len(c.TrustedProxies) != 2 {
		t.Fatal("explicit proxies not loaded")
	}
}

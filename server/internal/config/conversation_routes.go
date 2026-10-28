package config

import (
	"errors"
	"path"
	"strings"
	"unicode"
)

// ValidateConversationEndpoints protects the complete default and alias subtrees.
func ValidateConversationEndpoints(conversations, principals string, reserved ...string) error {
	// ServeMux interprets escapes and wildcard patterns at registration. Require
	// literal canonical reserved paths too, so overlap checks cannot be bypassed.
	for _, route := range reserved {
		if strings.ContainsAny(route, "%?#{}*\\") || path.Clean(route) != route || strings.IndexFunc(route, unicode.IsSpace) >= 0 || strings.IndexFunc(route, unicode.IsControl) >= 0 {
			return errors.New("reserved endpoints must be canonical literal paths")
		}
	}
	roots := []string{"/api/conversations", "/api/principals", conversations, principals}
	for i, a := range roots {
		if a == "/" || !strings.HasPrefix(a, "/") || strings.HasSuffix(a, "/") || path.Clean(a) != a || strings.ContainsAny(a, "%?#{}*\\") || strings.IndexFunc(a, unicode.IsSpace) >= 0 || strings.IndexFunc(a, unicode.IsControl) >= 0 {
			return errors.New("conversation endpoints must be canonical absolute paths")
		}
		for j := 0; j < i; j++ {
			b := roots[j]
			if a == b && ((i == 2 && j == 0) || (i == 3 && j == 1)) {
				continue
			}
			if routeOverlap(a, b) {
				return errors.New("conversation endpoints conflict")
			}
		}
		for _, b := range reserved {
			if routeOverlap(a, b) {
				return errors.New("conversation endpoint conflicts with a reserved route")
			}
		}
	}
	return nil
}
func routeOverlap(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}

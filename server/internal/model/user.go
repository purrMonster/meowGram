package model

import "time"

// User represents the local user account provisioned from Authelia OIDC identity.
type User struct {
	ID          string    `json:"id"`
	AutheliaSub string    `json:"authelia_sub"`
	Username    string    `json:"username"`
	CreatedAt   time.Time `json:"created_at"`
}

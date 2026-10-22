package botauth

import (
	"context"

	"meowgram/server/internal/repository"
)

// RepositoryResolver carries only the verified login uid into provisioning.
// The gate must authenticate and check group membership before every call.
func RepositoryResolver(repo *repository.BotRepository) Resolver {
	return func(ctx context.Context, identity Identity) (Principal, error) {
		p, err := repo.Resolve(ctx, identity.Namespace, identity.ExternalID, identity.Username)
		if err != nil {
			return Principal{}, err
		}
		return Principal{ID: p.ID, Username: p.Username}, nil
	}
}

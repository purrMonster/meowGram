package handler

import (
	"log/slog"
	"meowgram/server/internal/botauth"
	"meowgram/server/internal/repository"
	"net/http"
)

// RegisterSyncRoutes retains the default human-compatible alias without registering it twice.
func RegisterSyncRoutes(mux *http.ServeMux, configured string, repo *repository.MessageRepository, logger *slog.Logger, human func(http.Handler) http.Handler, gate *botauth.Gate) error {
	if err := RegisterSyncRoute(mux, "/api/messages/sync", repo, logger, human, gate); err != nil {
		return err
	}
	if configured != "/api/messages/sync" {
		return RegisterSyncRoute(mux, configured, repo, logger, human, gate)
	}
	return nil
}

func RegisterSyncRoute(mux *http.ServeMux, path string, repo *repository.MessageRepository, logger *slog.Logger, human func(http.Handler) http.Handler, gate *botauth.Gate) error {
	var route http.Handler = SyncHandler(repo, logger)
	if gate == nil {
		route = human(route)
	} else {
		copy := *gate
		copy.SyncPath = path
		copy.SendPath = ""
		copy.HumanOnly = human
		var err error
		route, err = copy.Handler(route)
		if err != nil {
			return err
		}
	}
	mux.Handle("GET "+path, route)
	return nil
}

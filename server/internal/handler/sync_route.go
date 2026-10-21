package handler

import (
	"log/slog"
	"meowgram/server/internal/botauth"
	"meowgram/server/internal/repository"
	"net/http"
)

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

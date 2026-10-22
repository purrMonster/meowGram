package handler

import (
	"meowgram/server/internal/botauth"
	"meowgram/server/internal/chat"
	"net/http"
	"time"
)

// RegisterSendRoute is shared by production and the HTTP contract harness.
// The gate is installed only on the exact POST route; nil keeps bots disabled.
func RegisterSendRoute(mux *http.ServeMux, path string, service *chat.MessageService, retention time.Duration, human func(http.Handler) http.Handler, gate *botauth.Gate) error {
	var route http.Handler = SendHandler(service, retention)
	if gate == nil {
		route = human(route)
	} else {
		copy := *gate
		copy.SendPath = path
		copy.SyncPath = ""
		copy.HumanOnly = human
		var err error
		route, err = copy.Handler(route)
		if err != nil {
			return err
		}
	}
	mux.Handle("POST "+path, route)
	return nil
}

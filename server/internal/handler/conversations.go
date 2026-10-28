package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"meowgram/server/internal/auth"
	"meowgram/server/internal/repository"
)

// ConversationStore keeps HTTP validation separate from transactional access checks.
type ConversationStore interface {
	Create(context.Context, string, *string, []repository.PrincipalRef, bool) (repository.Conversation, bool, error)
	Rename(context.Context, string, string, *string) (repository.Conversation, error)
	ChangeMember(context.Context, string, string, repository.PrincipalRef, bool) error
	SetMute(context.Context, string, *string, bool) error
	MarkRead(context.Context, string, *string, string) error
	List(context.Context, string, int, string) (repository.ConversationPage, error)
	Members(context.Context, string, string, int, string) (repository.MemberPage, error)
	Principals(context.Context, string, string, int, string) (repository.PrincipalPage, error)
}

// RegisterConversationRoutes retains canonical routes when aliases are configured.
// The outer guard deliberately runs before authentication, method and body parsing.
func RegisterConversationRoutes(mux *http.ServeMux, conversations, principals string, store ConversationStore, human func(http.Handler) http.Handler) {
	for _, group := range []struct {
		paths  []string
		picker bool
	}{{[]string{"/api/conversations", conversations}, false}, {[]string{"/api/principals", principals}, true}} {
		seen := map[string]bool{}
		for _, root := range group.paths {
			if root == "" || seen[root] {
				continue
			}
			seen[root] = true
			inner := human(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { serveConversation(w, r, root, group.picker, store) }))
			guarded := ConversationBasicGuard(inner, conversations, principals)
			mux.Handle(root, guarded)
			mux.Handle(root+"/", guarded)
		}
	}
}

// ConversationBasicGuard must wrap CORS so a Basic-authenticated OPTIONS request
// cannot bypass the human-only contract through preflight handling. Genuine
// browser preflights carry no Authorization header and continue unchanged.
func ConversationBasicGuard(next http.Handler, conversations, principals string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		protected := false
		for _, root := range []string{"/api/conversations", "/api/principals", conversations, principals} {
			if root != "" && (r.URL.Path == root || strings.HasPrefix(r.URL.Path, root+"/")) {
				protected = true
				break
			}
		}
		if protected {
			w.Header().Set("Cache-Control", "no-store")
			for _, value := range r.Header.Values("Authorization") {
				fields := strings.Fields(value)
				if len(fields) > 0 && strings.EqualFold(fields[0], "Basic") {
					writeSyncJSONError(w, 401, "unauthorized", "bearer authentication required")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
func serveConversation(w http.ResponseWriter, r *http.Request, root string, picker bool, store ConversationStore) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok || user == nil {
		writeSyncJSONError(w, 401, "unauthorized", "bearer authentication required")
		return
	}
	actor := user.AutheliaSub
	bad := func() { conversationError(w, repository.ErrConversationInput) }
	method := func(allowed string) bool {
		if r.Method == allowed {
			return true
		}
		w.Header().Set("Allow", allowed)
		writeSyncJSONError(w, 405, "method_not_allowed", "method not allowed")
		return false
	}
	suffix := strings.TrimPrefix(r.URL.EscapedPath(), (&url.URL{Path: root}).EscapedPath())
	var parts []string
	if suffix != "" {
		if !strings.HasPrefix(suffix, "/") {
			bad()
			return
		}
		parts = strings.Split(suffix[1:], "/")
		for i, p := range parts {
			v, e := url.PathUnescape(p)
			if e != nil || v == "" {
				bad()
				return
			}
			parts[i] = v
		}
	}
	allowed := []string{}
	page := false
	if r.Method == "GET" && ((len(parts) == 0) || (!picker && len(parts) == 2 && parts[1] == "members")) {
		allowed = []string{"limit", "cursor"}
		page = true
		if picker {
			allowed = append(allowed, "q")
		}
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		bad()
		return
	}
	for k, v := range query {
		found := false
		for _, a := range allowed {
			if k == a {
				found = true
			}
		}
		if !found || len(v) != 1 {
			bad()
			return
		}
	}
	limit := 50
	if picker {
		limit = 20
	}
	if page {
		if raw, exists := query["limit"]; exists {
			n, e := strconv.Atoi(raw[0])
			max := 100
			if picker {
				max = 50
			}
			if e != nil || n < 1 || n > max {
				bad()
				return
			}
			limit = n
		}
		if v, exists := query["cursor"]; exists && v[0] == "" {
			bad()
			return
		}
	}
	finish := func(v any, e error, status int) {
		if e != nil {
			conversationError(w, e)
			return
		}
		if status == 204 {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	if picker {
		if len(parts) != 0 {
			http.NotFound(w, r)
			return
		}
		if !method("GET") {
			return
		}
		q := query.Get("q")
		if utf8.RuneCountInString(q) < 1 || utf8.RuneCountInString(q) > 100 {
			bad()
			return
		}
		v, e := store.Principals(r.Context(), actor, q, limit, query.Get("cursor"))
		finish(v, e, 200)
		return
	}
	if len(parts) == 0 && r.Method == "GET" {
		v, e := store.List(r.Context(), actor, limit, query.Get("cursor"))
		finish(v, e, 200)
		return
	}
	if len(parts) == 0 || (len(parts) == 1 && parts[0] == "start-chat") {
		if !method("POST") {
			return
		}
		reuse := len(parts) == 1
		var input struct {
			Name    *string                   `json:"name"`
			Members []repository.PrincipalRef `json:"members"`
		}
		fields, e := conversationBody(w, r, &input)
		if e != nil {
			bad()
			return
		}
		if _, has := fields["name"]; reuse && has {
			bad()
			return
		}
		if !reuse && (input.Name == nil || strings.TrimSpace(*input.Name) == "") {
			bad()
			return
		}
		if input.Members == nil {
			bad()
			return
		}
		v, created, e := store.Create(r.Context(), actor, input.Name, input.Members, reuse)
		if reuse {
			finish(struct {
				Conversation repository.Conversation `json:"conversation"`
				Created      bool                    `json:"created"`
			}{v, created}, e, 200)
		} else {
			finish(v, e, 201)
		}
		return
	}
	personal := len(parts) == 2 && (parts[1] == "mute" || parts[1] == "read")
	var channel *string
	if personal && parts[0] == "lounge" {
	} else {
		id, e := repository.ParseChannel(parts[0])
		if e != nil {
			bad()
			return
		}
		channel = &id
	}
	if len(parts) == 1 {
		if !method("PATCH") {
			return
		}
		var input struct {
			Name *string `json:"name"`
		}
		fields, e := conversationBody(w, r, &input)
		if _, has := fields["name"]; e != nil || !has {
			bad()
			return
		}
		v, e := store.Rename(r.Context(), actor, *channel, input.Name)
		finish(v, e, 200)
		return
	}
	if personal {
		if !method("PUT") {
			return
		}
		if parts[1] == "mute" {
			var input struct {
				Muted *bool `json:"muted"`
			}
			_, e := conversationBody(w, r, &input)
			if e != nil || input.Muted == nil {
				bad()
				return
			}
			finish(nil, store.SetMute(r.Context(), actor, channel, *input.Muted), 204)
		} else {
			var input struct {
				MessageID string `json:"message_id"`
			}
			_, e := conversationBody(w, r, &input)
			id, parse := repository.ParseChannel(input.MessageID)
			if e != nil || parse != nil {
				bad()
				return
			}
			finish(nil, store.MarkRead(r.Context(), actor, channel, id), 204)
		}
		return
	}
	if len(parts) == 2 && parts[1] == "members" {
		if r.Method == "GET" {
			v, e := store.Members(r.Context(), actor, *channel, limit, query.Get("cursor"))
			finish(v, e, 200)
			return
		}
		if !method("POST") {
			return
		}
		var input repository.PrincipalRef
		_, e := conversationBody(w, r, &input)
		if e != nil {
			bad()
			return
		}
		input, e = repository.NormalizePrincipal(input)
		if e != nil {
			bad()
			return
		}
		finish(nil, store.ChangeMember(r.Context(), actor, *channel, input, true), 204)
		return
	}
	if len(parts) == 4 && parts[1] == "members" {
		if !method("DELETE") {
			return
		}
		if !conversationEmptyBody(w, r) {
			bad()
			return
		}
		p, e := repository.NormalizePrincipal(repository.PrincipalRef{Kind: parts[2], ID: parts[3]})
		if e != nil {
			bad()
			return
		}
		finish(nil, store.ChangeMember(r.Context(), actor, *channel, p, false), 204)
		return
	}
	if len(parts) == 2 && parts[1] == "leave" {
		if !method("POST") {
			return
		}
		if !conversationEmptyBody(w, r) {
			bad()
			return
		}
		finish(nil, store.ChangeMember(r.Context(), actor, *channel, repository.PrincipalRef{Kind: "human", ID: actor}, false), 204)
		return
	}
	http.NotFound(w, r)
}

func conversationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrConversationInput):
		writeSyncJSONError(w, 400, "invalid_request", "invalid request")
	case errors.Is(err, repository.ErrConversationForbidden):
		writeSyncJSONError(w, 403, "forbidden", "request not permitted")
	default:
		w.Header().Set("Retry-After", "1")
		writeSyncJSONError(w, 503, "service_unavailable", "service temporarily unavailable")
	}
}
func conversationEmptyBody(w http.ResponseWriter, r *http.Request) bool {
	b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 32768))
	return e == nil && len(bytes.TrimSpace(b)) == 0
}

// Reject duplicate keys at every nesting level before decoding typed fields.
func conversationBody(w http.ResponseWriter, r *http.Request, target any) (map[string]json.RawMessage, error) {
	typ, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || typ != "application/json" {
		return nil, repository.ErrConversationInput
	}
	b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 32768))
	if e != nil {
		return nil, e
	}
	if !utf8.Valid(b) {
		return nil, repository.ErrConversationInput
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if e = conversationJSONValue(d, 0); e != nil {
		return nil, e
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, repository.ErrConversationInput
	}
	var fields map[string]json.RawMessage
	if e = json.Unmarshal(b, &fields); e != nil || fields == nil {
		return nil, repository.ErrConversationInput
	}
	if e = conversationJSONFields(b, reflect.TypeOf(target)); e != nil {
		return nil, e
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e = d.Decode(target); e != nil {
		return nil, e
	}
	return fields, nil
}
func conversationJSONValue(d *json.Decoder, depth int) error {
	if depth > 16 {
		return repository.ErrConversationInput
	}
	tok, e := d.Token()
	if e != nil {
		return e
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, e := d.Token()
			if e != nil {
				return e
			}
			s, ok := key.(string)
			if !ok || seen[s] {
				return repository.ErrConversationInput
			}
			seen[s] = true
			if e = conversationJSONValue(d, depth+1); e != nil {
				return e
			}
		}
	case '[':
		for d.More() {
			if e = conversationJSONValue(d, depth+1); e != nil {
				return e
			}
		}
	default:
		return repository.ErrConversationInput
	}
	_, e = d.Token()
	return e
}

// encoding/json matches field names case-insensitively; the wire contract does not.
func conversationJSONFields(raw json.RawMessage, typ reflect.Type) error {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Struct:
		var values map[string]json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil || values == nil {
			return repository.ErrConversationInput
		}
		allowed := map[string]reflect.Type{}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			key := strings.Split(f.Tag.Get("json"), ",")[0]
			if key != "" && key != "-" {
				allowed[key] = f.Type
			}
		}
		for key, value := range values {
			field, ok := allowed[key]
			if !ok {
				return repository.ErrConversationInput
			}
			if err := conversationJSONFields(value, field); err != nil {
				return err
			}
		}
	case reflect.Slice:
		var values []json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil {
			return repository.ErrConversationInput
		}
		for _, value := range values {
			if err := conversationJSONFields(value, typ.Elem()); err != nil {
				return err
			}
		}
	}
	return nil
}

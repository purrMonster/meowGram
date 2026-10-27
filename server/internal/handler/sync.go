package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"meowgram/server/internal/auth"
	"meowgram/server/internal/botauth"
	"meowgram/server/internal/model"
	"meowgram/server/internal/repository"
)

// SyncHandler serves the catch-up synchronization endpoint (UST-1.4.3).
//
// Endpoint Contract:
//
//	GET /api/messages/sync?after={iso8601_timestamp}&after_id={uuid}
//
// Architectural Rationale:
//   - When a client reconnects after an extended offline duration, the default
//     50-message WebSocket hydration burst cannot guarantee complete timeline recovery.
//   - Instead of overloading the WebSocket handshake frame with complex pagination,
//     a dedicated HTTP REST query allows deterministic, paginated gap-filling.
//   - Capped strictly at 500 messages per request to prevent server-side buffer bloat
//     and mobile network latency degradation.
//
// Timezone Standardization:
//   - PostgreSQL stores `messages.created_at` as `TIMESTAMPTZ` (stored internally in UTC).
//   - Client cursors contain both created_at and the message UUID, so equal-time
//     messages remain ordered and cannot be skipped across page boundaries.
//   - Client timestamps are parsed using RFC3339Nano / RFC3339 layouts.
//   - If the incoming query string contains an offset (e.g. `+05:30` or `-04:00`),
//     it is normalized directly to UTC (`.UTC()`) before querying the database.
//   - If URL decoding turns `+` into a space ` ` (e.g. `2026-10-03T19:10:00 05:30`),
//     the parser intelligently restores the `+` sign for seamless parsing.
//
// SyncPageSize is the maximum number of messages returned per sync request.
const SyncPageSize = 500

func SyncHandler(msgRepo *repository.MessageRepository, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		var senderID, username string
		kind := "human"
		if bot, ok := botauth.PrincipalFromContext(r.Context()); ok {
			senderID, username = bot.ID, bot.Username
			kind = "bot"
		} else if user, ok := auth.UserFromContext(r.Context()); ok && user != nil {
			senderID, username = user.AutheliaSub, user.Username
		} else {
			writeSyncJSONError(w, 401, "unauthorized", "authenticated sender required")
			return
		}
		query, queryErr := url.ParseQuery(r.URL.RawQuery)
		if queryErr != nil {
			conversationError(w, repository.ErrConversationInput)
			return
		}
		for key, values := range query {
			if len(values) != 1 || (key != "after" && key != "after_id" && key != "channel_id" && key != "scope") {
				conversationError(w, repository.ErrConversationInput)
				return
			}
		}
		var channel *string
		rawChannel, hasChannel := query["channel_id"]
		rawScope, hasScope := query["scope"]
		if hasChannel && hasScope {
			conversationError(w, repository.ErrConversationInput)
			return
		}
		if hasChannel {
			id, e := repository.ParseChannel(rawChannel[0])
			if e != nil {
				conversationError(w, e)
				return
			}
			channel = &id
		}
		memberScope := false
		if hasScope {
			if rawScope[0] != "member" {
				conversationError(w, repository.ErrConversationInput)
				return
			}
			memberScope = true
		}
		// Validate mandatory cursor.
		rawAfter := query.Get("after")
		if strings.TrimSpace(rawAfter) == "" {
			logger.Warn("Sync request missing 'after' parameter",
				"remote_addr", r.RemoteAddr,
				"sub", senderID,
			)
			writeSyncJSONError(w, http.StatusBadRequest, "bad_request", "missing required query parameter 'after' (expected ISO 8601 / RFC 3339 timestamp)")
			return
		}

		// 3. Parse and standardize timestamp to UTC
		afterTime, err := ParseSyncTimestamp(rawAfter)
		if err != nil {
			logger.Warn("Sync request provided malformed timestamp",
				"remote_addr", r.RemoteAddr,
				"sub", senderID,
				"raw_after", rawAfter,
				"error", err,
			)
			writeSyncJSONError(w, http.StatusBadRequest, "bad_request", fmt.Sprintf("invalid 'after' timestamp: %v", err))
			return
		}

		// 4. Query persistent storage for chronological gap fill (max SyncPageSize records)
		afterID := query.Get("after_id")
		if afterID != "" && !uuidPattern.MatchString(afterID) {
			writeSyncJSONError(w, http.StatusBadRequest, "bad_request", "invalid 'after_id' (expected UUID)")
			return
		}
		messages, version, err := msgRepo.GetScopedMessagesAfter(r.Context(), repository.SendPrincipal{Kind: kind, ID: senderID, Username: username}, channel, memberScope, afterTime, afterID, SyncPageSize)
		if err != nil {
			if errors.Is(err, repository.ErrConversationForbidden) || errors.Is(err, repository.ErrConversationInput) {
				conversationError(w, err)
				return
			}
			logger.Error("Failed to query messages for sync",
				"sub", senderID,
				"after", afterTime,
				"error", err,
			)
			w.Header().Set("Retry-After", "1")
			writeSyncJSONError(w, http.StatusServiceUnavailable, "unavailable", "message storage unavailable")
			return
		}

		if memberScope {
			w.Header().Set("X-Membership-Version", strconv.FormatInt(version, 10))
		}

		// Ensure we always serialize an empty JSON array `[]` rather than `null`
		if messages == nil {
			messages = make([]*model.Message, 0)
		}

		logger.Info("Catch-up sync served successfully",
			"sub", senderID,
			"username", username,
			"after_utc", afterTime.Format(time.RFC3339Nano),
			"delivered_count", len(messages),
		)

		// 5. Respond with JSON message list. A full page means the client should
		// request the next page using the last message's created_at as `after`.
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if len(messages) >= SyncPageSize {
			w.Header().Set("X-Has-More", "true")
		}
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(messages); err != nil {
			logger.Error("Failed to encode sync response", "error", err)
		}
	}
}

// ParseSyncTimestamp parses an ISO 8601 or RFC 3339 timestamp and normalizes to UTC.
//
// Mechanics:
// - Supports nanosecond precision (`time.RFC3339Nano`) and second precision (`time.RFC3339`).
// - Supports epoch timestamps (in seconds or milliseconds) for universal compatibility.
// - Corrects unencoded `+` characters converted to spaces during HTTP URL decoding.
// - Guarantees the returned `time.Time` is in `.UTC()` location.
func ParseSyncTimestamp(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, errors.New("timestamp cannot be empty")
	}

	// 1. Check numeric Unix epoch timestamps
	if unixVal, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if unixVal > 100000000000 {
			// Values greater than 100B are millisecond timestamps
			return time.UnixMilli(unixVal).UTC(), nil
		}
		return time.Unix(unixVal, 0).UTC(), nil
	}

	// 2. Correct space-encoded '+' in timezone offsets (e.g. "2026-10-03T19:10:00 05:30").
	// Only a trailing " HH:MM" after a T-separated time is treated as an offset, so the
	// space-separated "2006-01-02 15:04:05" layout below stays reachable.
	if m := spaceOffsetPattern.FindStringSubmatch(raw); m != nil {
		raw = m[1] + "+" + m[2]
	}

	// 3. Match against supported ISO 8601 / RFC 3339 layouts
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("expected RFC3339/ISO8601 format (e.g. '2026-10-03T13:40:00Z'), received %q", raw)
}

var spaceOffsetPattern = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?) (\d{2}:?\d{2})$`)
var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func writeSyncJSONError(w http.ResponseWriter, statusCode int, errCode string, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   errCode,
		"message": message,
	})
}

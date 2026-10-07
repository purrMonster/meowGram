package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"meowgram/server/internal/auth"
	"meowgram/server/internal/model"
	"meowgram/server/internal/repository"
)

// SyncHandler serves the catch-up synchronization endpoint (UST-1.4.3).
//
// Endpoint Contract:
//
//	GET /api/messages/sync?after={iso8601_timestamp}
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
		// 1. Enforce authenticated context from Authelia OIDC middleware
		user, hasUser := auth.UserFromContext(r.Context())
		if !hasUser || user == nil {
			writeSyncJSONError(w, http.StatusUnauthorized, "unauthorized", "valid Authelia OIDC token required")
			return
		}

		// 2. Validate mandatory 'after' query parameter
		rawAfter := r.URL.Query().Get("after")
		if strings.TrimSpace(rawAfter) == "" {
			logger.Warn("Sync request missing 'after' parameter",
				"remote_addr", r.RemoteAddr,
				"sub", user.AutheliaSub,
			)
			writeSyncJSONError(w, http.StatusBadRequest, "bad_request", "missing required query parameter 'after' (expected ISO 8601 / RFC 3339 timestamp)")
			return
		}

		// 3. Parse and standardize timestamp to UTC
		afterTime, err := ParseSyncTimestamp(rawAfter)
		if err != nil {
			logger.Warn("Sync request provided malformed timestamp",
				"remote_addr", r.RemoteAddr,
				"sub", user.AutheliaSub,
				"raw_after", rawAfter,
				"error", err,
			)
			writeSyncJSONError(w, http.StatusBadRequest, "bad_request", fmt.Sprintf("invalid 'after' timestamp: %v", err))
			return
		}

		// 4. Query persistent storage for chronological gap fill (max SyncPageSize records)
		messages, err := msgRepo.GetMessagesAfter(r.Context(), afterTime, SyncPageSize)
		if err != nil {
			logger.Error("Failed to query messages for sync",
				"sub", user.AutheliaSub,
				"after", afterTime,
				"error", err,
			)
			writeSyncJSONError(w, http.StatusInternalServerError, "internal_error", "failed to retrieve catch-up messages from database")
			return
		}

		// Ensure we always serialize an empty JSON array `[]` rather than `null`
		if messages == nil {
			messages = make([]*model.Message, 0)
		}

		logger.Info("Catch-up sync served successfully",
			"sub", user.AutheliaSub,
			"username", user.Username,
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

func writeSyncJSONError(w http.ResponseWriter, statusCode int, errCode string, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   errCode,
		"message": message,
	})
}

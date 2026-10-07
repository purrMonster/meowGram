package handler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"meowgram/server/internal/auth"
	"meowgram/server/internal/model"
)

func TestParseSyncTimestamp(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErr     bool
		expectedUTC time.Time
	}{
		{
			name:        "RFC3339 Standard UTC",
			input:       "2026-10-03T13:40:00Z",
			wantErr:     false,
			expectedUTC: time.Date(2026, 10, 3, 13, 40, 0, 0, time.UTC),
		},
		{
			name:        "RFC3339 with positive offset (+05:30)",
			input:       "2026-10-03T19:10:00+05:30",
			wantErr:     false,
			expectedUTC: time.Date(2026, 10, 3, 13, 40, 0, 0, time.UTC),
		},
		{
			name:        "RFC3339 with negative offset (-04:00)",
			input:       "2026-10-03T09:40:00-04:00",
			wantErr:     false,
			expectedUTC: time.Date(2026, 10, 3, 13, 40, 0, 0, time.UTC),
		},
		{
			name:        "RFC3339Nano precision",
			input:       "2026-10-03T13:40:00.500000000Z",
			wantErr:     false,
			expectedUTC: time.Date(2026, 10, 3, 13, 40, 0, 500000000, time.UTC),
		},
		{
			name:        "URL-decoded space in place of plus sign",
			input:       "2026-10-03T19:10:00 05:30",
			wantErr:     false,
			expectedUTC: time.Date(2026, 10, 3, 13, 40, 0, 0, time.UTC),
		},
		{
			name:        "Unix Epoch in seconds",
			input:       "1700000000",
			wantErr:     false,
			expectedUTC: time.Unix(1700000000, 0).UTC(),
		},
		{
			name:        "Unix Epoch in milliseconds",
			input:       "1700000000000",
			wantErr:     false,
			expectedUTC: time.UnixMilli(1700000000000).UTC(),
		},
		{
			name:        "Space-separated date and time (no offset)",
			input:       "2026-10-03 13:40:00",
			wantErr:     false,
			expectedUTC: time.Date(2026, 10, 3, 13, 40, 0, 0, time.UTC),
		},
		{
			name:        "URL-decoded space in place of plus sign with fractional seconds",
			input:       "2026-10-03T19:10:00.250 05:30",
			wantErr:     false,
			expectedUTC: time.Date(2026, 10, 3, 13, 40, 0, 250000000, time.UTC),
		},
		{
			name:    "Empty input",
			input:   "   ",
			wantErr: true,
		},
		{
			name:    "Invalid format",
			input:   "not-a-timestamp",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSyncTimestamp(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseSyncTimestamp(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr {
				if !got.Equal(tt.expectedUTC) {
					t.Errorf("ParseSyncTimestamp(%q) = %v, want %v", tt.input, got, tt.expectedUTC)
				}
				if got.Location() != time.UTC {
					t.Errorf("expected UTC location, got %v", got.Location())
				}
			}
		})
	}
}

func TestSyncHandler_Validations(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := SyncHandler(nil, logger)

	t.Run("Rejects unauthenticated request with 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/messages/sync?after=2026-10-03T13:40:00Z", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
		}
	})

	t.Run("Rejects missing after parameter with 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/messages/sync", nil)
		// Inject mock user context
		ctx := context.WithValue(req.Context(), auth.UserContextKey, &model.User{
			ID:          "usr-test",
			AutheliaSub: "sub-test",
			Username:    "whiskers",
		})
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("Rejects malformed after parameter with 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/messages/sync?after=invalid-time", nil)
		ctx := context.WithValue(req.Context(), auth.UserContextKey, &model.User{
			ID:          "usr-test",
			AutheliaSub: "sub-test",
			Username:    "whiskers",
		})
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})
}

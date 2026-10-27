package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

func TestActualV200Decoder(t *testing.T) {
	const expected = "be15cec6ceed2cb4a21b69cfe5e4548d2bfd818c"
	out, err := exec.Command("/legacy-decoder", "source").Output()
	if err != nil || strings.TrimSpace(string(out)) != expected {
		t.Fatalf("wrong actual source identity: %v", err)
	}
	const lounge = `{"id":"11111111-1111-4111-8111-111111111111","sender_id":"synthetic-human","sender_kind":"human","username":"Human","text_content":"LOUNGE_POSITIVE_CONTROL","created_at":"2020-01-01T00:00:00Z"}`
	private := strings.Replace(lounge, "LOUNGE_POSITIVE_CONTROL", "PRIVATE_POSITIVE_CONTROL", 1)
	private = strings.TrimSuffix(private, "}") + `,"channel_id":"22222222-2222-4222-8222-222222222222"}`
	for _, tc := range []struct{ name, mode, input, canary string }{
		{"lounge", "message", lounge, "LOUNGE_POSITIVE_CONTROL"},
		{"new_null_lounge", "message", strings.TrimSuffix(lounge, "}") + `,"channel_id":null}`, "LOUNGE_POSITIVE_CONTROL"},
		{"private_field_ignored", "message", private, "PRIVATE_POSITIVE_CONTROL"},
		{"history_private_field_ignored", "history", "[" + lounge + "," + private + "]", "PRIVATE_POSITIVE_CONTROL"},
		{"live_private_field_ignored", "live", strings.TrimSuffix(private, "}") + `,"type":"chat"}`, "PRIVATE_POSITIVE_CONTROL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("/legacy-decoder", tc.mode)
			cmd.Stdin = strings.NewReader(tc.input)
			got, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			if !json.Valid(got) || !bytes.Contains(got, []byte(tc.canary)) {
				t.Fatal("decoder failed positive control")
			}
			if bytes.Contains(got, []byte("channel_id")) {
				t.Fatal("unexpected channel-aware legacy decoder")
			}
			if tc.mode == "history" && !bytes.Contains(got, []byte("LOUNGE_POSITIVE_CONTROL")) {
				t.Fatal("history lost lounge control")
			}
		})
	}
	t.Run("malformed_input_rejected", func(t *testing.T) {
		cmd := exec.Command("/legacy-decoder", "message")
		cmd.Stdin = strings.NewReader("{")
		if err := cmd.Run(); err == nil {
			t.Fatal("malformed input accepted")
		}
	})
	t.Run("trailing_value_rejected", func(t *testing.T) {
		cmd := exec.Command("/legacy-decoder", "message")
		cmd.Stdin = strings.NewReader(lounge + lounge)
		if err := cmd.Run(); err == nil {
			t.Fatal("trailing input accepted")
		}
	})
}

package cmd

import (
	"encoding/json"
	"testing"
)

func TestIsBugWebhookEvent(t *testing.T) {
	cases := []struct {
		name string
		got  bool
	}{
		{"bug::create", true},
		{"bug::update", true},
		{"bug_create", true},
		{"bug_update", true},
		{"story::create", false},
		{"task_update", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isBugWebhookEvent(tc.name); got != tc.got {
				t.Fatalf("isBugWebhookEvent(%q)=%v want=%v", tc.name, got, tc.got)
			}
		})
	}
}

func TestExtractBugEventTarget(t *testing.T) {
	cases := []struct {
		name       string
		payload    string
		wantOK     bool
		wantWS     string
		wantBugID  string
		wantReason string
	}{
		{
			name:      "event bug id",
			payload:   `{"id":1,"received_at":1,"event":{"event":"bug::create","workspace_id":"123","bug":{"id":"456"}}}`,
			wantOK:    true,
			wantWS:    "123",
			wantBugID: "456",
		},
		{
			name:      "object id",
			payload:   `{"id":2,"received_at":1,"event":{"event":"bug_update","workspace_id":123,"object":{"id":456}}}`,
			wantOK:    true,
			wantWS:    "123",
			wantBugID: "456",
		},
		{
			name:      "data nested id",
			payload:   `{"id":3,"received_at":1,"event":{"event":"bug::update","workspace_id":"123","data":{"bug":{"id":"789"}}}}`,
			wantOK:    true,
			wantWS:    "123",
			wantBugID: "789",
		},
		{
			name:      "large numeric ids",
			payload:   `{"id":4,"received_at":1,"event":{"event":"bug::create","workspace_id":9007199254740993,"bug":{"id":9007199254740995}}}`,
			wantOK:    true,
			wantWS:    "9007199254740993",
			wantBugID: "9007199254740995",
		},
		{
			name:       "story skipped",
			payload:    `{"id":5,"received_at":1,"event":{"event":"story::create","workspace_id":"123","story":{"id":"456"}}}`,
			wantOK:     false,
			wantReason: "not_bug_event",
		},
		{
			name:       "missing workspace",
			payload:    `{"id":6,"received_at":1,"event":{"event":"bug::create","bug":{"id":"456"}}}`,
			wantOK:     false,
			wantReason: "missing_workspace_id",
		},
		{
			name:       "missing bug id",
			payload:    `{"id":7,"received_at":1,"event":{"event":"bug::create","workspace_id":"123"}}`,
			wantOK:     false,
			wantReason: "missing_bug_id",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ev streamEvent
			if err := json.Unmarshal([]byte(tc.payload), &ev); err != nil {
				t.Fatal(err)
			}
			got, ok, reason := extractBugEventTarget(&ev)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v want=%v reason=%s", ok, tc.wantOK, reason)
			}
			if reason != tc.wantReason {
				t.Fatalf("reason=%q want=%q", reason, tc.wantReason)
			}
			if ok {
				if got.WorkspaceID != tc.wantWS || got.BugID != tc.wantBugID || got.EventID != ev.ID {
					t.Fatalf("target=%+v want workspace=%s bug=%s event=%d", got, tc.wantWS, tc.wantBugID, ev.ID)
				}
			}
		})
	}
}

func TestExtractBugEventTargetRejectsInvalidEvents(t *testing.T) {
	cases := []struct {
		name       string
		ev         *streamEvent
		wantReason string
	}{
		{
			name:       "nil event",
			wantReason: "nil_event",
		},
		{
			name:       "invalid payload",
			ev:         &streamEvent{ID: 1, Event: json.RawMessage(`{`)},
			wantReason: "invalid_payload",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok, reason := extractBugEventTarget(tc.ev)
			if ok {
				t.Fatal("ok=true want=false")
			}
			if reason != tc.wantReason {
				t.Fatalf("reason=%q want=%q", reason, tc.wantReason)
			}
		})
	}
}

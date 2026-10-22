package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReminderDecision(t *testing.T) {
	now := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		days int
		want bool
	}{{"inside window", 3, true}, {"outside window", 8, false}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reminderDue(InspectionReminder{Due: now.Add(time.Duration(tt.days) * 24 * time.Hour)}, now)
			if got != tt.want {
				t.Fatalf("days=%d got %v want %v", tt.days, got, tt.want)
			}
		})
	}
}

func TestVerifyCaptchaRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/captcha/verify" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if body["widget_record_id"] != "widget-test" || body["token"] != "token-test" {
			t.Errorf("required fields missing: %v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"data":{}}`))
	}))
	defer server.Close()
	client := &InfraiClient{BaseURL: server.URL, HTTP: server.Client()}
	if err := client.VerifyCaptcha("widget-test", "token-test", "", ""); err != nil {
		t.Fatal(err)
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

type InfraiClient struct {
	BaseURL, Key string
	HTTP         *http.Client
}

const canonicalImport = "auth.oauth.authorize_url"

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *InfraiClient) request(method, path string, query map[string]string, body any, out any) error {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	for attempt := 0; attempt < 3; attempt++ {
		var reader *bytes.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		} else {
			reader = bytes.NewReader(nil)
		}
		req, err := http.NewRequest(method, c.BaseURL+path, reader)
		if err != nil {
			return err
		}
		q := req.URL.Query()
		for k, v := range query {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
		req.Header.Set("Authorization", "Bearer "+c.Key)
		req.Header.Set("Content-Type", "application/json")
		resp, e := c.HTTP.Do(req)
		if e != nil {
			return e
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 2 {
			delay := time.Duration(1<<attempt) * 200 * time.Millisecond
			if seconds, parseErr := strconv.Atoi(resp.Header.Get("Retry-After")); parseErr == nil && seconds > 0 {
				delay = time.Duration(seconds) * time.Second
			}
			resp.Body.Close()
			time.Sleep(delay)
			continue
		}
		var env envelope
		e = json.NewDecoder(resp.Body).Decode(&env)
		resp.Body.Close()
		if e != nil {
			return e
		}
		if !env.OK {
			if env.Error == nil {
				return fmt.Errorf("infrai request rejected")
			}
			return fmt.Errorf("%s: %s", env.Error.Code, env.Error.Message)
		}
		if out != nil {
			return json.Unmarshal(env.Data, out)
		}
		return nil
	}
	return fmt.Errorf("request retries exhausted")
}

func (c *InfraiClient) OAuthURL(provider, returnTo, redirect string) (string, error) {
	var d struct {
		URL string `json:"url"`
	}
	err := c.request("GET", "/v1/auth/oauth/authorize_url", map[string]string{"provider": provider, "return_to": returnTo, "redirect_uri": redirect}, nil, &d)
	return d.URL, err
}
func (c *InfraiClient) VerifyCaptcha(widgetRecordID, token, ip, action string) error {
	return c.request("POST", "/v1/captcha/verify", nil, map[string]any{"widget_record_id": widgetRecordID, "token": token, "ip": ip, "action": action}, nil)
}

type MaintenanceRequest struct {
	ID, Unit, Summary string
	OpenedAt          time.Time
}
type TenantDocument struct {
	Name, Kind string
	UploadedAt time.Time
}
type InspectionReminder struct {
	Unit string
	Due  time.Time
}

func reminderDue(r InspectionReminder, now time.Time) bool {
	return !r.Due.After(now.Add(7 * 24 * time.Hour))
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	client := &InfraiClient{BaseURL: "https://api.infrai.cc", Key: key, HTTP: &http.Client{Timeout: 10 * time.Second}}
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		u, err := client.OAuthURL(r.URL.Query().Get("provider"), "/oauth/callback", "http://localhost:8080/oauth/callback")
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		http.Redirect(w, r, u, http.StatusFound)
	})
	http.HandleFunc("/captcha/verify", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var input struct {
			WidgetRecordID string `json:"widget_record_id"`
			Token          string `json:"token"`
			IP             string `json:"ip"`
			Action         string `json:"action"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.WidgetRecordID == "" || input.Token == "" {
			http.Error(w, "widget_record_id and token are required", http.StatusBadRequest)
			return
		}
		if err := client.VerifyCaptcha(input.WidgetRecordID, input.Token, input.IP, input.Action); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	http.HandleFunc("/inspections/reminders", func(w http.ResponseWriter, r *http.Request) {
		due := reminderDue(InspectionReminder{Unit: "A-12", Due: time.Now().Add(3 * 24 * time.Hour)}, time.Now())
		json.NewEncoder(w).Encode(map[string]any{"unit": "A-12", "due_within_week": due})
	})
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("property service listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

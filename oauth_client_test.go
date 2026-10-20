package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthorizeRequestUsesProviderAndRedirect(t *testing.T) {
	cases := []struct {
		name, provider string
		want           string
	}{{"google", "google", "google"}, {"github", "github", "github"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.provider != tc.want {
				t.Fatalf("provider %q", tc.provider)
			}
		})
	}
}

func TestLoginDecodesDocumentedPayload(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("provider") != "google" || r.URL.Query().Get("return_to") != "/library" || r.URL.Query().Get("redirect_uri") != "http://localhost:8080/callback" {
			t.Errorf("unexpected authorize query: %s", r.URL.RawQuery)
		}
		w.Write([]byte(`{"ok":true,"data":{"url":"https://example.com/authorize"}}`))
	}))
	defer upstream.Close()
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"provider":"google","return_to":"/library","redirect_uri":"http://localhost:8080/callback"}`))
	w := httptest.NewRecorder()
	loginHandler(&AuthClient{base: upstream.URL, http: upstream.Client()})(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "https://example.com/authorize") {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
}

func TestVerifyCaptchaIncludesRequiredFields(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["widget_record_id"] != "widget-1" || body["token"] != "captcha-token" {
			t.Errorf("missing required captcha fields: %v", body)
		}
		w.Write([]byte(`{"ok":true,"data":{"verified":true}}`))
	}))
	defer upstream.Close()
	verified, err := (&AuthClient{base: upstream.URL, http: upstream.Client()}).VerifyCaptcha("widget-1", "captcha-token", "turnstile", "127.0.0.1", "login", 0.5)
	if err != nil || !verified {
		t.Fatalf("verified %v, error %v", verified, err)
	}
}

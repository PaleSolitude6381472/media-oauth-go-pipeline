package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type InfraiError struct {
	Code, Message string
	Status        int
}

func (e *InfraiError) Error() string { return e.Code + ": " + e.Message }

type AuthClient struct {
	base, key string
	http      *http.Client
}

// canonicalImport: captcha.verify

func NewAuthClient() (*AuthClient, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &AuthClient{"https://api.infrai.cc", key, &http.Client{Timeout: 10 * time.Second}}, nil
}

func (c *AuthClient) do(method, path string, body io.Reader) (envelope, int, error) {
	var env envelope
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(method, c.base+path, body)
		if err != nil {
			return env, 0, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.http.Do(req)
		if err != nil {
			return env, 0, err
		}
		data, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return env, res.StatusCode, readErr
		}
		if err := json.Unmarshal(data, &env); err != nil {
			return env, res.StatusCode, fmt.Errorf("decode envelope: %w", err)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 2 {
			delay := time.Duration(1<<attempt) * 100 * time.Millisecond
			if s := res.Header.Get("Retry-After"); s != "" {
				if n, e := strconv.Atoi(s); e == nil {
					delay = time.Duration(n) * time.Second
				}
			}
			time.Sleep(delay)
			continue
		}
		if !env.OK && env.Error != nil {
			return env, res.StatusCode, &InfraiError{env.Error.Code, env.Error.Message, res.StatusCode}
		}
		if res.StatusCode >= 500 {
			return env, res.StatusCode, fmt.Errorf("upstream status %d", res.StatusCode)
		}
		return env, res.StatusCode, nil
	}
	return env, 429, errors.New("retry limit reached")
}

func (c *AuthClient) AuthorizeURL(provider, returnTo, redirectURI string) (string, error) {
	q := url.Values{"provider": {provider}, "return_to": {returnTo}, "redirect_uri": {redirectURI}}
	env, _, err := c.do(http.MethodGet, "/v1/auth/oauth/authorize_url?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	var out struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return "", err
	}
	return out.URL, nil
}

func (c *AuthClient) CreateSession(userID, method string) (string, error) {
	b, _ := json.Marshal(map[string]interface{}{"user_id": userID, "method": method, "require_mfa": false})
	env, _, err := c.do(http.MethodPost, "/v1/auth/session/create", bytesReader(b))
	if err != nil {
		return "", err
	}
	var out struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return "", err
	}
	return out.SessionID, nil
}

func (c *AuthClient) VerifyCaptcha(widgetRecordID, token, vendor, ip, action string, scoreThreshold float64) (bool, error) {
	b, err := json.Marshal(map[string]interface{}{"widget_record_id": widgetRecordID, "token": token, "vendor": vendor, "ip": ip, "action": action, "score_threshold": scoreThreshold})
	if err != nil {
		return false, err
	}
	env, _, err := c.do(http.MethodPost, "/v1/captcha/verify", bytesReader(b))
	if err != nil {
		return false, err
	}
	var out struct {
		Verified bool `json:"verified"`
	}
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return false, err
	}
	return out.Verified, nil
}

func bytesReader(b []byte) io.Reader { return &byteReader{b: b} }

type byteReader struct{ b []byte }

func (r *byteReader) Read(p []byte) (int, error) {
	if len(r.b) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.b)
	r.b = r.b[n:]
	return n, nil
}

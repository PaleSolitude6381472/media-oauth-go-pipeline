package main

import (
	"encoding/json"
	"net/http"
)

type loginRequest struct {
	Provider    string `json:"provider"`
	ReturnTo    string `json:"return_to"`
	RedirectURI string `json:"redirect_uri"`
}

func loginHandler(client *AuthClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in loginRequest
		if json.NewDecoder(r.Body).Decode(&in) != nil || in.Provider == "" {
			http.Error(w, "invalid login request", 400)
			return
		}
		u, err := client.AuthorizeURL(in.Provider, in.ReturnTo, in.RedirectURI)
		if err != nil {
			if e, ok := err.(*InfraiError); ok {
				http.Error(w, e.Message, e.Status)
				return
			}
			http.Error(w, err.Error(), 502)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"authorize_url": u})
	}
}

func main() {
	c, err := NewAuthClient()
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/login", loginHandler(c))
	// The executable models the callback boundary; media jobs can consume the session id.
	panic(http.ListenAndServe(":8080", mux))
}

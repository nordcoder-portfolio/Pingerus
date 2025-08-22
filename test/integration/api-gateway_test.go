//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

const agBaseURL = "http://127.0.0.1:8080"

func httpPostJSON(t *testing.T, url string, body any, wantCode int) []byte {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("http POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != wantCode {
		t.Fatalf("http POST %s: got %d want %d body=%s", url, resp.StatusCode, wantCode, string(data))
	}
	return data
}

func httpGetAuth(t *testing.T, url, token string, wantCode int) []byte {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("http GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != wantCode {
		t.Fatalf("http GET %s: got %d want %d body=%s", url, resp.StatusCode, wantCode, string(data))
	}
	return data
}

func TestAuthAndChecks_Basic(t *testing.T) {
	email := "it-gw@example.com"
	pass := "supersecret"

	signupResp := httpPostJSON(t, agBaseURL+"/v1/auth/sign-up", map[string]string{
		"email":    email,
		"password": pass,
	}, 200)

	var su struct {
		AccessToken string `json:"accessToken"`
		User        struct {
			Id    json.Number `json:"id"`
			Email string      `json:"email"`
		} `json:"user"`
	}
	if err := json.Unmarshal(signupResp, &su); err != nil {
		t.Fatalf("unmarshal signup: %v body=%s", err, string(signupResp))
	}
	t.Logf("[signup] got token len=%d user=%s", len(su.AccessToken), su.User.Email)

	signinResp := httpPostJSON(t, agBaseURL+"/v1/auth/sign-in", map[string]string{
		"email":    email,
		"password": pass,
	}, 200)

	var si struct {
		AccessToken string `json:"accessToken"`
		User        struct {
			Id    json.Number `json:"id"`
			Email string      `json:"email"`
		} `json:"user"`
	}
	if err := json.Unmarshal(signinResp, &si); err != nil {
		t.Fatalf("unmarshal signin: %v body=%s", err, string(signinResp))
	}
	t.Logf("[signin] got token len=%d user=%s", len(si.AccessToken), si.User.Email)

	meResp := httpGetAuth(t, agBaseURL+"/v1/auth/me", si.AccessToken, 200)
	t.Logf("[me] body=%s", string(meResp))
}

func TestCheck_EnqueueKafka(t *testing.T) {
	email := "it-gw2@example.com"
	pass := "supersecret"

	_ = httpPostJSON(t, agBaseURL+"/v1/auth/sign-up", map[string]string{
		"email":    email,
		"password": pass,
	}, 200)

	signinResp := httpPostJSON(t, agBaseURL+"/v1/auth/sign-in", map[string]string{
		"email":    email,
		"password": pass,
	}, 200)
	var si struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(signinResp, &si); err != nil {
		t.Fatalf("unmarshal signin: %v body=%s", err, string(signinResp))
	}

	createReq := map[string]any{
		"user_id":      1,
		"url":          "http://example.com/ping",
		"interval_sec": 30,
	}
	reqBody, _ := json.Marshal(createReq)
	req, _ := http.NewRequest(http.MethodPost, agBaseURL+"/v1/checks", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+si.AccessToken)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("http POST check: %v", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("http POST check: got %d want 200 body=%s", resp.StatusCode, string(data))
	}
	t.Logf("[create check] ok: %s", string(data))
}

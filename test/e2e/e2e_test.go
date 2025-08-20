//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type cfg struct {
	APIBase     string // http://localhost:8080
	MailhogBase string // http://localhost:8025
	WaitEmail   time.Duration
}

func loadCfg() cfg {
	c := cfg{
		APIBase:     getenv("E2E_API_BASE", "http://localhost:8080"),
		MailhogBase: getenv("E2E_MAILHOG_BASE", "http://localhost:8025"),
		WaitEmail:   mustParseDur(getenv("E2E_WAIT_EMAIL", "30s")),
	}
	return c
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func mustParseDur(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return d
}

// --- DTOs под API
type authResp struct {
	AccessToken string `json:"accessToken"`
	User        struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
}

type createCheckReq struct {
	Url         string `json:"url"`
	IntervalSec int32  `json:"interval_sec"`
}

type checkResp struct {
	Check struct {
		ID string `json:"id"`
	} `json:"check"`
}

// --- Mailhog API v2 response (минимум полей)
type mailhogMessages struct {
	Count    int          `json:"count"`
	Total    int          `json:"total"`
	Start    int          `json:"start"`
	Messages []mailhogMsg `json:"items"`
}
type mailhogMsg struct {
	To      []mailhogPerson `json:"To"`
	Content struct {
		Headers map[string][]string `json:"Headers"`
		Body    string              `json:"Body"`
	} `json:"Content"`
}
type mailhogPerson struct {
	Mailbox string `json:"Mailbox"`
	Domain  string `json:"Domain"`
}

func (p mailhogPerson) Email() string {
	if p.Domain == "" {
		return p.Mailbox
	}
	return p.Mailbox + "@" + p.Domain
}

// --- helpers

func postJSON(t *testing.T, url string, in any, out any, bearer string) {
	t.Helper()
	b, _ := json.Marshal(in)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("POST %s => %d: %s", url, resp.StatusCode, string(body))
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			t.Fatalf("unmarshal %s: %v; body=%s", url, err, string(body))
		}
	}
}

func getJSON(t *testing.T, url string, into any) {
	t.Helper()
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	all, _ := io.ReadAll(resp.Body)
	require.NoError(t, json.Unmarshal(all, into))
}

// --- сам тест

func Test_CheckCreation_LeadsToEmail(t *testing.T) {
	c := loadCfg()

	for {
		t.Log("check api-gateway healthy!!!")
		resp, _ := http.Get(c.APIBase + "/healthz")
		if resp.StatusCode == 200 {
			resp.Body.Close()
			break
		}
		resp.Body.Close()
		time.Sleep(1 * time.Second)
	}

	email := fmt.Sprintf("e2e_%d@pingerus.dev", time.Now().UnixNano())
	pass := "P@ssw0rd!"

	var aresp authResp
	postJSON(t, c.APIBase+"/v1/auth/sign-up", map[string]string{
		"email":    email,
		"password": pass,
	}, &aresp, "")

	t.Log(aresp)

	require.NotEmpty(t, aresp.AccessToken)
	uid, err := strconv.ParseInt(aresp.User.ID, 10, 64)
	require.NoError(t, err)
	t.Logf("signed up as %s (id=%d)", aresp.User.Email, uid)

	var cresp checkResp
	postJSON(t, c.APIBase+"/v1/checks", createCheckReq{
		Url:         "http://http-echo",
		IntervalSec: 10,
	}, &cresp, aresp.AccessToken)

	require.NotZero(t, cresp.Check.ID)
	cid, err := strconv.ParseInt(cresp.Check.ID, 10, 64)
	require.NoError(t, err)
	t.Logf("check created (id=%d)", cid)

	deadline := time.Now().Add(c.WaitEmail)
	var lastErr error

	for time.Now().Before(deadline) {
		msgs := fetchMailhog(t, c, email)
		for _, m := range msgs {
			subj := headerFirst(m.Content.Headers, "Subject")
			if subj == "" {
				continue
			}
			if strings.Contains(subj, "Site status changed") {
				t.Logf("got email: %q", subj)
				return
			}
		}
		lastErr = fmt.Errorf("no email yet")
		time.Sleep(1 * time.Second)
	}
	require.NoError(t, lastErr, "email didn't arrive in time")
}

func fetchMailhog(t *testing.T, c cfg, toEmail string) []mailhogMsg {
	t.Helper()
	var out mailhogMessages
	getJSON(t, c.MailhogBase+"/api/v2/messages", &out)
	var res []mailhogMsg
	for _, m := range out.Messages {
		for _, rcpt := range m.To {
			if strings.EqualFold(rcpt.Email(), toEmail) {
				res = append(res, m)
				break
			}
		}
	}
	return res
}

func headerFirst(h map[string][]string, key string) string {
	for k, v := range h {
		if strings.EqualFold(k, key) && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

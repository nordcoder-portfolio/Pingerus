//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
)

/********** ENV CONFIG **********/

type Cfg struct {
	KafkaBootstrap string
	DBDSN          string
	MailhogAPI     string
	PWInTopic      string
	PWOutTopic     string
	PWHealthURL    string
	AGBaseURL      string
	AGUsersPath    string
	AGChecksPath   string
	AGEnqueueTmpl  string
}

func LoadCfg() Cfg {
	return Cfg{
		KafkaBootstrap: getenv("IT_BOOTSTRAP", "127.0.0.1:19092"),
		DBDSN:          getenv("IT_DB_DSN", "postgres://postgres:secret@127.0.0.1:55432/pingerus?sslmode=disable"),
		MailhogAPI:     getenv("IT_MAILHOG_API", "http://127.0.0.1:18025"),
		PWInTopic:      getenv("IT_PW_IN_TOPIC", "check-request"),
		PWOutTopic:     getenv("IT_PW_OUT_TOPIC", "status-change"),
		PWHealthURL:    getenv("IT_PW_HEALTH", "http://127.0.0.1:8083/healthz"),
		AGBaseURL:      getenv("IT_AG_BASE", "http://127.0.0.1:8080"),
		AGUsersPath:    getenv("IT_AG_USERS", "/v1/users"),
		AGChecksPath:   getenv("IT_AG_CHECKS", "/v1/checks"),
		AGEnqueueTmpl:  getenv("IT_AG_ENQ_TMPL", "/v1/checks/%d/enqueue"),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func TCPReachable(addr string, timeout time.Duration) error {
	d := net.Dialer{Timeout: timeout}
	c, err := d.Dial("tcp", addr)
	if err != nil {
		return err
	}
	_ = c.Close()
	return nil
}

func WaitTCP(t *testing.T, name, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		if err := TCPReachable(addr, 1500*time.Millisecond); err == nil {
			t.Logf("[it] %s ready at %s", name, addr)
			return
		} else {
			last = err
			time.Sleep(300 * time.Millisecond)
		}
	}
	t.Fatalf("[it] %s not reachable at %s: %v", name, addr, last)
}

func WaitHealthz(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == 200 {
			_ = resp.Body.Close()
			t.Logf("[it] healthz OK: %s", url)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("[it] healthz failed: %s", url)
}

func HTTPDoJSON(t *testing.T, method, url string, body []byte, want int) []byte {
	t.Helper()
	req, _ := http.NewRequest(method, url, bytesReader(body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("[http] %s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != want {
		t.Fatalf("[http] %s %s: got %d want %d, body=%s", method, url, resp.StatusCode, want, string(b))
	}
	return b
}

func bytesReader(b []byte) io.Reader {
	if b == nil {
		return nil
	}
	return strings.NewReader(string(b))
}

func EnsureTopic(t *testing.T, bootstrap, topic string) {
	t.Helper()
	WaitTCP(t, "kafka", bootstrap, 60*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	conn, err := kafka.DialContext(ctx, "tcp", bootstrap)
	if err != nil {
		t.Fatalf("[kafka] dial: %v", err)
	}
	defer conn.Close()

	if err := conn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	}); err != nil {
		t.Fatalf("[kafka] create topic %q: %v", topic, err)
	}
	parts, err := conn.ReadPartitions(topic)
	if err != nil || len(parts) == 0 {
		t.Fatalf("[kafka] partitions for %q: %v, len=%d", topic, err, len(parts))
	}
	t.Logf("[kafka] topic=%q partitions=%d leader=%s:%d", topic, len(parts), parts[0].Leader.Host, parts[0].Leader.Port)
}

func PublishProto(t *testing.T, bootstrap, topic string, key []byte, m proto.Message) {
	t.Helper()
	if err := TCPReachable(bootstrap, 2*time.Second); err != nil {
		t.Fatalf("[kafka] broker unreachable %s: %v", bootstrap, err)
	}
	w := &kafka.Writer{
		Addr:         kafka.TCP(bootstrap),
		Topic:        topic,
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireOne,
		Async:        false,
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Logf("[kafka] writer close: %v", err)
		}
	}()
	value, err := proto.Marshal(m)
	if err != nil {
		t.Fatalf("[kafka] marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := w.WriteMessages(ctx, kafka.Message{Key: key, Value: value}); err != nil {
		t.Fatalf("[kafka] write: %v", err)
	}
	t.Logf("[kafka] publish ok topic=%s key=%s len=%d", topic, string(key), len(value))
}

func ReadOneProto[T proto.Message](t *testing.T, bootstrap, topic, group string, timeout time.Duration, dst T) (T, bool) {
	t.Helper()
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{bootstrap},
		GroupID:  group,
		Topic:    topic,
		MinBytes: 1e3,
		MaxBytes: 10e6,
	})
	defer r.Close()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	msg, err := r.ReadMessage(ctx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			var zero T
			return zero, false
		}
		t.Fatalf("[kafka] read %s: %v", topic, err)
	}
	if err := proto.Unmarshal(msg.Value, dst); err != nil {
		t.Fatalf("[kafka] unmarshal: %v", err)
	}
	return dst, true
}

func DBOpen(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("[db] open: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("[db] ping: %v", err)
	}
	return db
}

func SeedUser(t *testing.T, db *sql.DB, id int64, email string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx, `
    insert into users (id, email, password_hash)
    values ($1, $2, $3)
    on conflict (id) do update set
      email = excluded.email,
      password_hash = excluded.password_hash
  `, id, email, "not_used_for_itests")
	if err != nil {
		t.Fatalf("[db] seed user: %v", err)
	}
}

func SeedCheck(t *testing.T, db *sql.DB, id, userID int64, host string, lastStatus *bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	if lastStatus == nil {
		_, _ = db.ExecContext(ctx, `
      insert into checks (id, user_id, host, interval_sec, next_run, active)
      values ($1, $2, $3, $4, $5, $6)
      on conflict (id) do update set
        user_id = excluded.user_id,
        host = excluded.host,
        interval_sec = excluded.interval_sec,
        next_run = excluded.next_run,
        active = excluded.active
    `, id, userID, host, 30, time.Now().UTC(), true)
	} else {
		_, _ = db.ExecContext(ctx, `
      insert into checks (id, user_id, host, interval_sec, last_status, next_run, active)
      values ($1, $2, $3, $4, $5, $6, $7)
      on conflict (id) do update set
        user_id = excluded.user_id,
        host = excluded.host,
        interval_sec = excluded.interval_sec,
        last_status = excluded.last_status,
        next_run = excluded.next_run,
        active = excluded.active
    `, id, userID, host, 30, *lastStatus, time.Now().UTC(), true)
	}
	_, _ = db.ExecContext(ctx, `
    update checks
    set
      last_status = coalesce(last_status, false),
      next_run = coalesce(next_run, now()),
      active = coalesce(active, true)
    where id = $1
  `, id)
}

func GetCheckLastStatus(t *testing.T, db *sql.DB, id int64) (sql.NullBool, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	var nb sql.NullBool
	err := db.QueryRowContext(ctx, `select last_status from checks where id = $1`, id).Scan(&nb)
	return nb, err
}

func FindNotification(t *testing.T, db *sql.DB, userID, checkID int64) (bool, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	var payload string
	err := db.QueryRowContext(ctx, `
    select payload
    from notifications
    where user_id = $1 and check_id = $2
    order by sent_at desc
    limit 1
  `, userID, checkID).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ""
	}
	if err != nil {
		t.Fatalf("[db] notifications: %v", err)
	}
	return true, payload
}

type MHResp struct {
	Total int
	Items []struct {
		Content struct {
			Headers map[string][]string `json:"Headers"`
			Body    string              `json:"Body"`
		} `json:"Content"`
	}
}

func MailhogPurge(t *testing.T, api string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, strings.TrimRight(api, "/")+"/api/v1/messages", nil)
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
}

func mailhogCountRaw(t *testing.T, api string) (int, MHResp, error) {
	t.Helper()
	url := strings.TrimRight(api, "/") + "/api/v2/messages"
	resp, err := http.Get(url)
	if err != nil {
		return 0, MHResp{}, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return 0, MHResp{}, fmt.Errorf("mailhog http %d: %s", resp.StatusCode, string(b))
	}
	var out MHResp
	if err := json.Unmarshal(b, &out); err != nil {
		return 0, MHResp{}, err
	}
	return out.Total, out, nil
}

func WaitMailhogCount(t *testing.T, api string, want int, timeout time.Duration) MHResp {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last MHResp
	for time.Now().Before(deadline) {
		n, r, err := mailhogCountRaw(t, api)
		if err == nil && n >= want {
			return r
		}
		time.Sleep(250 * time.Millisecond)
	}
	return last
}

func ExpectNoMailhog(t *testing.T, api string, duration time.Duration) {
	t.Helper()
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		n, _, err := mailhogCountRaw(t, api)
		if err == nil && n == 0 {
			time.Sleep(200 * time.Millisecond)
			n2, _, _ := mailhogCountRaw(t, api)
			if n2 == 0 {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("[mailhog] unexpected messages")
}

func RandID() int64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return int64(time.Now().Unix()%1_000_000)*1_000 + int64(b[0])
}

func KeyFromInt64(id int64) []byte {
	return []byte(strconv.FormatInt(id, 10))
}

//go:build integration

package integration

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	pb "github.com/NordCoder/Pingerus/generated/v1"
)

func TestPingWorker_StatusChanged_PublishesOne(t *testing.T) {
	cfg := LoadCfg()
	WaitTCP(t, "kafka", cfg.KafkaBootstrap, 60*time.Second)
	WaitHealthz(t, cfg.PWHealthURL, 90*time.Second)
	EnsureTopic(t, cfg.KafkaBootstrap, cfg.PWInTopic)
	EnsureTopic(t, cfg.KafkaBootstrap, cfg.PWOutTopic)

	db := DBOpen(t, cfg.DBDSN)
	defer db.Close()

	userID := RandID()
	checkID := RandID()
	email := fmt.Sprintf("pw-%d@example.com", userID)
	SeedUser(t, db, userID, email)
	SeedCheck(t, db, checkID, userID, "http://http-echo:80/", itPtrBool(false))

	outbox, _ := detectOutboxCount(t, db)

	PublishProto(t, cfg.KafkaBootstrap, cfg.PWInTopic, KeyFromInt64(checkID), &pb.CheckRequest{CheckId: int32(checkID)})

	var ev pb.StatusChange
	got, ok := ReadOneProto(t, cfg.KafkaBootstrap, cfg.PWOutTopic, "pw-it-status-1", 30*time.Second, &ev)
	if !ok {
		t.Fatalf("no status-change")
	}

	if int64(got.GetCheckId()) != checkID || !got.GetNewStatus() {
		t.Fatalf("wrong status-change: %+v", got)
	}
	var ev2 pb.StatusChange
	if _, ok2 := ReadOneProto(t, cfg.KafkaBootstrap, cfg.PWOutTopic, "pw-it-status-1", 2*time.Second, &ev2); ok2 {
		t.Fatalf("unexpected second status-change")
	}

	nb, err := GetCheckLastStatus(t, db, checkID)
	if err != nil || !nb.Valid || !nb.Bool {
		t.Fatalf("checks.last_status not true: %v valid=%v", err, nb.Valid)
	}

	if outbox.ok && outbox.table != "" {
		after := outbox.countFn()
		if after != outbox.base+1 {
			t.Fatalf("outbox mismatch: got=%d want=%d", after, outbox.base+1)
		}
	}
}

func TestPingWorker_NoChange_NoPublish(t *testing.T) {
	cfg := LoadCfg()
	WaitTCP(t, "kafka", cfg.KafkaBootstrap, 60*time.Second)
	WaitHealthz(t, cfg.PWHealthURL, 90*time.Second)
	EnsureTopic(t, cfg.KafkaBootstrap, cfg.PWInTopic)
	EnsureTopic(t, cfg.KafkaBootstrap, cfg.PWOutTopic)

	db := DBOpen(t, cfg.DBDSN)
	defer db.Close()

	userID := RandID()
	checkID := RandID()
	email := fmt.Sprintf("pw-nc-%d@example.com", userID)
	SeedUser(t, db, userID, email)
	SeedCheck(t, db, checkID, userID, "http://http-echo:80/", itPtrBool(true))

	outbox, _ := detectOutboxCount(t, db)

	PublishProto(t, cfg.KafkaBootstrap, cfg.PWInTopic, KeyFromInt64(checkID), &pb.CheckRequest{CheckId: int32(checkID)})

	var ev pb.StatusChange
	if _, ok := ReadOneProto(t, cfg.KafkaBootstrap, cfg.PWOutTopic, "pw-it-nochange", 3*time.Second, &ev); ok {
		t.Fatalf("unexpected status-change when no state changed")
	}

	nb, err := GetCheckLastStatus(t, db, checkID)
	if err != nil || !nb.Valid || !nb.Bool {
		t.Fatalf("checks.last_status should stay true: %v valid=%v", err, nb.Valid)
	}

	if outbox.ok && outbox.table != "" {
		after := outbox.countFn()
		if after != outbox.base {
			t.Fatalf("outbox grew unexpectedly: base=%d after=%d", outbox.base, after)
		}
	}
}

type outboxInfo struct {
	ok      bool
	table   string
	base    int64
	countFn func() int64
}

func detectOutboxCount(t *testing.T, db *sql.DB) (outboxInfo, error) {
	names := []string{"outbox", "event_outbox", "kafka_outbox"}
	for _, n := range names {
		var exists bool
		if err := db.QueryRow(`
      select exists(
        select 1 from information_schema.tables
        where table_schema = 'public' and table_name = $1
      )
    `, n).Scan(&exists); err == nil && exists {
			var base int64
			_ = db.QueryRow(fmt.Sprintf(`select count(1) from %s`, n)).Scan(&base)
			return outboxInfo{
				ok:    true,
				table: n,
				base:  base,
				countFn: func() int64 {
					var c int64
					_ = db.QueryRow(fmt.Sprintf(`select count(1) from %s`, n)).Scan(&c)
					return c
				},
			}, nil
		}
	}
	return outboxInfo{ok: false}, nil
}

func itPtrBool(b bool) *bool { return &b }

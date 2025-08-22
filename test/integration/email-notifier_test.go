//go:build integration

package integration

import (
	"fmt"
	"strings"
	"testing"
	"time"

	pb "github.com/NordCoder/Pingerus/generated/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestEmailNotifier_HappyPath(t *testing.T) {
	cfg := LoadCfg()
	MailhogPurge(t, cfg.MailhogAPI)
	EnsureTopic(t, cfg.KafkaBootstrap, cfg.PWOutTopic)

	db := DBOpen(t, cfg.DBDSN)
	defer db.Close()

	userID := RandID()
	checkID := RandID()
	email := fmt.Sprintf("en-%d@example.com", userID)
	host := "http://example.com/ping"

	SeedUser(t, db, userID, email)
	SeedCheck(t, db, checkID, userID, host, itPtrBool(false))

	msg := &pb.StatusChange{
		CheckId:   int32(checkID),
		OldStatus: false,
		NewStatus: true,
		Ts:        timestamppb.New(time.Now().UTC()),
	}
	PublishProto(t, cfg.KafkaBootstrap, cfg.PWOutTopic, KeyFromInt64(checkID), msg)

	rep := WaitMailhogCount(t, cfg.MailhogAPI, 1, 25*time.Second)
	if len(rep.Items) == 0 {
		t.Fatalf("no mail")
	}
	headers := rep.Items[0].Content.Headers
	body := rep.Items[0].Content.Body
	subj := ""
	if v, ok := headers["Subject"]; ok && len(v) > 0 {
		subj = v[0]
	}
	if !strings.Contains(subj, "Site status changed") {
		t.Fatalf("bad subject: %q", subj)
	}
	if !strings.Contains(body, host) || !strings.Contains(body, "false") || !strings.Contains(body, "true") {
		t.Fatalf("bad body: %q", body)
	}

	ok, payload := FindNotification(t, db, userID, checkID)
	if !ok || payload == "" {
		t.Fatalf("notification not stored")
	}
}

func TestEmailNotifier_InvalidCheckID_Ignored(t *testing.T) {
	cfg := LoadCfg()
	MailhogPurge(t, cfg.MailhogAPI)
	EnsureTopic(t, cfg.KafkaBootstrap, cfg.PWOutTopic)

	msg := &pb.StatusChange{
		CheckId:   0,
		OldStatus: true,
		NewStatus: false,
		Ts:        timestamppb.New(time.Now().UTC()),
	}
	PublishProto(t, cfg.KafkaBootstrap, cfg.PWOutTopic, []byte("0"), msg)
	ExpectNoMailhog(t, cfg.MailhogAPI, 6*time.Second)
}

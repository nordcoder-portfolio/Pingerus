package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

func main() {
	broker := env("KAFKA_BROKER", "kafka:9092")
	topics := strings.Split(env("KAFKA_TOPICS", ""), ",")
	partitions := envInt("KAFKA_PARTITIONS", 1)
	rf := envInt("KAFKA_RF", 1)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for _, t := range topics {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if err := ensureTopic(ctx, broker, t, partitions, rf); err != nil {
			log.Fatalf("ensure topic %q: %v", t, err)
		}
		if err := waitTopicReady(ctx, broker, t); err != nil {
			log.Fatalf("wait topic %q: %v", t, err)
		}
		log.Printf("topic %q ready", t)
	}
	log.Println("kafka-init ok")
}

func ensureTopic(ctx context.Context, broker, topic string, parts, rf int) error {
	conn, err := kafka.DialContext(ctx, "tcp", broker)
	if err != nil {
		return err
	}
	defer conn.Close()

	ctrl, err := conn.Controller()
	if err != nil {
		return err
	}
	ctrlAddr := net.JoinHostPort(ctrl.Host, strconv.Itoa(ctrl.Port))

	cconn, err := kafka.DialContext(ctx, "tcp", ctrlAddr)
	if err != nil {
		return err
	}
	defer cconn.Close()

	tc := kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     parts,
		ReplicationFactor: rf,
	}

	if err := cconn.CreateTopics(tc); err != nil && !strings.Contains(err.Error(), "already exists") {
		return err
	}
	return nil
}

func waitTopicReady(ctx context.Context, broker, topic string) error {
	backoff := 200 * time.Millisecond
	max := 5 * time.Second
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := kafka.Dial("tcp", broker)
		if err == nil {
			parts, e2 := conn.ReadPartitions(topic)
			conn.Close()
			if e2 == nil && len(parts) > 0 && allHaveLeader(parts) {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < max {
			backoff *= 2
		}
	}
	return fmt.Errorf("topic %s not ready in time", topic)
}

func allHaveLeader(parts []kafka.Partition) bool {
	for _, p := range parts {
		if p.Leader.ID == -1 {
			return false
		}
	}
	return true
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 {
			return n
		}
	}
	return def
}

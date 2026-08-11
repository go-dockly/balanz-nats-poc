package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	defaultNATSURL = "nats://localhost:4222"
	streamName     = "METER_DATA"
	consumerName   = "demo-consumer"
)

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = defaultNATSURL
	}

	nc, err := nats.Connect(natsURL, nats.Name("balanz-meter-consumer"))
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("jetstream: %v", err)
	}

	ctx := context.Background()

	// Wait a moment for the stream to exist (created by nats-server).
	var stream jetstream.Stream
	for i := 0; i < 20; i++ {
		stream, err = js.Stream(ctx, streamName)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		log.Fatalf("stream %q not found: %v (start nats-server first)", streamName, err)
	}

	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: "meter.reading",
		DeliverPolicy: jetstream.DeliverNewPolicy,
	})
	if err != nil {
		log.Fatalf("consumer: %v", err)
	}

	log.Printf("consuming from stream %q subject meter.reading ...", streamName)

	cc, err := cons.Consume(func(msg jetstream.Msg) {
		var payload map[string]interface{}
		if err := json.Unmarshal(msg.Data(), &payload); err != nil {
			log.Printf("bad payload: %v", err)
			_ = msg.Ack()
			return
		}

		meta, _ := msg.Metadata()
		fmt.Printf("\n─── Meter Reading ──────────────────────────────\n")
		fmt.Printf("  seq        : %d\n", meta.Sequence.Stream)
		fmt.Printf("  asset_id   : %v\n", payload["asset_id"])
		fmt.Printf("  meter_id   : %v\n", payload["meter_id"])
		fmt.Printf("  power_kw   : %v\n", payload["active_power_kw"])
		fmt.Printf("  energy_kwh : %v\n", payload["energy_kwh"])
		fmt.Printf("  timestamp  : %v\n", payload["timestamp"])
		fmt.Printf("────────────────────────────────────────────────\n")

		_ = msg.Ack()
	})
	if err != nil {
		log.Fatalf("consume: %v", err)
	}
	defer cc.Stop()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("consumer stopped")
}

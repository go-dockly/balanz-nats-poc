package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	meterv1 "github.com/balanz-energy/nats-poc/gen/meter/v1"
)

const defaultIngestAddr = "localhost:50051"

func main() {
	addr := os.Getenv("INGEST_ADDR")
	if addr == "" {
		addr = defaultIngestAddr
	}

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	client := meterv1.NewIngestServiceClient(conn)

	assets := []string{"NL-SOLAR-001", "NL-SOLAR-042", "NL-WIND-007", "NL-BESS-003"}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for i := 0; i < 5; i++ {
		asset := assets[rand.Intn(len(assets))]
		power := 50 + rand.Float64()*200 // 50-250 kW
		energy := power * 0.25           // rough 15-min interval

		req := &meterv1.IngestReadingRequest{
			Reading: &meterv1.MeterReading{
				AssetId:       asset,
				MeterId:       "M-" + asset[len(asset)-3:],
				ActivePowerKw: power,
				EnergyKwh:     energy,
				Timestamp:     timestamppb.Now(),
				Labels: map[string]string{
					"region": "NL",
					"type":   "renewable",
				},
			},
		}

		resp, err := client.IngestReading(ctx, req)
		if err != nil {
			log.Printf("ingest error: %v", err)
			continue
		}

		if resp.Accepted {
			fmt.Printf("✓ accepted  asset=%-14s power=%6.1f kW  msg_id=%s\n",
				asset, power, resp.MessageId)
		} else {
			fmt.Printf("✗ rejected  asset=%-14s  err=%v\n", asset, resp.Error)
		}

		time.Sleep(300 * time.Millisecond)
	}
}

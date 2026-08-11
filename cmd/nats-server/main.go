package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commonv1 "github.com/balanz-energy/nats-poc/gen/common/v1"
	natsv1 "github.com/balanz-energy/nats-poc/gen/nats/v1"
)

const (
	defaultNATSURL = "nats://localhost:4222"
	streamName     = "METER_DATA"
	defaultSubject = "meter.reading"
	listenAddr     = ":50052"
)

type publisherServer struct {
	natsv1.UnimplementedNatsPublisherServiceServer
	js jetstream.JetStream
}

func (s *publisherServer) PublishMeterReading(ctx context.Context, req *natsv1.PublishMeterReadingRequest) (*natsv1.PublishMeterReadingResponse, error) {
	if req.Reading == nil {
		return &natsv1.PublishMeterReadingResponse{
			Published: false,
			Error: &commonv1.DomainError{
				Code:    "INVALID_ARGUMENT",
				Message: "reading is required",
			},
		}, nil
	}

	subject := req.Subject
	if subject == "" {
		subject = defaultSubject
	}

	// Serialize as JSON for the PoC. Production would typically use protobuf bytes
	// or a well-defined envelope with schema versioning.
	payload, err := json.Marshal(map[string]interface{}{
		"asset_id":        req.Reading.AssetId,
		"meter_id":        req.Reading.MeterId,
		"active_power_kw": req.Reading.ActivePowerKw,
		"energy_kwh":      req.Reading.EnergyKwh,
		"timestamp":       req.Reading.Timestamp.AsTime().UTC().Format(time.RFC3339Nano),
		"labels":          req.Reading.Labels,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal: %v", err)
	}

	ack, err := s.js.Publish(ctx, subject, payload)
	if err != nil {
		log.Printf("publish failed: %v", err)
		return &natsv1.PublishMeterReadingResponse{
			Published: false,
			Error: &commonv1.DomainError{
				Code:    "PUBLISH_FAILED",
				Message: err.Error(),
			},
		}, nil
	}

	log.Printf("published to %s seq=%d asset=%s power=%.2f kW",
		subject, ack.Sequence, req.Reading.AssetId, req.Reading.ActivePowerKw)

	return &natsv1.PublishMeterReadingResponse{
		Published: true,
		Subject:   subject,
		Sequence:  ack.Sequence,
	}, nil
}

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = defaultNATSURL
	}

	nc, err := nats.Connect(natsURL,
		nats.Name("balanz-nats-publisher"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		log.Fatalf("nats connect: %v", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("jetstream: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      streamName,
		Subjects:  []string{"meter.>"},
		Retention: jetstream.LimitsPolicy,
		MaxAge:    24 * time.Hour,
		Storage:   jetstream.FileStorage,
		Replicas:  1,
	})
	if err != nil {
		log.Fatalf("create stream: %v", err)
	}
	log.Printf("JetStream stream %q ready", streamName)

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	natsv1.RegisterNatsPublisherServiceServer(grpcServer, &publisherServer{js: js})

	go func() {
		log.Printf("NATS Publisher gRPC listening on %s", listenAddr)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("serve: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down...")
	grpcServer.GracefulStop()
}

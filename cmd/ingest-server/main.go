package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	commonv1 "github.com/balanz-energy/nats-poc/gen/common/v1"
	meterv1 "github.com/balanz-energy/nats-poc/gen/meter/v1"
	natsv1 "github.com/balanz-energy/nats-poc/gen/nats/v1"
)

const (
	listenAddr          = ":50051"
	defaultNATSGRPCAddr = "localhost:50052"
)

type ingestServer struct {
	meterv1.UnimplementedIngestServiceServer
	natsClient natsv1.NatsPublisherServiceClient
}

func (s *ingestServer) IngestReading(ctx context.Context, req *meterv1.IngestReadingRequest) (*meterv1.IngestReadingResponse, error) {
	if req.Reading == nil {
		return &meterv1.IngestReadingResponse{
			Accepted: false,
			Error: &commonv1.DomainError{
				Code:    "INVALID_ARGUMENT",
				Message: "reading is required",
			},
		}, nil
	}

	// Call the dedicated NATS publisher service over gRPC.
	pubResp, err := s.natsClient.PublishMeterReading(ctx, &natsv1.PublishMeterReadingRequest{
		Reading: req.Reading,
		Subject: "meter.reading",
	})
	if err != nil {
		log.Printf("call to nats publisher failed: %v", err)
		return nil, status.Errorf(codes.Internal, "nats publisher unavailable: %v", err)
	}

	if !pubResp.Published {
		msg := "publish rejected"
		if pubResp.Error != nil {
			msg = pubResp.Error.Message
		}
		return &meterv1.IngestReadingResponse{
			Accepted: false,
			Error: &commonv1.DomainError{
				Code:    "PUBLISH_FAILED",
				Message: msg,
			},
		}, nil
	}

	log.Printf("ingested asset=%s power=%.2f kW → NATS seq=%d",
		req.Reading.AssetId, req.Reading.ActivePowerKw, pubResp.Sequence)

	return &meterv1.IngestReadingResponse{
		Accepted:  true,
		MessageId: formatSeq(pubResp.Sequence),
	}, nil
}

func formatSeq(seq uint64) string {
	if seq == 0 {
		return "js-0"
	}
	var b [20]byte
	i := len(b)
	u := seq
	for u > 0 {
		i--
		b[i] = byte('0' + u%10)
		u /= 10
	}
	return "js-" + string(b[i:])
}

func main() {
	natsGRPC := os.Getenv("NATS_GRPC_ADDR")
	if natsGRPC == "" {
		natsGRPC = defaultNATSGRPCAddr
	}

	conn, err := grpc.NewClient(natsGRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial nats publisher: %v", err)
	}
	defer conn.Close()

	natsClient := natsv1.NewNatsPublisherServiceClient(conn)

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	meterv1.RegisterIngestServiceServer(grpcServer, &ingestServer{natsClient: natsClient})

	go func() {
		log.Printf("Ingest gRPC listening on %s (will call NATS publisher at %s)", listenAddr, natsGRPC)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("serve: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down ingest server...")
	grpcServer.GracefulStop()
}

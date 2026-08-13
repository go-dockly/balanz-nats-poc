package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-dockly/balanz-nats-poc/pkg/clickhouse"
	"github.com/go-dockly/balanz-nats-poc/pkg/util"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

const (
	defaultNATSURL        = "nats://localhost:4222"
	defaultClickHouseAddr = "clickhouse://default:pass@localhost:9000/default"
	streamName            = "METER_DATA"
	consumerName          = "clickhouse-consumer"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := clickhouse.NewStore(ctx, logger, util.GetEnv("CLICKHOUSE_ADDR", defaultClickHouseAddr))
	if err != nil {
		logger.Error("clickhouse.NewStore", zap.Error(err))
		return
	}

	nc, err := nats.Connect(util.GetEnv("NATS_URL", defaultNATSURL), nats.Name("balanz-clickhouse-consumer"))
	if err != nil {
		logger.Error("nats.Connect", zap.Error(err))
		return
	}
	js, err := jetstream.New(nc)
	if err != nil {
		logger.Error("jetstream.New", zap.Error(err))
		return
	}

	stream, err := util.WaitForStream(ctx, js, streamName)
	if err != nil {
		logger.Error("waitForStream", zap.Error(err))
		return
	}

	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: "meter.reading",
		DeliverPolicy: jetstream.DeliverNewPolicy,
	})
	if err != nil {
		logger.Error("stream.CreateOrUpdateConsumer", zap.Error(err))
		return
	}
	logger.Info("clickhouse consumer ready",
		zap.String("stream", streamName),
		zap.String("consumer", consumerName),
	)

	cc, err := cons.Consume(func(msg jetstream.Msg) {
		var r clickhouse.Reading
		if err := json.Unmarshal(msg.Data(), &r); err != nil {
			logger.Error("bad payload", zap.Error(err))
			_ = msg.Term()
			return
		}

		meta, err := msg.Metadata()
		if err != nil {
			logger.Error("msg.Metadata", zap.Error(err))
			_ = msg.Term()
			return
		}

		if err := ch.InsertMeterReading(ctx, &r, meta.Sequence.Stream); err != nil {
			logger.Error("InsertMeterReading",
				zap.Error(err),
				zap.String("asset_id", r.AssetID),
				zap.Uint64("nats_seq", meta.Sequence.Stream),
			)
			_ = msg.Nak() // redelivery
			return
		}
		logger.Debug("InsertMeterReading success",
			zap.String("asset_id", r.AssetID),
			zap.Float64("power_kw", r.ActivePowerKW),
			zap.Uint64("nats_seq", meta.Sequence.Stream),
		)
		_ = msg.Ack()
	}, jetstream.ConsumeErrHandler(func(_ jetstream.ConsumeContext, err error) {
		logger.Error("consume error", zap.Error(err))
	}))
	if err != nil {
		logger.Fatal("cons.Consume", zap.Error(err))
	}
	defer cc.Stop()

	// graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	logger.Info("shutting down clickhouse consumer")
	cancel()
}

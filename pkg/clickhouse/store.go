package clickhouse

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

type Store struct {
	logger *zap.Logger
	conn   driver.Conn
}

// NewStore creates the database if needed, connects and auto-migrates the schema
func NewStore(ctx context.Context, logger *zap.Logger, chDSN string) (s *Store, err error) {
	s = &Store{logger: logger}

	params, err := ParseDSN(chDSN, "default", "default", "")
	if err != nil {
		return nil, fmt.Errorf("parse clickhouse dsn: %w", err)
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{params.Addr},
		Auth: clickhouse.Auth{
			Database: params.Database,
			Username: params.Username,
			Password: params.Password,
		},
		TLS: func() *tls.Config {
			if params.Secure {
				return &tls.Config{}
			}
			return nil
		}(),
	})
	if err != nil {
		return nil, fmt.Errorf("clickhouse open: %w", err)
	}
	s.conn = conn

	if err := s.conn.Ping(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}

	if err := s.ensureTable(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ensure table: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.conn.Close()
}

func (s *Store) ensureTable(ctx context.Context) error {
	return s.conn.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS meter_readings (
            asset_id         String,
            meter_id         String,
            active_power_kw  Float64,
            energy_kwh       Float64,
            ts               DateTime64(9, 'UTC'),
            nats_seq         UInt64,
            ingested_at      DateTime64(3, 'UTC') DEFAULT now64(3),
            labels           Map(String, String)
        ) ENGINE = MergeTree()
        ORDER BY (asset_id, ts)
        TTL toDateTime(ts) + INTERVAL 90 DAY
    `)
}

// InsertMeterReading single-row insert; use batch for throughput
func (s *Store) InsertMeterReading(ctx context.Context, r *Reading, natsSeq uint64) error {
	ts, err := time.Parse(time.RFC3339Nano, r.Timestamp)
	if err != nil {
		// fallback – still store something rather than drop the message
		ts = time.Now().UTC()
		s.logger.Warn("bad timestamp, using now", zap.String("raw", r.Timestamp), zap.Error(err))
	}

	labels := r.Labels
	if labels == nil {
		labels = map[string]string{}
	}

	return s.conn.Exec(ctx, `
    INSERT INTO meter_readings
        (asset_id, meter_id, active_power_kw, energy_kwh, ts, nats_seq, labels)
    VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.AssetID,
		r.MeterID,
		r.ActivePowerKW,
		r.EnergyKWh,
		ts,
		natsSeq,
		labels,
	)
}

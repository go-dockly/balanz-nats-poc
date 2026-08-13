package clickhouse

// Reading matches the JSON published by nats-server.
type Reading struct {
	AssetID       string            `json:"asset_id"`
	MeterID       string            `json:"meter_id"`
	ActivePowerKW float64           `json:"active_power_kw"`
	EnergyKWh     float64           `json:"energy_kwh"`
	Timestamp     string            `json:"timestamp"` // RFC3339Nano
	Labels        map[string]string `json:"labels,omitempty"`
}

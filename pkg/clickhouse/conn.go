package clickhouse

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ConnParams holds the resolved connection settings for clickhouse-go
type ConnParams struct {
	Addr     string // host:port
	Database string
	Username string
	Password string
	Secure   bool // true if scheme requested TLS (clickhouses://)
}

// ParseDSN accepts either:
//   - clickhouse://user:pass@host:9000/dbname?secure=true
//   - clickhouses://user:pass@host:9440/dbname   (implies TLS)
//   - a bare host:port (e.g. "localhost:9000"), in which case
//     database/username/password fall back to the provided defaults
func ParseDSN(dsn, defaultDB, defaultUser, defaultPass string) (ConnParams, error) {
	p := ConnParams{
		Database: defaultDB,
		Username: defaultUser,
		Password: defaultPass,
	}

	// Bare "host:port" (no scheme) to avoid url.Parse interpreting
	// the host as a scheme when colon is present
	if !strings.Contains(dsn, "://") {
		p.Addr = dsn
		return p, nil
	}

	u, err := url.Parse(dsn)
	if err != nil {
		return p, fmt.Errorf("parse clickhouse dsn: %w", err)
	}

	switch u.Scheme {
	case "clickhouse":
		p.Secure = false
	case "clickhouses":
		p.Secure = true
	default:
		return p, fmt.Errorf("unsupported clickhouse dsn scheme %q", u.Scheme)
	}

	if u.Host == "" {
		return p, fmt.Errorf("clickhouse dsn missing host: %q", dsn)
	}
	p.Addr = u.Host // includes port if present eg "host:9000"

	if u.User != nil {
		if uname := u.User.Username(); uname != "" {
			p.Username = uname
		}
		if pass, ok := u.User.Password(); ok {
			p.Password = pass
		}
	}

	if db := strings.Trim(u.Path, "/"); db != "" {
		p.Database = db
	}

	if q := u.Query(); q.Get("secure") != "" {
		if b, err := strconv.ParseBool(q.Get("secure")); err == nil {
			p.Secure = b
		}
	}

	return p, nil
}

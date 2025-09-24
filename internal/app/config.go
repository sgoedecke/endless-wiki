package app

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

// Config contains runtime configuration derived from environment variables.
type Config struct {
	DSN        string
	Port       string
	GroqAPIKey string
}

// LoadConfig populates Config from environment variables, applying reasonable defaults.
func LoadConfig() (Config, error) {
	cfg := Config{
		Port:       defaultEnv("PORT", "8080"),
		GroqAPIKey: os.Getenv("GROQ_API_KEY"),
	}

	rawDSN := os.Getenv("MYSQL_DSN")
	if rawDSN == "" {
		rawDSN = os.Getenv("DATABASE_URL")
	}
	if rawDSN == "" {
		return cfg, fmt.Errorf("missing MYSQL_DSN or DATABASE_URL environment variable")
	}

	normalized, err := normalizeMySQLDSN(rawDSN)
	if err != nil {
		return cfg, err
	}
	cfg.DSN = normalized

	return cfg, nil
}

// normalizeMySQLDSN converts URLs like mysql://user:pass@host:port/db into Go-MySQL DSNs.
func normalizeMySQLDSN(input string) (string, error) {
	if !strings.Contains(input, "://") {
		// assume it is already a DSN the driver understands
		return appendDefaultParams(input), nil
	}

	u, err := url.Parse(input)
	if err != nil {
		return "", fmt.Errorf("parse mysql url: %w", err)
	}

	if u.Scheme != "mysql" && u.Scheme != "mariadb" {
		return "", fmt.Errorf("unsupported database scheme %q", u.Scheme)
	}

	if u.Host == "" {
		return "", fmt.Errorf("missing host in database url")
	}

	user := ""
	if u.User != nil {
		user = u.User.Username()
		if password, ok := u.User.Password(); ok {
			user = fmt.Sprintf("%s:%s", user, password)
		}
	}

	host := u.Host
	if !strings.Contains(host, ":") {
		host = net.JoinHostPort(host, "3306")
	}

	path := strings.TrimPrefix(u.Path, "/")
	if path == "" {
		return "", fmt.Errorf("missing database name in url path")
	}

	params := u.RawQuery
	if params != "" {
		params = "?" + params
	}

	dsn := fmt.Sprintf("%s@tcp(%s)/%s%s", user, host, path, params)
	return appendDefaultParams(dsn), nil
}

func defaultEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func appendDefaultParams(dsn string) string {
	if !strings.Contains(dsn, "parseTime=") {
		separator := "?"
		if strings.Contains(dsn, "?") {
			separator = "&"
		}
		dsn = dsn + separator + "parseTime=true"
	}
	return dsn
}

package config

import (
	"errors"
	"strings"

	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Postgres PostgresConfig
}

type PostgresConfig struct {
	DSN string
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Postgres.DSN) == "" {
		return errors.New("postgres dsn is empty")
	}
	return nil
}

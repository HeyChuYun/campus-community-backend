package config

import (
	"errors"
	"strings"

	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.ListenOn) == "" {
		return errors.New("listen on is empty")
	}
	return nil
}

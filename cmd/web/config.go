package web

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/trento-project/trento/internal/db"
	"github.com/trento-project/trento/web"
)

func LoadConfig() (*web.Config, error) {
	enablemTLS := viper.GetBool("enable-mtls")
	cert := viper.GetString("cert")
	key := viper.GetString("key")
	ca := viper.GetString("ca")

	if enablemTLS {
		var err error

		if cert == "" {
			err = fmt.Errorf("you must provide a server ssl certificate")
		}
		if key == "" {
			err = errors.Wrap(err, "you must provide a key to enable mTLS")
		}
		if ca == "" {
			err = errors.Wrap(err, "you must provide a CA ssl certificate")
		}
		if err != nil {
			return nil, err
		}
	}

	return &web.Config{
		Host:          viper.GetString("host"),
		Port:          viper.GetInt("port"),
		CollectorPort: viper.GetInt("collector-port"),
		EnablemTLS:    enablemTLS,
		Cert:          cert,
		Key:           key,
		CA:            ca,
		DBConfig:      LoadDBConfig(),
	}, nil
}

func LoadDBConfig() *db.Config {
	return &db.Config{
		Host:     viper.GetString("db-host"),
		Port:     viper.GetString("db-port"),
		User:     viper.GetString("db-user"),
		Password: viper.GetString("db-password"),
		DBName:   viper.GetString("db-name"),
	}
}

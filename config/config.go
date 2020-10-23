// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package config

import (
	"errors"
	"github.com/go-ini/ini"
	"github.com/sirupsen/logrus"
)

type Logging struct {
	Level string `ini:"level"`
}

var logging = &Logging{
	Level: "info",
}

type RedisInfo struct {
	Host     string `ini:"host"`
	Port     string `ini:"port"`
	User     string `ini:"user"`
	Password string `ini:"password"`
	PoolSize int    `ini:"pool_size"`
}

var redisInfo = &RedisInfo{
	Port:     "6380",
	PoolSize: 64,
}

type DbInfo struct {
	Host         string `ini:"host"`
	Port         string `ini:"port"`
	Database     string `ini:"database"`
	User         string `ini:"user"`
	Password     string `ini:"password"`
	MaxOpenConns int    `ini:"max_open_conns"`
}

var dbDriver string

var dbInfo = &DbInfo{
	Port:         "3306",
	Database:     "icingadb",
	MaxOpenConns: 50,
}

type MetricsInfo struct {
	Host string `ini:"host"`
	Port string `ini:"port"`
}

var metricsInfo = &MetricsInfo{
	Port: "8080",
}

func ParseConfig(path string) error {
	cfg, err := ini.Load(path)
	if err != nil {
		return err
	}

	if err := cfg.Section("logging").MapTo(logging); err != nil {
		return err
	}

	_, err = logrus.ParseLevel(logging.Level)
	if err != nil {
		return err
	}

	if err := cfg.Section("redis").MapTo(redisInfo); err != nil {
		return err
	}

	if redisInfo.Host == "" {
		return errors.New("missing redis host")
	}

	var db *ini.Section

	if db, err = cfg.GetSection("mysql"); err == nil {
		if _, err = cfg.GetSection("pgsql"); err == nil {
			return errors.New("too many databases")
		} else {
			dbDriver = "mysql"
		}
	} else {
		if db, err = cfg.GetSection("pgsql"); err == nil {
			dbDriver = "postgres"
		} else {
			return errors.New("missing database")
		}
	}

	if err = db.MapTo(dbInfo); err != nil {
		return err
	}

	if err = cfg.Section("metrics").MapTo(metricsInfo); err != nil {
		return err
	}

	if dbInfo.Host == "" {
		return errors.New("missing database host")
	}
	if dbInfo.User == "" || dbInfo.Password == "" {
		return errors.New("missing database credentials")
	}

	return nil
}

func GetLogging() *Logging {
	return logging
}

func GetDbInfo() (driver string, info *DbInfo) {
	return dbDriver, dbInfo
}

func GetRedisInfo() *RedisInfo {
	return redisInfo
}

func GetMetricsInfo() *MetricsInfo {
	return metricsInfo
}

package config

import (
	"errors"
	"github.com/go-ini/ini"
	"github.com/sirupsen/logrus"
)

type Logging struct {
	Level    string `ini:"level"`
}

var logging = &Logging{
	Level: "info",
}

type RedisInfo struct {
	Host     string `ini:"host"`
	Port     string `ini:"port"`
	User     string `ini:"user"`
	Password string `ini:"password"`
}

var redisInfo = &RedisInfo{
	Port: "6379",
}

type MysqlInfo struct {
	Host     string `ini:"host"`
	Port     string `ini:"port"`
	Database string `ini:"database"`
	User     string `ini:"user"`
	Password string `ini:"password"`
}

var mysqlInfo = &MysqlInfo{
	Port:     "3306",
	Database: "icingadb",
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

	if err = cfg.Section("mysql").MapTo(mysqlInfo); err != nil {
		return err
	}

	if mysqlInfo.Host == "" {
		return errors.New("missing mysql host")
	}
	if mysqlInfo.User == "" || mysqlInfo.Password == "" {
		return errors.New("missing mysql credentials")
	}

	return nil
}

func GetLogging() *Logging {
	return logging
}

func GetMysqlInfo() *MysqlInfo {
	return mysqlInfo
}

func GetRedisInfo() *RedisInfo {
	return redisInfo
}

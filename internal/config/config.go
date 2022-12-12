package config

import (
	"bytes"
	"io/ioutil"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const cfgFile = "config/config.toml"

var c Config

type General struct {
	Version       string
	Log_level     uint8 `mapstructure:"log_level"`
	Log_colors    bool  `mapstructure:"log_disable_colors"`
	Log_timestamp bool  `mapstructure:"log_disable_timestamp"`
	N_routines    int   `mapstructure:"n_routines"`
}

type ScraperConfig struct {
	FilePath  string   `mapstructure:"file_path"`
	PortRange []string `mapstructure:"port_range"`
	UserAgent string   `mapstructure:"user_agent"`
}

type Database struct {
	User         string `mapstructure:"db_user"`
	Password     string `mapstructure:"db_password"`
	Host         string `mapstructure:"db_host"`
	Port         string `mapstructure:"db_port"`
	Name         string `mapstructure:"db_name"`
	ClearDBTable bool   `mapstructure:"clear_db_table"`
}

type Config struct {
	General       General
	ScraperConfig ScraperConfig `mapstructure:"scraper"`
	Database      Database
}

func init() {
	viper.Set("general.version", "v.1.0.2")
	viper.SetDefault("general.log_level", 2)
	viper.SetDefault("general.log_disable_colors", false)
	viper.SetDefault("general.log_disable_timestamp", false)
	viper.SetDefault("general.n_routines", 50)
	viper.SetDefault("database.db_host", "mariadb")
	viper.SetDefault("database.db_name", "port_scraper")
	viper.SetDefault("database.db_user", "root")
	viper.SetDefault("database.db_password", "root")
	viper.SetDefault("database.db_port", "3306")
	viper.SetDefault("database.clear_db_table", false)
	viper.SetDefault("database.file_path", "./config/test.csv")
	viper.SetDefault("database.port_range", []string{"22", "80", "8080", "443", "8443", "1883", "8883", "9092", "1880", "3000", "8123", "32400", "10011", "3306", "27017", "5432", "6379", "8086", "1521", "9200", "25565", "27015"})
	viper.SetDefault("database.user_agent", "Mozilla/5.0 (compatible; PortScraper/1.0; +https://YOURDOMAIN.COM)")

	init_config()
	viper.WriteConfigAs(cfgFile)
}

func init_config() {
	b, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		log.Error("error loading config file")
	}
	viper.SetConfigType("toml")
	if err := viper.ReadConfig(bytes.NewBuffer(b)); err != nil {
		log.Error("error loading config file")
	}
	if err := viper.Unmarshal(&c); err != nil {
		log.Error("unmarshal config file error")
	}
	// var log = logrus.New()

	// log.Formatter.(*logrus.TextFormatter).DisableTimestamp = c.General.Log_timestamp
	log.SetFormatter(&log.TextFormatter{
		DisableColors:    c.General.Log_colors,
		DisableTimestamp: c.General.Log_timestamp,
	})

	if c.General.Log_level == 3 {
		log.SetLevel(logrus.DebugLevel)
	} else if c.General.Log_level == 2 {
		log.SetLevel(logrus.InfoLevel)
	} else if c.General.Log_level == 1 {
		log.SetLevel(logrus.WarnLevel)
	} else {
		log.SetLevel(logrus.ErrorLevel)
	}
	log.Debug("Config Object: \n", c)
}

func Init_config() {
	log.Infof("--- Config Version %s", c.General.Version)
	log.Infof("--- Config UserAgent: %s", GetUserAgent())
}

func GetScraperConfig() ScraperConfig {
	return c.ScraperConfig
}

func GetDBEnv() Database {
	return c.Database
}

func GetGeneralConfig() General {
	return c.General
}

func GetUserAgent() string {
	return c.ScraperConfig.UserAgent
}

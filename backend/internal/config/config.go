package config

import (
	"fmt"
	"github.com/spf13/viper"
)

type Config struct {
	Server        ServerConfig        `mapstructure:"server"`
	Database      DatabaseConfig      `mapstructure:"database"`
	MQTT          MQTTConfig          `mapstructure:"mqtt"`
	WearCalc      WearCalcConfig      `mapstructure:"wear_calculation"`
	LifePred      LifePredConfig      `mapstructure:"life_prediction"`
	Alert         AlertConfig         `mapstructure:"alert"`
}

type ServerConfig struct {
	Port        int      `mapstructure:"port"`
	ModbusPort  int      `mapstructure:"modbus_port"`
	CORSOrigins []string `mapstructure:"cors_origins"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

type MQTTConfig struct {
	Broker      string `mapstructure:"broker"`
	ClientID    string `mapstructure:"client_id"`
	Username    string `mapstructure:"username"`
	Password    string `mapstructure:"password"`
	TopicPrefix string `mapstructure:"topic_prefix"`
}

type WearCalcConfig struct {
	IntervalMinutes  int     `mapstructure:"interval_minutes"`
	ArchardK         float64 `mapstructure:"archard_k"`
	EHLReferenceTemp float64 `mapstructure:"ehl_reference_temp"`
}

type LifePredConfig struct {
	IntervalMinutes       int     `mapstructure:"interval_minutes"`
	WeibullDefaultShape   float64 `mapstructure:"weibull_default_shape"`
	WeibullDefaultScale   float64 `mapstructure:"weibull_default_scale"`
	MinSamplesForFit      int     `mapstructure:"min_samples_for_fit"`
}

type AlertConfig struct {
	WearWarningRatio   float64 `mapstructure:"wear_warning_ratio"`
	WearCriticalRatio  float64 `mapstructure:"wear_critical_ratio"`
	OilFilmMinimum     float64 `mapstructure:"oil_film_minimum"`
	CooldownMinutes    int     `mapstructure:"cooldown_minutes"`
}

var AppConfig *Config

func Load(path string) error {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return fmt.Errorf("解析配置失败: %w", err)
	}

	AppConfig = cfg
	return nil
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
	)
}

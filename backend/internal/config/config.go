package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	Server        ServerConfig        `mapstructure:"server"`
	Database      DatabaseConfig      `mapstructure:"database"`
	MQTT          MQTTConfig          `mapstructure:"mqtt"`
	WearCalc      WearCalcConfig      `mapstructure:"wear_calculation"`
	LifePred      LifePredConfig      `mapstructure:"life_prediction"`
	Alert         AlertConfig         `mapstructure:"alert"`
	WearParams    WearParamsConfig
	Lubrication   LubricationConfig
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

type WearParamsConfig struct {
	ArchardKBase                 float64           `json:"archard_k_base"`
	EHLReferenceTempCelsius      float64           `json:"ehl_reference_temp_celsius"`
	PressureViscosityCoefficient float64           `json:"pressure_viscosity_coefficient"`
	ReducedElasticModulusPa      float64           `json:"reduced_elastic_modulus_pa"`
	SurfaceRoughnessRMSMeters    float64           `json:"surface_roughness_rms_meters"`
	TempCorrectionFactorPerDegree float64          `json:"temp_correction_factor_per_degree"`
	FullFilmLambdaThreshold      float64           `json:"full_film_lambda_threshold"`
	MixedLubricationLambdaThreshold float64        `json:"mixed_lubrication_lambda_threshold"`
	WearCoefficientFactors       map[string]float64 `json:"wear_coefficient_factors"`
	HardnessConversionFactor     float64           `json:"hardness_conversion_factor"`
	OilFilmGrid                  OilFilmGridConfig `json:"oil_film_grid"`
}

type OilFilmGridConfig struct {
	SizeX int `json:"size_x"`
	SizeY int `json:"size_y"`
}

type LubricationConfig struct {
	MixedLubrication      MixedLubricationConfig      `json:"mixed_lubrication"`
	EHLCorrection         EHLCorrectionConfig         `json:"ehl_correction"`
	OilFilmRuptureThresholdMicrom float64             `json:"oil_film_rupture_threshold_microm"`
	ViscosityIndex        int                           `json:"viscosity_index"`
	PressureCoefficientAlpha float64                    `json:"pressure_coefficient_alpha"`
}

type MixedLubricationConfig struct {
	SommerfeldThresholdLow         float64 `json:"sommerfeld_threshold_low"`
	SommerfeldThresholdHigh        float64 `json:"sommerfeld_threshold_high"`
	CorrectionFactorLowSommerfeld  float64 `json:"correction_factor_low_sommerfeld"`
	LoadSeverityExponent           float64 `json:"load_severity_exponent"`
	LoadSeverityReferenceN         float64 `json:"load_severity_reference_n"`
	AsperityContactExponent        float64 `json:"asperity_contact_exponent"`
	AsperityEffectFactor           float64 `json:"asperity_effect_factor"`
	BoundaryMaxContactRatio        float64 `json:"boundary_max_contact_ratio"`
}

type EHLCorrectionConfig struct {
	DowsonHigginsonCoefficient     float64 `json:"dowson_higginson_coefficient"`
	AlternativeFormulaCoefficient  float64 `json:"alternative_formula_coefficient"`
	AlternativeFormulaThresholdU   float64 `json:"alternative_formula_threshold_u"`
	TempViscosityExponent          float64 `json:"temp_viscosity_exponent"`
	LambdaMinClamp                 float64 `json:"lambda_min_clamp"`
	LambdaMaxClamp                 float64 `json:"lambda_max_clamp"`
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

	if err := loadWearParams("config/wear_params.json", &cfg.WearParams); err != nil {
		return fmt.Errorf("加载磨损参数失败: %w", err)
	}

	if err := loadLubricationParams("config/lubrication_params.json", &cfg.Lubrication); err != nil {
		return fmt.Errorf("加载润滑参数失败: %w", err)
	}

	AppConfig = cfg
	return nil
}

func loadWearParams(path string, params *WearParamsConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取磨损参数文件失败: %w", err)
	}
	if err := json.Unmarshal(data, params); err != nil {
		return fmt.Errorf("解析磨损参数失败: %w", err)
	}
	return nil
}

func loadLubricationParams(path string, params *LubricationConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取润滑参数文件失败: %w", err)
	}
	if err := json.Unmarshal(data, params); err != nil {
		return fmt.Errorf("解析润滑参数失败: %w", err)
	}
	return nil
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
	)
}

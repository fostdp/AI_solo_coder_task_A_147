package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"noria-bearing-system/internal/config"
	"noria-bearing-system/internal/models"
)

type DB struct {
	pool *pgxpool.Pool
}

var Instance *DB

func Connect() error {
	cfg, err := pgxpool.ParseConfig(config.AppConfig.Database.DSN())
	if err != nil {
		return fmt.Errorf("解析数据库配置失败: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("创建数据库连接池失败: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("数据库连接失败: %w", err)
	}

	Instance = &DB{pool: pool}
	return nil
}

func (db *DB) Close() {
	db.pool.Close()
}

func (db *DB) Ping(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

func (db *DB) GetPool() *pgxpool.Pool {
	return db.pool
}

func (db *DB) InsertSensorData(ctx context.Context, data *models.SensorData) error {
	query := `
		INSERT INTO sensor_data (time, bearing_id, temperature, radial_load, rotational_speed, oil_film_thickness, ambient_temp, humidity, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := db.pool.Exec(ctx, query,
		data.Time, data.BearingID, data.Temperature, data.RadialLoad,
		data.RotationalSpeed, data.OilFilmThickness, data.AmbientTemp, data.Humidity, data.Source,
	)
	return err
}

func (db *DB) GetAllBearings(ctx context.Context) ([]models.Bearing, error) {
	query := `SELECT id, noria_wheel_id, bearing_code, position, bearing_type, inner_diameter, outer_diameter, width, material, hardness_hv, rated_life_hours, wear_limit_microm, initial_wear_microm, lubrication_type, oil_viscosity_pas, installed_at, status, created_at FROM bearings WHERE status = 'active'`
	rows, err := db.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bearings []models.Bearing
	for rows.Next() {
		var b models.Bearing
		err := rows.Scan(&b.ID, &b.NoriaWheelID, &b.BearingCode, &b.Position, &b.BearingType,
			&b.InnerDiameter, &b.OuterDiameter, &b.Width, &b.Material, &b.HardnessHV,
			&b.RatedLifeHours, &b.WearLimitMicrom, &b.InitialWearMicrom, &b.LubricationType,
			&b.OilViscosityPaS, &b.InstalledAt, &b.Status, &b.CreatedAt)
		if err != nil {
			return nil, err
		}
		bearings = append(bearings, b)
	}
	return bearings, rows.Err()
}

func (db *DB) GetSensorDataByTimeRange(ctx context.Context, bearingID int, start, end time.Time) ([]models.SensorData, error) {
	query := `
		SELECT time, bearing_id, temperature, radial_load, rotational_speed, oil_film_thickness, ambient_temp, humidity, source
		FROM sensor_data
		WHERE bearing_id = $1 AND time >= $2 AND time <= $3
		ORDER BY time ASC
	`
	rows, err := db.pool.Query(ctx, query, bearingID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var data []models.SensorData
	for rows.Next() {
		var d models.SensorData
		err := rows.Scan(&d.Time, &d.BearingID, &d.Temperature, &d.RadialLoad,
			&d.RotationalSpeed, &d.OilFilmThickness, &d.AmbientTemp, &d.Humidity, &d.Source)
		if err != nil {
			return nil, err
		}
		data = append(data, d)
	}
	return data, rows.Err()
}

func (db *DB) GetLatestSensorData(ctx context.Context, bearingID int) (*models.SensorData, error) {
	query := `
		SELECT time, bearing_id, temperature, radial_load, rotational_speed, oil_film_thickness, ambient_temp, humidity, source
		FROM sensor_data
		WHERE bearing_id = $1
		ORDER BY time DESC
		LIMIT 1
	`
	row := db.pool.QueryRow(ctx, query, bearingID)
	var d models.SensorData
	err := row.Scan(&d.Time, &d.BearingID, &d.Temperature, &d.RadialLoad,
		&d.RotationalSpeed, &d.OilFilmThickness, &d.AmbientTemp, &d.Humidity, &d.Source)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (db *DB) InsertWearResult(ctx context.Context, wr *models.WearResult) error {
	query := `
		INSERT INTO wear_results (bearing_id, calculated_at, period_start, period_end, wear_depth_microm, wear_rate_microm_per_hour, total_wear_microm, archard_wear_volume, ehl_film_parameter, sliding_distance, wear_coefficient, contact_pressure, calculation_note)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := db.pool.Exec(ctx, query,
		wr.BearingID, wr.CalculatedAt, wr.PeriodStart, wr.PeriodEnd,
		wr.WearDepthMicrom, wr.WearRateMicromPerHour, wr.TotalWearMicrom,
		wr.ArchardWearVolume, wr.EHLFilmParameter, wr.SlidingDistance,
		wr.WearCoefficient, wr.ContactPressure, wr.CalculationNote,
	)
	return err
}

func (db *DB) GetLatestWearResult(ctx context.Context, bearingID int) (*models.WearResult, error) {
	query := `
		SELECT id, bearing_id, calculated_at, period_start, period_end, wear_depth_microm, wear_rate_microm_per_hour, total_wear_microm, archard_wear_volume, ehl_film_parameter, sliding_distance, wear_coefficient, contact_pressure, calculation_note
		FROM wear_results
		WHERE bearing_id = $1
		ORDER BY calculated_at DESC
		LIMIT 1
	`
	row := db.pool.QueryRow(ctx, query, bearingID)
	var wr models.WearResult
	err := row.Scan(&wr.ID, &wr.BearingID, &wr.CalculatedAt, &wr.PeriodStart, &wr.PeriodEnd,
		&wr.WearDepthMicrom, &wr.WearRateMicromPerHour, &wr.TotalWearMicrom,
		&wr.ArchardWearVolume, &wr.EHLFilmParameter, &wr.SlidingDistance,
		&wr.WearCoefficient, &wr.ContactPressure, &wr.CalculationNote)
	if err != nil {
		return nil, err
	}
	return &wr, nil
}

func (db *DB) GetWearHistory(ctx context.Context, bearingID int, limit int) ([]models.WearResult, error) {
	query := `
		SELECT id, bearing_id, calculated_at, period_start, period_end, wear_depth_microm, wear_rate_microm_per_hour, total_wear_microm, archard_wear_volume, ehl_film_parameter, sliding_distance, wear_coefficient, contact_pressure, calculation_note
		FROM wear_results
		WHERE bearing_id = $1
		ORDER BY calculated_at DESC
		LIMIT $2
	`
	rows, err := db.pool.Query(ctx, query, bearingID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.WearResult
	for rows.Next() {
		var wr models.WearResult
		err := rows.Scan(&wr.ID, &wr.BearingID, &wr.CalculatedAt, &wr.PeriodStart, &wr.PeriodEnd,
			&wr.WearDepthMicrom, &wr.WearRateMicromPerHour, &wr.TotalWearMicrom,
			&wr.ArchardWearVolume, &wr.EHLFilmParameter, &wr.SlidingDistance,
			&wr.WearCoefficient, &wr.ContactPressure, &wr.CalculationNote)
		if err != nil {
			return nil, err
		}
		results = append(results, wr)
	}
	return results, rows.Err()
}

func (db *DB) InsertLifePrediction(ctx context.Context, lp *models.LifePrediction) error {
	query := `
		INSERT INTO life_predictions (bearing_id, predicted_at, weibull_shape, weibull_scale, running_hours, predicted_rul_hours, reliability, failure_probability, confidence_interval_low, confidence_interval_high, wear_rate_trend, fatigue_damage, prediction_method)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := db.pool.Exec(ctx, query,
		lp.BearingID, lp.PredictedAt, lp.WeibullShape, lp.WeibullScale, lp.RunningHours,
		lp.PredictedRULHours, lp.Reliability, lp.FailureProbability,
		lp.ConfidenceIntervalLow, lp.ConfidenceIntervalHigh, lp.WearRateTrend,
		lp.FatigueDamage, lp.PredictionMethod,
	)
	return err
}

func (db *DB) GetLatestLifePrediction(ctx context.Context, bearingID int) (*models.LifePrediction, error) {
	query := `
		SELECT id, bearing_id, predicted_at, weibull_shape, weibull_scale, running_hours, predicted_rul_hours, reliability, failure_probability, confidence_interval_low, confidence_interval_high, wear_rate_trend, fatigue_damage, prediction_method
		FROM life_predictions
		WHERE bearing_id = $1
		ORDER BY predicted_at DESC
		LIMIT 1
	`
	row := db.pool.QueryRow(ctx, query, bearingID)
	var lp models.LifePrediction
	err := row.Scan(&lp.ID, &lp.BearingID, &lp.PredictedAt, &lp.WeibullShape, &lp.WeibullScale,
		&lp.RunningHours, &lp.PredictedRULHours, &lp.Reliability, &lp.FailureProbability,
		&lp.ConfidenceIntervalLow, &lp.ConfidenceIntervalHigh, &lp.WearRateTrend,
		&lp.FatigueDamage, &lp.PredictionMethod)
	if err != nil {
		return nil, err
	}
	return &lp, nil
}

func (db *DB) InsertAlertEvent(ctx context.Context, alert *models.AlertEvent) error {
	query := `
		INSERT INTO alert_events (bearing_id, alert_time, alert_type, alert_level, alert_message, threshold_value, actual_value, mqtt_topic)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := db.pool.Exec(ctx, query,
		alert.BearingID, alert.AlertTime, alert.AlertType, alert.AlertLevel,
		alert.AlertMessage, alert.ThresholdValue, alert.ActualValue, alert.MQTTTopic,
	)
	return err
}

func (db *DB) GetLatestAlertTime(ctx context.Context, bearingID int, alertType string) (*time.Time, error) {
	query := `
		SELECT MAX(alert_time) FROM alert_events
		WHERE bearing_id = $1 AND alert_type = $2
	`
	var t *time.Time
	err := db.pool.QueryRow(ctx, query, bearingID, alertType).Scan(&t)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (db *DB) GetAllNoriaWheels(ctx context.Context) ([]models.NoriaWheel, error) {
	query := `SELECT id, name, location, diameter, buckets, installation_date, description, created_at, updated_at FROM noria_wheels`
	rows, err := db.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var wheels []models.NoriaWheel
	for rows.Next() {
		var w models.NoriaWheel
		err := rows.Scan(&w.ID, &w.Name, &w.Location, &w.Diameter, &w.Buckets,
			&w.InstallationDate, &w.Description, &w.CreatedAt, &w.UpdatedAt)
		if err != nil {
			return nil, err
		}
		wheels = append(wheels, w)
	}
	return wheels, rows.Err()
}

func (db *DB) GetBearingsByNoriaWheel(ctx context.Context, noriaID int) ([]models.Bearing, error) {
	query := `SELECT id, noria_wheel_id, bearing_code, position, bearing_type, inner_diameter, outer_diameter, width, material, hardness_hv, rated_life_hours, wear_limit_microm, initial_wear_microm, lubrication_type, oil_viscosity_pas, installed_at, status, created_at FROM bearings WHERE noria_wheel_id = $1`
	rows, err := db.pool.Query(ctx, query, noriaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bearings []models.Bearing
	for rows.Next() {
		var b models.Bearing
		err := rows.Scan(&b.ID, &b.NoriaWheelID, &b.BearingCode, &b.Position, &b.BearingType,
			&b.InnerDiameter, &b.OuterDiameter, &b.Width, &b.Material, &b.HardnessHV,
			&b.RatedLifeHours, &b.WearLimitMicrom, &b.InitialWearMicrom, &b.LubricationType,
			&b.OilViscosityPaS, &b.InstalledAt, &b.Status, &b.CreatedAt)
		if err != nil {
			return nil, err
		}
		bearings = append(bearings, b)
	}
	return bearings, rows.Err()
}

func (db *DB) GetBearingLatestStatus(ctx context.Context) ([]models.BearingLatestStatus, error) {
	query := `
		SELECT bearing_id, bearing_code, position, bearing_type, noria_wheel_id, noria_name,
			last_data_time, temperature, radial_load, rotational_speed, oil_film_thickness,
			total_wear_microm, wear_rate_microm_per_hour, predicted_rul_hours, reliability, health_status
		FROM v_bearing_latest_status
	`
	rows, err := db.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var statuses []models.BearingLatestStatus
	for rows.Next() {
		var s models.BearingLatestStatus
		err := rows.Scan(&s.BearingID, &s.BearingCode, &s.Position, &s.BearingType,
			&s.NoriaWheelID, &s.NoriaName, &s.LastDataTime, &s.Temperature,
			&s.RadialLoad, &s.RotationalSpeed, &s.OilFilmThickness,
			&s.TotalWearMicrom, &s.WearRateMicromPerHour,
			&s.PredictedRULHours, &s.Reliability, &s.HealthStatus)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, s)
	}
	return statuses, rows.Err()
}

func (db *DB) GetRecentAlerts(ctx context.Context, limit int) ([]models.AlertEvent, error) {
	query := `
		SELECT id, bearing_id, alert_time, alert_type, alert_level, alert_message,
			threshold_value, actual_value, mqtt_topic, acknowledged, acknowledged_at, resolved, resolved_at
		FROM alert_events
		ORDER BY alert_time DESC
		LIMIT $1
	`
	rows, err := db.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []models.AlertEvent
	for rows.Next() {
		var a models.AlertEvent
		err := rows.Scan(&a.ID, &a.BearingID, &a.AlertTime, &a.AlertType, &a.AlertLevel,
			&a.AlertMessage, &a.ThresholdValue, &a.ActualValue, &a.MQTTTopic,
			&a.Acknowledged, &a.AcknowledgedAt, &a.Resolved, &a.ResolvedAt)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

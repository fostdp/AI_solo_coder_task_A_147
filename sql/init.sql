-- ============================================================
-- 古代水转筒车轴承磨损仿真与寿命预测系统
-- TimescaleDB 初始化脚本
-- ============================================================

-- 创建数据库（如果不存在的话需要手动执行）
-- CREATE DATABASE noria_bearing WITH ENCODING 'UTF8';

-- 连接到数据库后执行以下脚本

-- 启用 TimescaleDB 扩展
CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

-- ============================================================
-- 1. 筒车基础信息表
-- ============================================================
CREATE TABLE IF NOT EXISTS noria_wheels (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    location VARCHAR(200),
    diameter NUMERIC(10, 2) NOT NULL,
    buckets INTEGER NOT NULL,
    installation_date DATE,
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================================
-- 2. 轴承信息表
-- ============================================================
CREATE TABLE IF NOT EXISTS bearings (
    id SERIAL PRIMARY KEY,
    noria_wheel_id INTEGER NOT NULL REFERENCES noria_wheels(id) ON DELETE CASCADE,
    bearing_code VARCHAR(50) NOT NULL UNIQUE,
    position VARCHAR(50) NOT NULL,
    bearing_type VARCHAR(50) NOT NULL,
    inner_diameter NUMERIC(10, 4) NOT NULL,
    outer_diameter NUMERIC(10, 4) NOT NULL,
    width NUMERIC(10, 4) NOT NULL,
    material VARCHAR(100) NOT NULL,
    hardness_hv NUMERIC(10, 2),
    rated_life_hours NUMERIC(15, 2) DEFAULT 50000,
    wear_limit_microm NUMERIC(10, 4) DEFAULT 100,
    initial_wear_microm NUMERIC(10, 4) DEFAULT 0,
    lubrication_type VARCHAR(50) DEFAULT 'grease',
    oil_viscosity_pas NUMERIC(10, 6) DEFAULT 0.05,
    installed_at TIMESTAMPTZ DEFAULT NOW(),
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_bearings_noria_wheel ON bearings(noria_wheel_id);
CREATE INDEX IF NOT EXISTS idx_bearings_status ON bearings(status);

-- ============================================================
-- 3. 传感器实时数据表（超表）
-- ============================================================
CREATE TABLE IF NOT EXISTS sensor_data (
    time TIMESTAMPTZ NOT NULL,
    bearing_id INTEGER NOT NULL REFERENCES bearings(id) ON DELETE CASCADE,
    temperature NUMERIC(10, 4) NOT NULL,
    radial_load NUMERIC(15, 4) NOT NULL,
    rotational_speed NUMERIC(10, 4) NOT NULL,
    oil_film_thickness NUMERIC(10, 6) NOT NULL,
    ambient_temp NUMERIC(10, 4),
    humidity NUMERIC(10, 4),
    source VARCHAR(20) DEFAULT 'modbus'
);

-- 转换为 TimescaleDB 超表
SELECT create_hypertable('sensor_data', 'time', if_not_exists => TRUE);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_sensor_data_bearing_time ON sensor_data(bearing_id, time DESC);

-- ============================================================
-- 4. 磨损计算结果表
-- ============================================================
CREATE TABLE IF NOT EXISTS wear_results (
    id BIGSERIAL PRIMARY KEY,
    bearing_id INTEGER NOT NULL REFERENCES bearings(id) ON DELETE CASCADE,
    calculated_at TIMESTAMPTZ NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    wear_depth_microm NUMERIC(12, 6) NOT NULL,
    wear_rate_microm_per_hour NUMERIC(12, 6),
    total_wear_microm NUMERIC(12, 6) NOT NULL,
    archard_wear_volume NUMERIC(15, 6),
    ehl_film_parameter NUMERIC(10, 4),
    sliding_distance NUMERIC(15, 4),
    wear_coefficient NUMERIC(15, 10),
    contact_pressure NUMERIC(15, 4),
    calculation_note TEXT
);

SELECT create_hypertable('wear_results', 'calculated_at', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_wear_results_bearing ON wear_results(bearing_id, calculated_at DESC);

-- ============================================================
-- 5. 寿命预测结果表
-- ============================================================
CREATE TABLE IF NOT EXISTS life_predictions (
    id BIGSERIAL PRIMARY KEY,
    bearing_id INTEGER NOT NULL REFERENCES bearings(id) ON DELETE CASCADE,
    predicted_at TIMESTAMPTZ NOT NULL,
    weibull_shape NUMERIC(10, 6) NOT NULL,
    weibull_scale NUMERIC(15, 4) NOT NULL,
    running_hours NUMERIC(15, 4) NOT NULL,
    predicted_rul_hours NUMERIC(15, 4) NOT NULL,
    reliability NUMERIC(10, 6),
    failure_probability NUMERIC(10, 6),
    confidence_interval_low NUMERIC(15, 4),
    confidence_interval_high NUMERIC(15, 4),
    wear_rate_trend NUMERIC(12, 6),
    fatigue_damage NUMERIC(10, 6),
    prediction_method VARCHAR(50) DEFAULT 'weibull_mixed'
);

SELECT create_hypertable('life_predictions', 'predicted_at', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_life_predictions_bearing ON life_predictions(bearing_id, predicted_at DESC);

-- ============================================================
-- 6. 告警事件表
-- ============================================================
CREATE TABLE IF NOT EXISTS alert_events (
    id BIGSERIAL PRIMARY KEY,
    bearing_id INTEGER NOT NULL REFERENCES bearings(id) ON DELETE CASCADE,
    alert_time TIMESTAMPTZ NOT NULL,
    alert_type VARCHAR(30) NOT NULL,
    alert_level VARCHAR(20) NOT NULL,
    alert_message TEXT NOT NULL,
    threshold_value NUMERIC(15, 6),
    actual_value NUMERIC(15, 6),
    mqtt_topic VARCHAR(200),
    acknowledged BOOLEAN DEFAULT FALSE,
    acknowledged_at TIMESTAMPTZ,
    resolved BOOLEAN DEFAULT FALSE,
    resolved_at TIMESTAMPTZ
);

SELECT create_hypertable('alert_events', 'alert_time', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_alerts_bearing ON alert_events(bearing_id, alert_time DESC);
CREATE INDEX IF NOT EXISTS idx_alerts_level ON alert_events(alert_level, alert_time DESC);
CREATE INDEX IF NOT EXISTS idx_alerts_unresolved ON alert_events(resolved, alert_time DESC);

-- ============================================================
-- 7. 油膜厚度网格数据（用于颜色云图）
-- ============================================================
CREATE TABLE IF NOT EXISTS oil_film_maps (
    id BIGSERIAL PRIMARY KEY,
    bearing_id INTEGER NOT NULL REFERENCES bearings(id) ON DELETE CASCADE,
    captured_at TIMESTAMPTZ NOT NULL,
    grid_size_x INTEGER NOT NULL,
    grid_size_y INTEGER NOT NULL,
    film_data JSONB NOT NULL
);

SELECT create_hypertable('oil_film_maps', 'captured_at', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_oil_film_maps_bearing ON oil_film_maps(bearing_id, captured_at DESC);

-- ============================================================
-- 视图：轴承最新状态汇总
-- ============================================================
CREATE OR REPLACE VIEW v_bearing_latest_status AS
SELECT DISTINCT ON (b.id)
    b.id AS bearing_id,
    b.bearing_code,
    b.position,
    b.bearing_type,
    nw.id AS noria_wheel_id,
    nw.name AS noria_name,
    sd.time AS last_data_time,
    sd.temperature,
    sd.radial_load,
    sd.rotational_speed,
    sd.oil_film_thickness,
    wr.total_wear_microm,
    wr.wear_rate_microm_per_hour,
    lp.predicted_rul_hours,
    lp.reliability,
    CASE
        WHEN wr.total_wear_microm >= b.wear_limit_microm THEN 'critical'
        WHEN wr.total_wear_microm >= b.wear_limit_microm * 0.8 THEN 'warning'
        WHEN sd.oil_film_thickness < 0.5 THEN 'warning'
        ELSE 'normal'
    END AS health_status
FROM bearings b
INNER JOIN noria_wheels nw ON nw.id = b.noria_wheel_id
LEFT JOIN sensor_data sd ON sd.bearing_id = b.id
LEFT JOIN wear_results wr ON wr.bearing_id = b.id
LEFT JOIN life_predictions lp ON lp.bearing_id = b.id
ORDER BY b.id, sd.time DESC, wr.calculated_at DESC, lp.predicted_at DESC;

-- ============================================================
-- 初始化示例数据
-- ============================================================
-- 插入唐代水转筒车
INSERT INTO noria_wheels (name, location, diameter, buckets, installation_date, description) VALUES
('唐代高转筒车一号', '陕西省西安曲江池遗址', 8.5, 36, '2024-03-15', '复原唐代形制，木质结构，外径8.5米'),
('唐代水转筒车二号', '河南省洛阳隋唐大运河博物馆', 6.2, 24, '2024-05-20', '中型复原筒车，用于灌溉演示')
ON CONFLICT DO NOTHING;

-- 插入轴承信息
INSERT INTO bearings (
    noria_wheel_id, bearing_code, position, bearing_type,
    inner_diameter, outer_diameter, width, material, hardness_hv,
    rated_life_hours, wear_limit_microm, initial_wear_microm,
    lubrication_type, oil_viscosity_pas, status
) VALUES
(1, 'NRW-001-BR-A', '主轴上轴承', '滑动轴承-木质', 80.0, 120.0, 100.0, '青冈木包铜', 180.0, 40000, 150.0, 0.0, 'animal_fat', 0.08, 'active'),
(1, 'NRW-001-BR-B', '主轴下轴承', '滑动轴承-木质', 80.0, 120.0, 100.0, '青冈木包铜', 180.0, 40000, 150.0, 0.0, 'animal_fat', 0.08, 'active'),
(2, 'NRW-002-BR-A', '主轴轴承', '滑动轴承-石质', 60.0, 100.0, 80.0, '砂岩', 220.0, 60000, 200.0, 0.0, 'vegetable_oil', 0.06, 'active')
ON CONFLICT (bearing_code) DO NOTHING;

-- ============================================================
-- 连续聚合：每小时平均数据
-- ============================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS sensor_data_hourly
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', time) AS bucket,
    bearing_id,
    AVG(temperature) AS avg_temperature,
    MAX(temperature) AS max_temperature,
    MIN(temperature) AS min_temperature,
    AVG(radial_load) AS avg_radial_load,
    MAX(radial_load) AS max_radial_load,
    AVG(rotational_speed) AS avg_rotational_speed,
    AVG(oil_film_thickness) AS avg_oil_film_thickness,
    MIN(oil_film_thickness) AS min_oil_film_thickness,
    COUNT(*) AS sample_count
FROM sensor_data
GROUP BY bucket, bearing_id
WITH NO DATA;

SELECT add_continuous_aggregate_policy('sensor_data_hourly',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour',
    if_not_exists => TRUE
);

-- ============================================================
-- 数据保留策略：原始数据保留1年，聚合数据保留5年
-- ============================================================
SELECT add_retention_policy('sensor_data', INTERVAL '1 year', if_not_exists => TRUE);
SELECT add_retention_policy('sensor_data_hourly', INTERVAL '5 years', if_not_exists => TRUE);

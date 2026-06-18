-- ============================================================
-- TimescaleDB 高级策略：降采样连续聚合 + 数据保留
-- 在 init.sql 之后执行
-- ============================================================

-- ============================================================
-- 1. 原始传感器数据：按 5分钟、15分钟、1小时、1天 多层聚合
-- ============================================================

-- 5分钟粒度聚合
CREATE MATERIALIZED VIEW IF NOT EXISTS sensor_data_5min
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('5 minutes', time) AS bucket,
    bearing_id,
    AVG(temperature)::NUMERIC(10,4) AS avg_temperature,
    MAX(temperature)::NUMERIC(10,4) AS max_temperature,
    MIN(temperature)::NUMERIC(10,4) AS min_temperature,
    STDDEV(temperature)::NUMERIC(10,4) AS stddev_temperature,
    AVG(radial_load)::NUMERIC(15,4) AS avg_radial_load,
    MAX(radial_load)::NUMERIC(15,4) AS max_radial_load,
    MIN(radial_load)::NUMERIC(15,4) AS min_radial_load,
    AVG(rotational_speed)::NUMERIC(10,4) AS avg_rotational_speed,
    MAX(rotational_speed)::NUMERIC(10,4) AS max_rotational_speed,
    MIN(rotational_speed)::NUMERIC(10,4) AS min_rotational_speed,
    AVG(oil_film_thickness)::NUMERIC(10,6) AS avg_oil_film_thickness,
    MIN(oil_film_thickness)::NUMERIC(10,6) AS min_oil_film_thickness,
    COUNT(*) AS sample_count
FROM sensor_data
GROUP BY bucket, bearing_id
WITH NO DATA;

SELECT add_continuous_aggregate_policy('sensor_data_5min',
    start_offset => INTERVAL '15 minutes',
    end_offset => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes',
    if_not_exists => TRUE
);

-- 15分钟粒度聚合
CREATE MATERIALIZED VIEW IF NOT EXISTS sensor_data_15min
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('15 minutes', time) AS bucket,
    bearing_id,
    AVG(temperature)::NUMERIC(10,4) AS avg_temperature,
    MAX(temperature)::NUMERIC(10,4) AS max_temperature,
    MIN(temperature)::NUMERIC(10,4) AS min_temperature,
    STDDEV(temperature)::NUMERIC(10,4) AS stddev_temperature,
    AVG(radial_load)::NUMERIC(15,4) AS avg_radial_load,
    MAX(radial_load)::NUMERIC(15,4) AS max_radial_load,
    MIN(radial_load)::NUMERIC(15,4) AS min_radial_load,
    AVG(rotational_speed)::NUMERIC(10,4) AS avg_rotational_speed,
    MAX(rotational_speed)::NUMERIC(10,4) AS max_rotational_speed,
    MIN(rotational_speed)::NUMERIC(10,4) AS min_rotational_speed,
    AVG(oil_film_thickness)::NUMERIC(10,6) AS avg_oil_film_thickness,
    MIN(oil_film_thickness)::NUMERIC(10,6) AS min_oil_film_thickness,
    COUNT(*) AS sample_count
FROM sensor_data
GROUP BY bucket, bearing_id
WITH NO DATA;

SELECT add_continuous_aggregate_policy('sensor_data_15min',
    start_offset => INTERVAL '45 minutes',
    end_offset => INTERVAL '15 minutes',
    schedule_interval => INTERVAL '15 minutes',
    if_not_exists => TRUE
);

-- 1小时粒度聚合（已在init.sql中创建，此处补充刷新策略）
SELECT add_continuous_aggregate_policy('sensor_data_hourly',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour',
    if_not_exists => TRUE
);

-- 1天粒度聚合
CREATE MATERIALIZED VIEW IF NOT EXISTS sensor_data_daily
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', time) AS bucket,
    bearing_id,
    AVG(temperature)::NUMERIC(10,4) AS avg_temperature,
    MAX(temperature)::NUMERIC(10,4) AS max_temperature,
    MIN(temperature)::NUMERIC(10,4) AS min_temperature,
    STDDEV(temperature)::NUMERIC(10,4) AS stddev_temperature,
    AVG(radial_load)::NUMERIC(15,4) AS avg_radial_load,
    MAX(radial_load)::NUMERIC(15,4) AS max_radial_load,
    MIN(radial_load)::NUMERIC(15,4) AS min_radial_load,
    AVG(rotational_speed)::NUMERIC(10,4) AS avg_rotational_speed,
    MAX(rotational_speed)::NUMERIC(10,4) AS max_rotational_speed,
    MIN(rotational_speed)::NUMERIC(10,4) AS min_rotational_speed,
    AVG(oil_film_thickness)::NUMERIC(10,6) AS avg_oil_film_thickness,
    MIN(oil_film_thickness)::NUMERIC(10,6) AS min_oil_film_thickness,
    COUNT(*) AS sample_count
FROM sensor_data
GROUP BY bucket, bearing_id
WITH NO DATA;

SELECT add_continuous_aggregate_policy('sensor_data_daily',
    start_offset => INTERVAL '3 days',
    end_offset => INTERVAL '1 day',
    schedule_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- ============================================================
-- 2. 磨损结果聚合
-- ============================================================

CREATE MATERIALIZED VIEW IF NOT EXISTS wear_results_daily
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', calculated_at) AS bucket,
    bearing_id,
    SUM(wear_depth_microm)::NUMERIC(12,6) AS daily_wear_microm,
    AVG(wear_rate_microm_per_hour)::NUMERIC(12,6) AS avg_wear_rate,
    MAX(wear_rate_microm_per_hour)::NUMERIC(12,6) AS max_wear_rate,
    MAX(total_wear_microm)::NUMERIC(12,6) AS total_wear_microm,
    AVG(ehl_film_parameter)::NUMERIC(10,4) AS avg_ehl_lambda,
    MIN(ehl_film_parameter)::NUMERIC(10,4) AS min_ehl_lambda,
    COUNT(*) AS calc_count
FROM wear_results
GROUP BY bucket, bearing_id
WITH NO DATA;

SELECT add_continuous_aggregate_policy('wear_results_daily',
    start_offset => INTERVAL '3 days',
    end_offset => INTERVAL '1 day',
    schedule_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- ============================================================
-- 3. 告警事件统计
-- ============================================================

CREATE MATERIALIZED VIEW IF NOT EXISTS alert_events_daily
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', alert_time) AS bucket,
    bearing_id,
    alert_level,
    COUNT(*) AS alert_count
FROM alert_events
GROUP BY bucket, bearing_id, alert_level
WITH NO DATA;

SELECT add_continuous_aggregate_policy('alert_events_daily',
    start_offset => INTERVAL '3 days',
    end_offset => INTERVAL '1 day',
    schedule_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- ============================================================
-- 4. 数据保留策略（精细化分层）
-- ============================================================

-- 原始数据：保留 90天（高频原始数据）
SELECT add_retention_policy('sensor_data',
    drop_after => INTERVAL '90 days',
    if_not_exists => TRUE
);

-- 5分钟聚合：保留 30天
SELECT add_retention_policy('sensor_data_5min',
    drop_after => INTERVAL '30 days',
    if_not_exists => TRUE
);

-- 15分钟聚合：保留 90天
SELECT add_retention_policy('sensor_data_15min',
    drop_after => INTERVAL '90 days',
    if_not_exists => TRUE
);

-- 1小时聚合：保留 2年
SELECT add_retention_policy('sensor_data_hourly',
    drop_after => INTERVAL '2 years',
    if_not_exists => TRUE
);

-- 1天聚合：保留 10年（用于长期趋势分析）
SELECT add_retention_policy('sensor_data_daily',
    drop_after => INTERVAL '10 years',
    if_not_exists => TRUE
);

-- 磨损计算结果：保留 5年
SELECT add_retention_policy('wear_results',
    drop_after => INTERVAL '5 years',
    if_not_exists => TRUE
);

-- 磨损日聚合：保留 10年
SELECT add_retention_policy('wear_results_daily',
    drop_after => INTERVAL '10 years',
    if_not_exists => TRUE
);

-- 寿命预测：保留 5年
SELECT add_retention_policy('life_predictions',
    drop_after => INTERVAL '5 years',
    if_not_exists => TRUE
);

-- 告警事件：保留 3年
SELECT add_retention_policy('alert_events',
    drop_after => INTERVAL '3 years',
    if_not_exists => TRUE
);

-- 告警日统计：保留 10年
SELECT add_retention_policy('alert_events_daily',
    drop_after => INTERVAL '10 years',
    if_not_exists => TRUE
);

-- 油膜云图数据：保留 90天（体积较大）
SELECT add_retention_policy('oil_film_maps',
    drop_after => INTERVAL '90 days',
    if_not_exists => TRUE
);

-- ============================================================
-- 5. 压缩策略（TimescaleDB 企业级特性，开源版用 TOAST）
-- ============================================================

-- 对大数据表启用 TOAST 压缩（默认已启用，此处强制）
ALTER TABLE sensor_data SET (
    toast_tuple_target = 8160
);

ALTER TABLE oil_film_maps SET (
    toast_tuple_target = 8160
);

-- ============================================================
-- 6. 信息函数：查询当前保留策略状态
-- ============================================================

CREATE OR REPLACE FUNCTION show_retention_policies()
RETURNS TABLE(
    hypertable_name TEXT,
    drop_after TEXT,
    schedule_interval TEXT,
    next_start TIMESTAMPTZ
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        h.hypertable_name::TEXT,
        j.config->>'drop_after' AS drop_after,
        j.config->>'schedule_interval' AS schedule_interval,
        j.next_start
    FROM timescaledb_information.jobs j
    INNER JOIN timescaledb_information.hypertables h
        ON j.hypertable_id = h.hypertable_id
    WHERE j.proc_name = 'policy_retention'
    ORDER BY h.hypertable_name;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION show_retention_policies() IS '显示所有数据保留策略';

CREATE OR REPLACE FUNCTION show_cagg_policies()
RETURNS TABLE(
    view_name TEXT,
    refresh_interval TEXT,
    next_start TIMESTAMPTZ
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        c.view_name::TEXT,
        j.config->>'schedule_interval' AS refresh_interval,
        j.next_start
    FROM timescaledb_information.jobs j
    INNER JOIN timescaledb_information.continuous_aggregates c
        ON j.hypertable_id = c.materialization_hypertable
    WHERE j.proc_name = 'policy_refresh_continuous_aggregate'
    ORDER BY c.view_name;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION show_cagg_policies() IS '显示所有连续聚合刷新策略';

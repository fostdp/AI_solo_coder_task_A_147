package monitoring

import (
	"net/http"
	_ "net/http/pprof"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	instance *Metrics
	once     sync.Once
)

type Metrics struct {
	modbusReceived       prometheus.Counter
	modbusValidationFail prometheus.Counter
	wearCalculations     prometheus.Counter
	wearCalcErrors       prometheus.Counter
	wearDepthMicrom      prometheus.Gauge
	ehlFilmParameter     prometheus.Gauge
	lifePredictions      prometheus.Counter
	lifePredErrors       prometheus.Counter
	predictedRULHours    prometheus.Gauge
	weibullShape         prometheus.Gauge
	weibullScale         prometheus.Gauge
	reliability          prometheus.Gauge
	alertsSent           prometheus.Counter
	alertsSuppressed     prometheus.Counter
	bearingTemperature   prometheus.Gauge
	bearingRadialLoad    prometheus.Gauge
	bearingRotSpeed      prometheus.Gauge
	bearingOilFilm       prometheus.Gauge
	httpRequests         *prometheus.CounterVec
	httpRequestDuration  prometheus.Histogram
	ChannelDepth         *prometheus.GaugeVec
}

func Get() *Metrics {
	once.Do(func() {
		instance = &Metrics{
			modbusReceived: promauto.NewCounter(prometheus.CounterOpts{
				Name: "noria_modbus_received_total",
				Help: "Modbus接收到的数据包总数",
			}),
			modbusValidationFail: promauto.NewCounter(prometheus.CounterOpts{
				Name: "noria_modbus_validation_fail_total",
				Help: "Modbus数据校验失败次数",
			}),
			wearCalculations: promauto.NewCounter(prometheus.CounterOpts{
				Name: "noria_wear_calculations_total",
				Help: "磨损计算总次数",
			}),
			wearCalcErrors: promauto.NewCounter(prometheus.CounterOpts{
				Name: "noria_wear_calc_errors_total",
				Help: "磨损计算错误次数",
			}),
			wearDepthMicrom: promauto.NewGauge(prometheus.GaugeOpts{
				Name: "noria_wear_depth_microm",
				Help: "当前累计磨损深度 (μm)",
			}),
			ehlFilmParameter: promauto.NewGauge(prometheus.GaugeOpts{
				Name: "noria_ehl_film_parameter_lambda",
				Help: "弹流润滑膜厚参数 Lambda",
			}),
			lifePredictions: promauto.NewCounter(prometheus.CounterOpts{
				Name: "noria_life_predictions_total",
				Help: "寿命预测总次数",
			}),
			lifePredErrors: promauto.NewCounter(prometheus.CounterOpts{
				Name: "noria_life_pred_errors_total",
				Help: "寿命预测错误次数",
			}),
			predictedRULHours: promauto.NewGauge(prometheus.GaugeOpts{
				Name: "noria_predicted_rul_hours",
				Help: "预测剩余寿命 (小时)",
			}),
			weibullShape: promauto.NewGauge(prometheus.GaugeOpts{
				Name: "noria_weibull_shape_parameter",
				Help: "Weibull分布形状参数 Beta",
			}),
			weibullScale: promauto.NewGauge(prometheus.GaugeOpts{
				Name: "noria_weibull_scale_parameter",
				Help: "Weibull分布尺度参数 Eta",
			}),
			reliability: promauto.NewGauge(prometheus.GaugeOpts{
				Name: "noria_bearing_reliability",
				Help: "轴承可靠度 (0-1)",
			}),
			alertsSent: promauto.NewCounter(prometheus.CounterOpts{
				Name: "noria_alerts_sent_total",
				Help: "已发送告警总数",
			}),
			alertsSuppressed: promauto.NewCounter(prometheus.CounterOpts{
				Name: "noria_alerts_suppressed_total",
				Help: "被冷却机制抑制的告警数",
			}),
			bearingTemperature: promauto.NewGauge(prometheus.GaugeOpts{
				Name: "noria_bearing_temperature_celsius",
				Help: "轴承温度 (°C)",
			}),
			bearingRadialLoad: promauto.NewGauge(prometheus.GaugeOpts{
				Name: "noria_bearing_radial_load_newtons",
				Help: "径向载荷 (N)",
			}),
			bearingRotSpeed: promauto.NewGauge(prometheus.GaugeOpts{
				Name: "noria_bearing_rotational_speed_rpm",
				Help: "转速 (RPM)",
			}),
			bearingOilFilm: promauto.NewGauge(prometheus.GaugeOpts{
				Name: "noria_bearing_oil_film_thickness_microm",
				Help: "润滑油膜厚度 (μm)",
			}),
			httpRequests: promauto.NewCounterVec(prometheus.CounterOpts{
				Name: "noria_http_requests_total",
				Help: "HTTP请求总数",
			}, []string{"method", "path", "status"}),
			httpRequestDuration: promauto.NewHistogram(prometheus.HistogramOpts{
				Name:    "noria_http_request_duration_seconds",
				Help:    "HTTP请求耗时分布",
				Buckets: prometheus.DefBuckets,
			}),
			ChannelDepth: promauto.NewGaugeVec(prometheus.GaugeOpts{
				Name: "noria_channel_depth",
				Help: "各通信通道当前消息数",
			}, []string{"channel"}),
		}
	})
	return instance
}

func (m *Metrics) ModbusReceived(valid bool) {
	m.modbusReceived.Inc()
	if !valid {
		m.modbusValidationFail.Inc()
	}
}

func (m *Metrics) WearCalculated(err error, depthMicrom, lambda float64) {
	if err != nil {
		m.wearCalcErrors.Inc()
		return
	}
	m.wearCalculations.Inc()
	m.wearDepthMicrom.Set(depthMicrom)
	m.ehlFilmParameter.Set(lambda)
}

func (m *Metrics) LifePredicted(err error, rulHours, shape, scale, rel float64) {
	if err != nil {
		m.lifePredErrors.Inc()
		return
	}
	m.lifePredictions.Inc()
	m.predictedRULHours.Set(rulHours)
	m.weibullShape.Set(shape)
	m.weibullScale.Set(scale)
	m.reliability.Set(rel)
}

func (m *Metrics) AlertSent(suppressed bool) {
	if suppressed {
		m.alertsSuppressed.Inc()
	} else {
		m.alertsSent.Inc()
	}
}

func (m *Metrics) UpdateSensorData(temp, load, speed, oilFilm float64) {
	m.bearingTemperature.Set(temp)
	m.bearingRadialLoad.Set(load)
	m.bearingRotSpeed.Set(speed)
	m.bearingOilFilm.Set(oilFilm)
}

func (m *Metrics) SetChannelDepth(name string, depth int) {
	m.ChannelDepth.WithLabelValues(name).Set(float64(depth))
}

func (m *Metrics) HTTPRequest(method, path, status string, durationSec float64) {
	m.httpRequests.WithLabelValues(method, path, status).Inc()
	m.httpRequestDuration.Observe(durationSec)
}

func StartPProfServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>pprof</title></head><body>
			<a href="/debug/pprof/">Profiling Endpoints</a></body></html>`))
	})
	mux.Handle("/debug/pprof/", http.DefaultServeMux)

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()
	return srv
}

func StartPrometheusServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("metrics-ok"))
	})

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()
	return srv
}

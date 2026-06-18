package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"noria-bearing-system/internal/api"
	"noria-bearing-system/internal/config"
	"noria-bearing-system/internal/database"
	"noria-bearing-system/internal/models"
	"noria-bearing-system/internal/monitoring"
	"noria-bearing-system/internal/modules/alarm_mqtt"
	"noria-bearing-system/internal/modules/life_predictor"
	"noria-bearing-system/internal/modules/messages"
	"noria-bearing-system/internal/modules/modbus_receiver"
	"noria-bearing-system/internal/modules/wear_simulator"
	"noria-bearing-system/internal/scheduler"
)

var Version = "dev"

func main() {
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	if err := config.Load(configPath); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	log.Printf("水转筒车轴承磨损仿真系统 v%s", Version)
	log.Println("配置文件加载成功 (含JSON参数文件)")
	log.Printf("  - 磨损参数: Archard K=%.2e, EHL参考温度=%.1f°C",
		config.AppConfig.WearParams.ArchardKBase,
		config.AppConfig.WearParams.EHLReferenceTempCelsius)
	log.Printf("  - 润滑参数: Sommerfeld阈值=%.1e~%.1e, 油膜破裂阈值=%.2fμm",
		config.AppConfig.Lubrication.MixedLubrication.SommerfeldThresholdLow,
		config.AppConfig.Lubrication.MixedLubrication.SommerfeldThresholdHigh,
		config.AppConfig.Lubrication.OilFilmRuptureThresholdMicrom)

	if err := database.Connect(); err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	defer database.Instance.Close()
	log.Println("数据库连接成功")

	_ = monitoring.Get()
	pprofSrv := monitoring.StartPProfServer(":6060")
	metricsSrv := monitoring.StartPrometheusServer(":9090")
	log.Println("监控服务已启动 (pprof:6060, metrics:9090)")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if pprofSrv != nil {
			pprofSrv.Shutdown(shutdownCtx)
		}
		if metricsSrv != nil {
			metricsSrv.Shutdown(shutdownCtx)
		}
	}()

	channels := messages.NewModuleChannels(100)
	log.Println("模块通信通道已创建 (缓冲区: 100)")

	alertClient := alarm_mqtt.NewAlertClient(&config.AppConfig.MQTT)
	if err := alertClient.Connect(ctx); err != nil {
		log.Printf("MQTT告警客户端启动失败（将继续运行）: %v", err)
	} else {
		log.Println("MQTT告警客户端已连接")
	}

	alertManager := alarm_mqtt.NewAlertManager(
		channels.AlertChan,
		alertClient,
		config.AppConfig.Alert.CooldownMinutes,
	)
	alertManager.Start(ctx)

	wearSimulator := wear_simulator.NewWearSimulator(
		channels.WearRequestChan,
		channels.WearResultChan,
	)
	wearSimulator.Start(ctx)
	log.Println("磨损仿真器已启动")

	lifePredictor := life_predictor.NewLifePredictor(
		channels.LifeRequestChan,
		channels.LifeResultChan,
	)
	lifePredictor.Start(ctx)
	log.Println("寿命预测器已启动")

	sched := scheduler.NewScheduler(channels)
	sched.Start()
	defer sched.Stop()
	log.Println("调度器已启动")

	modbusReceiver := modbus_receiver.NewModbusReceiver(
		config.AppConfig.Server.ModbusPort,
		channels.SensorDataChan,
	)

	bearings, err := database.Instance.GetAllBearings(ctx)
	if err != nil {
		log.Printf("获取轴承列表失败: %v", err)
	} else {
		for i, b := range bearings {
			modbusReceiver.RegisterBearing(uint16(i*10), b.ID)
			log.Printf("注册Modbus地址 %d -> 轴承 %s (ID:%d)", i*10, b.BearingCode, b.ID)
		}
	}

	modbusReceiver.SetDataCallback(func(data *models.SensorData) {
		log.Printf("Modbus数据回调: 轴承ID=%d, 温度=%.2f°C", data.BearingID, data.Temperature)
	})

	go func() {
		for msg := range channels.SensorDataChan {
			if msg.Valid && msg.Data != nil {
				log.Printf("[数据总线] 传感器数据: 轴承 %d, 温度 %.2f°C, 载荷 %.0fN",
					msg.Data.BearingID, msg.Data.Temperature, msg.Data.RadialLoad)
			} else if !msg.Valid {
				log.Printf("[数据总线] 无效数据: %s", msg.Error)
			}
		}
	}()

	if err := modbusReceiver.Start(ctx); err != nil {
		log.Printf("Modbus接收器启动失败（将继续运行）: %v", err)
	} else {
		log.Printf("Modbus接收器已启动在端口 %d", config.AppConfig.Server.ModbusPort)
	}
	defer modbusReceiver.Stop()

	api.SetVersion(Version)

	r := gin.Default()
	r.Use(api.CORSMiddleware(config.AppConfig.Server.CORSOrigins))

	handler := api.NewHandler()

	r.GET("/health", handler.HealthCheck)
	r.GET("/api/health", handler.HealthCheck)

	apiV1 := r.Group("/api/v1")
	{
		apiV1.GET("/noria-wheels", handler.GetNoriaWheels)
		apiV1.GET("/bearings", handler.GetBearings)
		apiV1.GET("/bearings/:id", handler.GetBearingByID)
		apiV1.GET("/bearings/status", handler.GetBearingStatuses)

		apiV1.GET("/bearings/:bearing_id/sensor-data", handler.GetSensorData)
		apiV1.GET("/bearings/:bearing_id/sensor-data/latest", handler.GetLatestSensorData)
		apiV1.POST("/sensor-data", handler.PostSensorData)

		apiV1.GET("/bearings/:bearing_id/wear-history", handler.GetWearHistory)
		apiV1.GET("/bearings/:bearing_id/wear/latest", handler.GetLatestWearResult)
		apiV1.GET("/bearings/:bearing_id/life-prediction/latest", handler.GetLatestLifePrediction)
		apiV1.GET("/bearings/:bearing_id/oil-film-map", handler.GetOilFilmMap)

		apiV1.POST("/calculations/trigger", handler.TriggerCalculation)
		apiV1.GET("/alerts/recent", handler.GetRecentAlerts)

		apiV1.GET("/debug/weibull", handler.DebugWeibull)
	}

	r.Static("/static", "./static")

	go func() {
		addr := fmt.Sprintf(":%d", config.AppConfig.Server.Port)
		log.Printf("HTTP API 服务器启动在端口 %d", config.AppConfig.Server.Port)
		if err := r.Run(addr); err != nil {
			log.Fatalf("HTTP服务器启动失败: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("收到信号 %v, 正在关闭...", sig)
}

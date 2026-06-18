package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"noria-bearing-system/internal/api"
	"noria-bearing-system/internal/config"
	"noria-bearing-system/internal/database"
	"noria-bearing-system/internal/modbus"
	mqttpkg "noria-bearing-system/internal/mqtt"
	"noria-bearing-system/internal/models"
	"noria-bearing-system/internal/scheduler"
)

func main() {
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	if err := config.Load(configPath); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	log.Println("配置文件加载成功")

	if err := database.Connect(); err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	defer database.Instance.Close()
	log.Println("数据库连接成功")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	alertMgr := mqttpkg.NewAlertManager()
	if err := alertMgr.Start(); err != nil {
		log.Printf("MQTT告警客户端启动失败（将继续运行）: %v", err)
	}
	defer alertMgr.Stop()

	sched := scheduler.NewScheduler(alertMgr)
	sched.Start()
	defer sched.Stop()

	modbusServer := modbus.NewServer(config.AppConfig.Server.ModbusPort)

	bearings, err := database.Instance.GetAllBearings(ctx)
	if err != nil {
		log.Printf("获取轴承列表失败: %v", err)
	} else {
		for i, b := range bearings {
			modbusServer.RegisterBearing(uint16(i*10), b.ID)
			log.Printf("注册Modbus地址 %d -> 轴承 %s (ID:%d)", i*10, b.BearingCode, b.ID)
		}
	}

	modbusServer.SetDataCallback(func(data *models.SensorData) {
		log.Printf("Modbus数据回调: 轴承ID=%d", data.BearingID)
	})

	if err := modbusServer.Start(ctx); err != nil {
		log.Printf("Modbus服务器启动失败（将继续运行）: %v", err)
	}
	defer modbusServer.Stop()

	r := gin.Default()
	r.Use(api.CORSMiddleware(config.AppConfig.Server.CORSOrigins))

	handler := api.NewHandler()

	r.GET("/health", handler.HealthCheck)

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

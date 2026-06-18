@echo off
echo ================================================
echo   古代水转筒车轴承磨损仿真与寿命预测系统
echo   Ancient Noria Wheel Bearing System
echo ================================================
echo.

echo [1/4] 检查Go环境...
go version
if errorlevel 1 (
    echo [错误] 未安装Go，请先安装 Go 1.21+
    pause
    exit /b 1
)
echo.

echo [2/4] 准备后端...
cd backend
echo     下载依赖...
go mod tidy
if errorlevel 1 (
    echo [警告] 依赖下载可能有问题，请检查网络
)
echo.

echo [3/4] 准备前端静态文件...
if not exist "static" mkdir static
xcopy /E /Y /I ..\frontend static >nul
echo     完成
echo.

echo [4/4] 启动后端服务...
echo.
echo ================================================
echo   服务启动信息:
echo   - HTTP API:      http://localhost:8080
echo   - 前端页面:      http://localhost:8080/static/index.html
echo   - Modbus TCP:    localhost:5020
echo   - 健康检查:      http://localhost:8080/health
echo.
echo   模拟器启动(新终端):
echo     cd simulator
echo     python noria_sensor_simulator.py --fast
echo ================================================
echo.
echo 按 Ctrl+C 停止服务
echo.

go run main.go

cd ..
pause

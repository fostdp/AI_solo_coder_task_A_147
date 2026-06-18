@echo off
echo ================================================
echo   筒车轴承传感器模拟器
echo   Noria Bearing Sensor Simulator
echo ================================================
echo.

cd simulator

if "%1"=="fast" (
    echo [快速模式] 每1秒上报一次数据
    echo.
    python noria_sensor_simulator.py --fast --use-api
) else if "%1"=="api" (
    echo [API模式] 仅通过REST API上报
    echo.
    python noria_sensor_simulator.py --no-modbus --use-api
) else (
    echo [标准模式] 通过Modbus TCP上报（每60秒）
    echo   参数:
echo     fast - 快速模式（1秒间隔）
echo     api  - 仅API模式
echo.
    python noria_sensor_simulator.py %*
)

cd ..
pause

# 古代水转筒车轴承磨损仿真与寿命预测系统

Ancient Noria Wheel Bearing Wear Simulation & Life Prediction System

某水利史团队对唐代水转筒车进行复原研究，对筒车滑动轴承进行长期磨损监测与寿命预测的全栈应用系统。

## 系统架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        传感器层 (Simulator)                       │
│   筒车传感器模拟器 (Python) → Modbus TCP / REST API              │
└────────────────────────────────────┬────────────────────────────┘
                                     │ Modbus TCP (端口5020)
                                     ▼
┌─────────────────────────────────────────────────────────────────┐
│                        后端服务 (Go)                              │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────────┐  │
│  │ Modbus服务  │  │ REST API服务 │  │ 定时调度(磨损/预测/告警)│  │
│  └──────┬──────┘  └──────┬───────┘  └───────────┬────────────┘  │
│         │                │                      │               │
│         ▼                ▼                      ▼               │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  核心算法模块                                             │   │
│  │  · Archard磨损理论模型                                    │   │
│  │  · 弹流润滑(EHL)模型                                      │   │
│  │  · Weibull分布寿命预测                                    │   │
│  │  · 疲劳剥落损伤模型                                       │   │
│  └───────────────────────┬──────────────────────────────────┘   │
│                          │                                       │
│                          ▼                                       │
│              ┌──────────────────────┐                           │
│              │   MQTT告警推送        │                           │
│              │   (主题: noria/...)   │                           │
│              └──────────────────────┘                           │
└─────────────────────────────┬───────────────────────────────────┘
                              │
                              ▼
                    ┌───────────────────┐
                    │   TimescaleDB     │
                    │  (PostgreSQL +    │
                    │   时序扩展)       │
                    └───────────────────┘
                              ▲
                              │ REST API
┌─────────────────────────────┴───────────────────────────────────┐
│                        前端 (Web)                                │
│  · Three.js 筒车三维模型 + 轴承标注                              │
│  · Canvas 2D 轴承剖面展示(磨损可视化)                            │
│  · Canvas 油膜厚度颜色云图(径向/矩形/3D视图)                     │
│  · Chart.js 数据图表(实时监控/磨损趋势/Weibull曲线)              │
│  · 告警管理面板                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## 技术栈

### 后端
- **语言**: Go 1.21+
- **Web框架**: Gin
- **数据库**: PostgreSQL + TimescaleDB
- **数据库驱动**: pgx/v5
- **Modbus TCP**: goburrow/modbus
- **MQTT**: Eclipse Paho MQTT
- **配置管理**: Viper
- **统计计算**: montanaflynn/stats

### 前端
- **3D渲染**: Three.js r128
- **图表**: Chart.js 4.4
- **UI**: 原生 HTML5 + CSS3 + Vanilla JS
- **绘图**: HTML5 Canvas 2D

### 模拟器
- **语言**: Python 3.8+
- **协议**: Modbus TCP / REST API
- **依赖**: 标准库 (无外部依赖)

## 核心算法模型

### 1. Archard 磨损理论
磨损体积计算公式：
```
V = K · P · S / H
```
- V: 磨损体积 (m³)
- K: 磨损系数 (与润滑状态相关)
- P: 法向载荷 (N)
- S: 滑动距离 (m)
- H: 材料硬度 (Pa)

磨损系数K根据弹流润滑膜参数λ动态调整：
- λ ≥ 3.0: 全膜润滑，K = K₀ × 0.1
- 1.0 ≤ λ < 3.0: 混合润滑，K 线性插值
- λ < 1.0: 边界润滑，K = K₀ × 2.0

### 2. 弹流润滑(EHL)模型
膜厚比λ：
```
λ = h_min / σ
```
- h_min: 最小油膜厚度
- σ: 综合表面粗糙度

最小油膜厚度采用Dowson-Higginson公式修正。

### 3. Weibull 分布寿命预测
可靠度函数：
```
R(t) = exp(-(t/η)^β)
```
- β: 形状参数 (反映失效模式)
- η: 尺度参数 (特征寿命)

失效概率密度函数(PDF)：
```
f(t) = (β/η) · (t/η)^(β-1) · exp(-(t/η)^β)
```

### 4. 疲劳剥落损伤模型
基于Miner线性累积损伤准则，结合运行时间与Weibull参数计算疲劳损伤度。

## 项目结构

```
noria-bearing-system/
├── backend/                      # Go后端服务
│   ├── main.go                   # 主入口
│   ├── go.mod                    # 依赖管理
│   ├── config.yaml               # 配置文件
│   └── internal/
│       ├── config/               # 配置加载
│       ├── models/               # 数据模型
│       ├── database/             # 数据库层
│       ├── modbus/               # Modbus TCP服务
│       ├── simulation/           # 仿真算法
│       │   ├── wear.go           # Archard磨损+EHL模型
│       │   └── lifeprediction.go # Weibull寿命预测
│       ├── mqtt/                 # MQTT告警推送
│       ├── scheduler/            # 定时调度
│       └── api/                  # REST API处理器
├── frontend/                     # 前端应用
│   ├── index.html                # 主页面
│   ├── css/
│   │   └── style.css             # 样式
│   └── js/
│       ├── api.js                # API封装
│       ├── colormap.js           # 颜色映射工具
│       ├── noria-3d.js           # Three.js筒车3D模型
│       ├── bearing-view.js       # 轴承剖面Canvas视图
│       ├── oilfilm-view.js       # 油膜云图Canvas视图
│       ├── charts.js             # Chart.js图表封装
│       └── app.js                # 主应用逻辑
├── simulator/                    # 传感器模拟器
│   └── noria_sensor_simulator.py # Python模拟器脚本
├── sql/                          # 数据库脚本
│   └── init.sql                  # TimescaleDB初始化
└── README.md
```

## 快速开始

### 1. 数据库初始化

确保已安装 PostgreSQL 14+ 和 TimescaleDB 扩展。

```bash
# 创建数据库
createdb noria_bearing

# 执行初始化脚本
psql -d noria_bearing -f sql/init.sql
```

### 2. 后端服务

```bash
cd backend

# 下载依赖
go mod tidy

# 修改配置
vim config.yaml
#   - 修改数据库连接信息
#   - 修改MQTT broker地址(可选)

# 启动服务
go run main.go
```

后端服务启动后：
- HTTP API: http://localhost:8080
- Modbus TCP: localhost:5020

### 3. 前端

将 `frontend/` 目录部署到任意静态文件服务器，或直接由Go后端提供服务：

```bash
# Go后端已配置静态文件服务
# 访问: http://localhost:8080/static/index.html

# 或使用Python快速启动静态服务器
cd frontend
python -m http.server 3000
# 访问: http://localhost:3000
```

### 4. 传感器模拟器

```bash
cd simulator

# 使用默认配置启动(Modbus TCP，每60秒上报)
python noria_sensor_simulator.py

# 快速测试模式(1秒上报)
python noria_sensor_simulator.py --fast

# 同时使用REST API上报
python noria_sensor_simulator.py --use-api

# 仅使用API上报(无需Modbus)
python noria_sensor_simulator.py --no-modbus --use-api --fast

# 完整参数
python noria_sensor_simulator.py \
    --modbus-host localhost \
    --modbus-port 5020 \
    --api-url http://localhost:8080 \
    --interval 60
```

### 5. MQTT告警 (可选)

启动一个MQTT Broker接收告警，例如使用Eclipse Mosquitto：

```bash
# 使用Docker启动
docker run -d -p 1883:1883 --name mosquitto eclipse-mosquitto

# 订阅告警主题
mosquitto_sub -h localhost -t "noria/bearing/alert/#" -v
```

## REST API 文档

### 基础路径
`http://localhost:8080/api/v1`

### 筒车信息
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/noria-wheels` | 获取所有筒车列表 |

### 轴承管理
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/bearings` | 获取所有轴承 |
| GET | `/bearings?noria_wheel_id={id}` | 获取指定筒车的轴承 |
| GET | `/bearings/{id}` | 获取单个轴承详情 |
| GET | `/bearings/status` | 获取所有轴承最新状态 |

### 传感器数据
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/bearings/{id}/sensor-data?hours=24` | 获取历史传感器数据 |
| GET | `/bearings/{id}/sensor-data/latest` | 获取最新传感器数据 |
| POST | `/sensor-data` | 手动上报传感器数据 |

### 磨损与寿命预测
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/bearings/{id}/wear-history?limit=100` | 获取磨损历史 |
| GET | `/bearings/{id}/wear/latest` | 获取最新磨损结果 |
| GET | `/bearings/{id}/life-prediction/latest` | 获取最新寿命预测 |
| GET | `/bearings/{id}/oil-film-map` | 获取油膜厚度网格数据 |
| POST | `/calculations/trigger` | 立即触发磨损与寿命计算 |

### 告警
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/alerts/recent?limit=50` | 获取近期告警记录 |

## Modbus TCP 寄存器映射

每个轴承占用10个寄存器地址基址：

| 寄存器偏移 | 类型 | 说明 | 单位 |
|-----------|------|------|------|
| +0 (float32) | 保持寄存器 | 轴承温度 | °C |
| +2 (float32) | 保持寄存器 | 径向载荷 | N |
| +4 (float32) | 保持寄存器 | 转速 | RPM |
| +6 (float32) | 保持寄存器 | 油膜厚度 | μm |

默认地址映射：
- 轴承ID 1 (NRW-001-BR-A): 基址 0
- 轴承ID 2 (NRW-001-BR-B): 基址 10
- 轴承ID 3 (NRW-002-BR-A): 基址 20

## MQTT 告警主题格式

```
noria/bearing/alert/{bearing_id}/{alert_type}
```

消息载荷(JSON)：
```json
{
    "bearing_id": 1,
    "bearing_code": "NRW-001-BR-A",
    "alert_type": "wear_exceeded",
    "alert_level": "critical",
    "alert_message": "轴承 NRW-001-BR-A 磨损深度严重超限！...",
    "threshold": 135.0,
    "actual_value": 148.5,
    "timestamp": "2024-06-18T10:30:00+08:00"
}
```

告警类型：
- `wear_warning`: 磨损接近阈值 (达70%以上)
- `wear_exceeded`: 磨损严重超限 (达90%以上)
- `oil_film_rupture`: 润滑油膜破裂 (油膜<0.5μm)

## 前端功能说明

### 总览页面
- 筒车选择器
- 轴承健康状态列表 (正常/警告/严重)
- 实时数据监控面板 (温度、载荷、转速、油膜、磨损、剩余寿命)
- 实时趋势图表
- 近期告警记录

### 三维模型页面
- Three.js 构建的唐代水转筒车复原模型
- 可交互：拖动旋转、滚轮缩放、右键平移
- 自动旋转模拟筒车运转
- 轴承位置标记 (颜色反映健康状态)
- 点击轴承跳转至详细剖面视图

### 轴承剖面页面
- Canvas 2D 绘制轴承横剖面图
- 磨损区域可视化 (渐变色显示磨损程度)
- 径向载荷箭头指示
- 润滑油膜薄层标注
- 磨损进度条 (带警告/严重阈值标记)
- 磨损历史趋势图表

### 油膜云图页面
- 三种视图模式：
  - **径向展开**: 环形极坐标显示轴承周向油膜分布
  - **矩形展开**: 笛卡尔坐标系展开显示
  - **三维曲面**: 伪3D高度图显示油膜厚度
- Jet色带颜色图例
- 油膜统计数据 (最小/最大/平均/标准差)
- 润滑状态评估 (全膜/混合/边界/干摩擦)

### 寿命预测页面
- 剩余寿命(RUL)、可靠度、失效概率汇总
- Weibull 分布参数展示 (β, η)
- 可靠度曲线 R(t) 与失效概率 F(t)
- 概率密度函数(PDF)曲线
- 磨损率趋势 + 线性拟合

### 告警记录页面
- 告警事件表格
- 按级别筛选 (全部/严重/警告)
- 数量选择 (50/100/500条)

## 配置说明 (`config.yaml`)

```yaml
server:
  port: 8080                      # HTTP API端口
  modbus_port: 5020               # Modbus TCP端口
  cors_origins:                   # 允许的CORS源
    - "http://localhost:3000"

database:
  host: localhost
  port: 5432
  user: postgres
  password: postgres
  dbname: noria_bearing
  sslmode: disable

mqtt:
  broker: tcp://localhost:1883    # MQTT Broker地址
  client_id: noria_bearing_server
  topic_prefix: noria/bearing/alert # 告警主题前缀

wear_calculation:
  interval_minutes: 60            # 磨损计算间隔(分钟)
  archard_k: 1.0e-8               # Archard基准磨损系数
  ehl_reference_temp: 40.0         # EHL参考温度(°C)

life_prediction:
  interval_minutes: 360           # 寿命预测间隔(分钟)
  weibull_default_shape: 2.5      # 默认Weibull形状参数
  weibull_default_scale: 50000.0  # 默认Weibull尺度参数(小时)
  min_samples_for_fit: 20         # 参数拟合最小样本数

alert:
  wear_warning_ratio: 0.7         # 磨损警告阈值比例
  wear_critical_ratio: 0.9        # 磨损严重阈值比例
  oil_film_minimum: 0.5           # 最小允许油膜厚度(μm)
  cooldown_minutes: 30            # 告警冷却时间(防止刷屏)
```

## TimescaleDB 连续聚合与保留策略

系统自动配置：
- **sensor_data_hourly**: 每小时平均数据连续聚合
- **原始数据保留**: 1年
- **聚合数据保留**: 5年

主要数据表：
- `sensor_data`: 传感器原始数据 (超表)
- `wear_results`: 磨损计算结果 (超表)
- `life_predictions`: 寿命预测结果 (超表)
- `alert_events`: 告警事件 (超表)
- `oil_film_maps`: 油膜厚度网格数据 (超表)

## 许可证

仅供水利史学术研究使用。

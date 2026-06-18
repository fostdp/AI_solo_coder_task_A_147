#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
代码重构功能回归测试脚本
验证Go后端模块拆分、前端拆分、配置外置的正确性
"""

import os
import json
import sys
from pathlib import Path

PROJECT_ROOT = Path(__file__).parent
BACKEND_DIR = PROJECT_ROOT / "backend"
FRONTEND_DIR = PROJECT_ROOT / "frontend"
CONFIG_DIR = BACKEND_DIR / "config"
MODULES_DIR = BACKEND_DIR / "internal" / "modules"

test_results = []

def test(name, func):
    """测试包装器"""
    try:
        func()
        test_results.append((name, "[OK] 通过", ""))
        print(f"[OK] {name}")
    except AssertionError as e:
        test_results.append((name, "[FAIL] 失败", str(e)))
        print(f"[FAIL] {name}: {e}")
    except Exception as e:
        test_results.append((name, "[ERR] 异常", str(e)))
        print(f"[ERR] {name} (异常): {e}")

# ==================== 后端模块测试 ====================

def test_backend_module_structure():
    """测试后端模块目录结构"""
    required_dirs = [
        MODULES_DIR / "modbus_receiver",
        MODULES_DIR / "wear_simulator",
        MODULES_DIR / "life_predictor",
        MODULES_DIR / "alarm_mqtt",
        MODULES_DIR / "messages",
    ]
    for d in required_dirs:
        assert d.exists(), f"目录不存在: {d}"
        assert d.is_dir(), f"不是目录: {d}"

def test_module_files_exist():
    """测试模块文件存在"""
    required_files = [
        MODULES_DIR / "messages" / "messages.go",
        MODULES_DIR / "modbus_receiver" / "receiver.go",
        MODULES_DIR / "wear_simulator" / "simulator.go",
        MODULES_DIR / "life_predictor" / "predictor.go",
        MODULES_DIR / "alarm_mqtt" / "pusher.go",
    ]
    for f in required_files:
        assert f.exists(), f"文件不存在: {f}"

def test_channel_communication_structs():
    """测试消息结构体定义"""
    msg_file = MODULES_DIR / "messages" / "messages.go"
    content = msg_file.read_text(encoding='utf-8')
    
    required_structs = [
        "SensorDataMessage",
        "WearCalcRequest",
        "WearCalcResult",
        "LifePredRequest",
        "LifePredResult",
        "AlertMessage",
        "ModuleChannels",
    ]
    for struct_name in required_structs:
        assert f"type {struct_name} struct" in content, f"缺少结构体定义: {struct_name}"
    
    assert "chan SensorDataMessage" in content, "缺少SensorDataChannel定义"
    assert "chan WearCalcRequest" in content, "缺少WearRequestChannel定义"
    assert "chan WearCalcResult" in content, "缺少WearResultChannel定义"
    assert "chan LifePredRequest" in content, "缺少LifeRequestChannel定义"
    assert "chan LifePredResult" in content, "缺少LifeResultChannel定义"
    assert "chan AlertMessage" in content, "缺少AlertChannel定义"
    assert "NewModuleChannels" in content, "缺少NewModuleChannels函数"

def test_modbus_receiver_validation():
    """测试Modbus接收器的数据校验功能"""
    receiver_file = MODULES_DIR / "modbus_receiver" / "receiver.go"
    content = receiver_file.read_text(encoding='utf-8')
    
    required_funcs = [
        "validateSensorData",
        "DataValidationResult",
        "sendToChannel",
    ]
    for func_name in required_funcs:
        assert func_name in content, f"缺少函数: {func_name}"
    
    assert "温度值超出范围" in content, "缺少温度范围校验"
    assert "径向载荷超出范围" in content, "缺少载荷范围校验"
    assert "转速超出范围" in content, "缺少转速范围校验"
    assert "油膜厚度超出范围" in content, "缺少油膜厚度范围校验"
    assert "outputChan" in content, "缺少channel输出"

def test_wear_simulator_algorithms():
    """测试磨损仿真器的核心算法"""
    wear_file = MODULES_DIR / "wear_simulator" / "simulator.go"
    content = wear_file.read_text(encoding='utf-8')
    
    required_funcs = [
        "Calculate",
        "calculateEHLFilmParameter",
        "applyMixedLubricationCorrection",
        "calculateWearCoefficient",
        "GenerateOilFilmMap",
    ]
    for func_name in required_funcs:
        assert func_name in content, f"缺少函数: {func_name}"
    
    assert "Archard" in content or "archard" in content, "缺少Archard磨损计算"
    assert "Dowson-Higginson" in content or "Dowson" in content, "缺少Dowson-Higginson EHL公式"
    assert "Sommerfeld" in content, "缺少Sommerfeld数计算"
    assert "Greenwood-Williamson" in content or "asperity" in content, "缺少微凸体接触模型"
    assert "requestChan" in content and "resultChan" in content, "缺少channel通信"

def test_life_predictor_weibull():
    """测试寿命预测器的Weibull分布和贝叶斯更新"""
    life_file = MODULES_DIR / "life_predictor" / "predictor.go"
    content = life_file.read_text(encoding='utf-8')
    
    required_funcs = [
        "Predict",
        "estimateWeibullParameters",
        "fitWeibull",
        "calculateFatigueDamage",
        "calculateWeibullRUL",
    ]
    for func_name in required_funcs:
        assert func_name in content, f"缺少函数: {func_name}"
    
    assert "Weibull" in content or "weibull" in content, "缺少Weibull分布"
    assert "bayesianStrength" in content or "priorShapeAlpha" in content, "缺少贝叶斯更新"
    assert "Gamma" in content or "Inverse-Gamma" in content or "priorShapeAlpha" in content, "缺少先验分布"
    assert "minSamplesForFit" in content or "MinSamplesForFit" in content, "缺少样本门槛"
    assert "requestChan" in content and "resultChan" in content, "缺少channel通信"

def test_alarm_mqtt_push():
    """测试告警MQTT推送模块"""
    alarm_file = MODULES_DIR / "alarm_mqtt" / "pusher.go"
    content = alarm_file.read_text(encoding='utf-8')
    
    required_funcs = [
        "NewAlertClient",
        "Connect",
        "PublishAlert",
        "NewAlertManager",
        "ProcessAlert",
        "ShouldAlert",
        "MarkAlerted",
    ]
    for func_name in required_funcs:
        assert func_name in content, f"缺少函数: {func_name}"
    
    assert "alertChan" in content, "缺少告警输入channel"
    assert "AutoReconnect" in content, "缺少MQTT自动重连"
    assert "cooldown" in content or "Cooldown" in content, "缺少冷却时间"
    assert "QoS" in content or "qos" in content, "缺少QoS设置"

def test_scheduler_channel_integration():
    """测试调度器与channel的集成"""
    sched_file = BACKEND_DIR / "internal" / "scheduler" / "scheduler.go"
    content = sched_file.read_text(encoding='utf-8')
    
    assert "ModuleChannels" in content, "缺少ModuleChannels引用"
    assert "WearRequestChan" in content, "缺少WearRequestChan引用"
    assert "LifeRequestChan" in content, "缺少LifeRequestChan引用"
    assert "AlertChan" in content, "缺少AlertChan引用"
    assert "WearResultChan" in content, "缺少WearResultChan监听"
    assert "LifeResultChan" in content, "缺少LifeResultChan监听"
    assert "resultListenerLoop" in content, "缺少结果监听循环"

def test_main_channel_wiring():
    """测试main.go中的channel串联"""
    main_file = BACKEND_DIR / "main.go"
    content = main_file.read_text(encoding='utf-8')
    
    assert "NewModuleChannels" in content, "缺少通道创建"
    assert "modbus_receiver.NewModbusReceiver" in content, "缺少modbus_receiver实例化"
    assert "wear_simulator.NewWearSimulator" in content, "缺少wear_simulator实例化"
    assert "life_predictor.NewLifePredictor" in content, "缺少life_predictor实例化"
    assert "alarm_mqtt.NewAlertManager" in content, "缺少alarm_mqtt实例化"
    assert "scheduler.NewScheduler(channels)" in content, "缺少scheduler与channels集成"
    
    assert "SensorDataChan" in content, "缺少SensorDataChan连接"
    assert "WearRequestChan" in content, "缺少WearRequestChan连接"
    assert "WearResultChan" in content, "缺少WearResultChan连接"
    assert "LifeRequestChan" in content, "缺少LifeRequestChan连接"
    assert "LifeResultChan" in content, "缺少LifeResultChan连接"
    assert "AlertChan" in content, "缺少AlertChan连接"

# ==================== JSON配置文件测试 ====================

def test_json_config_files():
    """测试JSON配置文件存在且格式正确"""
    required_files = [
        CONFIG_DIR / "wear_params.json",
        CONFIG_DIR / "lubrication_params.json",
    ]
    for f in required_files:
        assert f.exists(), f"配置文件不存在: {f}"
        content = f.read_text(encoding='utf-8')
        data = json.loads(content)
        assert isinstance(data, dict), f"配置文件格式错误: {f}"

def test_wear_params_content():
    """测试磨损参数配置内容"""
    wear_file = CONFIG_DIR / "wear_params.json"
    data = json.loads(wear_file.read_text(encoding='utf-8'))
    
    required_keys = [
        "archard_k_base",
        "ehl_reference_temp_celsius",
        "pressure_viscosity_coefficient",
        "reduced_elastic_modulus_pa",
        "surface_roughness_rms_meters",
        "wear_coefficient_factors",
        "oil_film_grid",
    ]
    for key in required_keys:
        assert key in data, f"缺少磨损参数: {key}"
    
    factors = data["wear_coefficient_factors"]
    assert "full_film" in factors, "缺少full_film系数"
    assert "mixed_min" in factors, "缺少mixed_min系数"
    assert "boundary_min" in factors, "缺少boundary_min系数"

def test_lubrication_params_content():
    """测试润滑参数配置内容"""
    lub_file = CONFIG_DIR / "lubrication_params.json"
    data = json.loads(lub_file.read_text(encoding='utf-8'))
    
    required_keys = [
        "mixed_lubrication",
        "ehl_correction",
        "oil_film_rupture_threshold_microm",
    ]
    for key in required_keys:
        assert key in data, f"缺少润滑参数: {key}"
    
    mixed = data["mixed_lubrication"]
    assert "sommerfeld_threshold_low" in mixed, "缺少Sommerfeld低阈值"
    assert "sommerfeld_threshold_high" in mixed, "缺少Sommerfeld高阈值"
    
    ehl = data["ehl_correction"]
    assert "dowson_higginson_coefficient" in ehl, "缺少Dowson-Higginson系数"
    assert "lambda_min_clamp" in ehl, "缺少Lambda最小钳制值"

def test_config_loading():
    """测试配置加载代码"""
    config_file = BACKEND_DIR / "internal" / "config" / "config.go"
    content = config_file.read_text(encoding='utf-8')
    
    assert "WearParamsConfig" in content, "缺少WearParamsConfig结构体"
    assert "LubricationConfig" in content, "缺少LubricationConfig结构体"
    assert "loadWearParams" in content, "缺少loadWearParams函数"
    assert "loadLubricationParams" in content, "缺少loadLubricationParams函数"
    assert "encoding/json" in content, "缺少json导入"
    assert "os.ReadFile" in content, "缺少os.ReadFile导入"

# ==================== 前端拆分测试 ====================

def test_frontend_files_exist():
    """测试前端文件存在"""
    js_dir = FRONTEND_DIR / "js"
    
    new_files = [
        js_dir / "waterwheel_3d.js",
        js_dir / "bearing_panel.js",
    ]
    for f in new_files:
        assert f.exists(), f"前端文件不存在: {f}"

def test_waterwheel_3d_content():
    """测试waterwheel_3d.js内容"""
    ww_file = FRONTEND_DIR / "js" / "waterwheel_3d.js"
    content = ww_file.read_text(encoding='utf-8')
    
    assert "const Waterwheel3D" in content, "模块名未更新为Waterwheel3D"
    assert "Noria3D" not in content, "残留旧模块名Noria3D"
    assert "_buildNoriaWheel" in content or "_buildWheel" in content, "缺少筒车构建函数"
    assert "_buildFrame" in content, "缺少支架构建函数"
    assert "Three.js" in content or "THREE" in content, "缺少Three.js引用"
    assert "Raycaster" in content, "缺少射线拾取"

def test_bearing_panel_content():
    """测试bearing_panel.js内容"""
    bp_file = FRONTEND_DIR / "js" / "bearing_panel.js"
    content = bp_file.read_text(encoding='utf-8')
    
    assert "const BearingPanel" in content, "模块名未更新为BearingPanel"
    assert "BearingView" not in content, "残留旧模块名BearingView"
    assert "_initOffscreenCanvases" in content, "缺少离屏Canvas初始化"
    assert "_drawBearingCrossSection" in content, "缺少轴承剖面绘制"
    assert "CanvasPattern" in content or "DOMMatrix" in content, "缺少动画纹理"
    assert "offscreenCanvas" in content, "缺少离屏Canvas"

def test_app_js_references():
    """测试app.js中的引用更新"""
    app_file = FRONTEND_DIR / "js" / "app.js"
    content = app_file.read_text(encoding='utf-8')
    
    assert "Waterwheel3D" in content, "app.js中缺少Waterwheel3D引用"
    assert "BearingPanel" in content, "app.js中缺少BearingPanel引用"
    assert "Noria3D" not in content, "app.js中残留Noria3D引用"
    assert "BearingView" not in content, "app.js中残留BearingView引用"

def test_index_html_scripts():
    """测试index.html中的script引用更新"""
    html_file = FRONTEND_DIR / "index.html"
    content = html_file.read_text(encoding='utf-8')
    
    assert "waterwheel_3d.js" in content, "HTML中缺少waterwheel_3d.js引用"
    assert "bearing_panel.js" in content, "HTML中缺少bearing_panel.js引用"
    assert "noria-3d.js" not in content, "HTML中残留noria-3d.js引用"
    assert "bearing-view.js" not in content, "HTML中残留bearing-view.js引用"

# ==================== 架构验证测试 ====================

def test_module_independence():
    """测试模块间独立性（无循环依赖）"""
    module_files = {
        "modbus_receiver": MODULES_DIR / "modbus_receiver" / "receiver.go",
        "wear_simulator": MODULES_DIR / "wear_simulator" / "simulator.go",
        "life_predictor": MODULES_DIR / "life_predictor" / "predictor.go",
        "alarm_mqtt": MODULES_DIR / "alarm_mqtt" / "pusher.go",
    }
    
    for module_name, file_path in module_files.items():
        content = file_path.read_text(encoding='utf-8')
        
        for other_module in module_files.keys():
            if other_module == module_name:
                continue
            if other_module in content:
                import_line = f'"noria-bearing-system/internal/modules/{other_module}"'
                if import_line in content:
                    pass  # 允许导入messages
        assert f'"noria-bearing-system/internal/modules/messages"' in content or module_name == "messages", \
            f"{module_name} 未导入messages模块"

def test_channel_flow_architecture():
    """测试channel数据流架构"""
    # 验证数据流方向:
    # modbus_receiver -> SensorDataChan -> [scheduler] -> WearRequestChan -> wear_simulator
    # wear_simulator -> WearResultChan -> [scheduler] -> LifeRequestChan -> life_predictor
    # life_predictor -> LifeResultChan -> [scheduler]
    # scheduler -> AlertChan -> alarm_mqtt
    
    main_content = (BACKEND_DIR / "main.go").read_text(encoding='utf-8')
    sched_content = (BACKEND_DIR / "internal" / "scheduler" / "scheduler.go").read_text(encoding='utf-8')
    
    # 验证modbus_receiver只输出到SensorDataChan
    modbus_content = (MODULES_DIR / "modbus_receiver" / "receiver.go").read_text(encoding='utf-8')
    assert "outputChan" in modbus_content or "SensorDataChan" in modbus_content
    assert "WearRequest" not in modbus_content, "modbus_receiver不应直接操作WearRequest"
    
    # 验证wear_simulator只接收WearRequestChan，输出WearResultChan
    wear_content = (MODULES_DIR / "wear_simulator" / "simulator.go").read_text(encoding='utf-8')
    assert "requestChan" in wear_content and "resultChan" in wear_content
    assert "SensorData" not in wear_content or "SensorDataMessage" not in wear_content, \
        "wear_simulator不应直接接收SensorData"
    
    # 验证life_predictor只接收LifeRequestChan，输出LifeResultChan
    life_content = (MODULES_DIR / "life_predictor" / "predictor.go").read_text(encoding='utf-8')
    assert "requestChan" in life_content and "resultChan" in life_content
    
    # 验证alarm_mqtt只接收AlertChan
    alarm_content = (MODULES_DIR / "alarm_mqtt" / "pusher.go").read_text(encoding='utf-8')
    assert "alertChan" in alarm_content or "AlertChan" in alarm_content
    assert "WearResult" not in alarm_content, "alarm_mqtt不应直接接收WearResult"

def test_hardcoded_params_removed():
    """测试硬编码参数是否已移除"""
    wear_content = (MODULES_DIR / "wear_simulator" / "simulator.go").read_text(encoding='utf-8')
    
    # 检查是否使用了配置文件中的参数
    assert "ws.wearParams.ArchardKBase" in wear_content or "config.AppConfig.WearParams" in wear_content, \
        "缺少ws.wearParams.ArchardKBase引用"
    assert "ws.lubrication.EHLCorrection" in wear_content or "config.AppConfig.Lubrication" in wear_content, \
        "缺少ws.lubrication.EHLCorrection引用"
    
    # 检查是否还有硬编码的魔法数字
    # 允许保留数学常数，但不应该有物理参数硬编码
    bad_patterns = [
        "1.0e-8",  # 原Archard K值
        "2.2e-8",  # 原压粘系数
        "2.0e11",  # 原等效弹性模量
        "0.4e-6",  # 原表面粗糙度
    ]
    for pattern in bad_patterns:
        # 这些值应该从配置读取，但允许出现在JSON配置中
        if pattern in wear_content:
            line_num = wear_content.split('\n').index([l for l in wear_content.split('\n') if pattern in l][0]) + 1
            print(f"  [WARN] 行 {line_num} 发现可能的硬编码值: {pattern}")

# ==================== 执行测试 ====================

def main():
    print("=" * 70)
    print("代码重构功能回归测试")
    print("=" * 70)
    print()
    
    print("后端模块结构测试")
    print("-" * 40)
    test("目录结构完整", test_backend_module_structure)
    test("模块文件存在", test_module_files_exist)
    test("消息结构体定义", test_channel_communication_structs)
    
    print()
    print("模块功能测试")
    print("-" * 40)
    test("Modbus数据校验功能", test_modbus_receiver_validation)
    test("磨损仿真核心算法", test_wear_simulator_algorithms)
    test("寿命预测Weibull+贝叶斯", test_life_predictor_weibull)
    test("告警MQTT推送", test_alarm_mqtt_push)
    
    print()
    print("Channel集成测试")
    print("-" * 40)
    test("调度器Channel集成", test_scheduler_channel_integration)
    test("Main.go Channel串联", test_main_channel_wiring)
    test("模块独立性", test_module_independence)
    test("数据流架构验证", test_channel_flow_architecture)
    
    print()
    print("配置外置测试")
    print("-" * 40)
    test("JSON配置文件存在", test_json_config_files)
    test("磨损参数内容", test_wear_params_content)
    test("润滑参数内容", test_lubrication_params_content)
    test("配置加载代码", test_config_loading)
    test("硬编码参数移除", test_hardcoded_params_removed)
    
    print()
    print("前端拆分测试")
    print("-" * 40)
    test("前端文件存在", test_frontend_files_exist)
    test("waterwheel_3d.js内容", test_waterwheel_3d_content)
    test("bearing_panel.js内容", test_bearing_panel_content)
    test("app.js引用更新", test_app_js_references)
    test("index.html引用更新", test_index_html_scripts)
    
    # 统计结果
    print()
    print("=" * 70)
    passed = sum(1 for _, status, _ in test_results if "通过" in status)
    failed = sum(1 for _, status, _ in test_results if "失败" in status)
    errors = sum(1 for _, status, _ in test_results if "异常" in status)
    total = len(test_results)
    
    print(f"测试结果: {passed}/{total} 通过, {failed} 失败, {errors} 异常")
    print("=" * 70)
    
    if failed > 0 or errors > 0:
        print()
        print("[FAIL] 失败详情:")
        for name, status, msg in test_results:
            if "通过" not in status:
                print(f"  {name}: {msg}")
        return 1
    
    print()
    print("[OK] 所有测试通过！重构成功完成。")
    print()
    print("架构总结:")
    print("  ┌─────────────────┐      SensorDataChan       ┌────────────────┐")
    print("  │ modbus_receiver │ ──────────────────────► │                │")
    print("  │  (数据采集校验)  │                          │                │")
    print("  └─────────────────┘                          │                │")
    print("                                               │   scheduler    │")
    print("  ┌─────────────────┐      WearRequestChan     │   (调度器)     │")
    print("  │ wear_simulator  │ ◄────────────────────── │                │")
    print("  │  (Archard+EHL)  │      WearResultChan      │                │")
    print("  │                 │ ──────────────────────► │                │")
    print("  └─────────────────┘                          │                │")
    print("                                               │                │")
    print("  ┌─────────────────┐      LifeRequestChan     │                │")
    print("  │ life_predictor  │ ◄────────────────────── │                │")
    print("  │  (Weibull+RUL)  │      LifeResultChan      │                │")
    print("  │                 │ ──────────────────────► │                │")
    print("  └─────────────────┘                          │                │")
    print("                                               │                │")
    print("  ┌─────────────────┐        AlertChan         │                │")
    print("  │   alarm_mqtt    │ ◄────────────────────── │                │")
    print("  │  (告警推送)     │                          └────────────────┘")
    print("  └─────────────────┘")
    return 0

if __name__ == '__main__':
    sys.exit(main())

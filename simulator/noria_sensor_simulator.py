#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
古代水转筒车轴承传感器模拟器
通过 Modbus TCP 协议模拟上报轴承温度、径向载荷、转速、润滑油膜厚度数据

支持多工况配置：
  - load-profile: light / normal / heavy / extreme
  - speed-profile: low / normal / high / surge
"""

import struct
import socket
import time
import random
import math
import json
import argparse
import threading
import sys
from dataclasses import dataclass, field
from datetime import datetime
from typing import Callable, Dict


LOAD_PROFILES: Dict[str, Dict] = {
    "light": {
        "name": "轻载工况",
        "base_load": 2000.0,
        "load_noise": 150.0,
        "load_min": 300.0,
        "load_max": 8000.0,
        "load_peak_prob": 0.001,
        "load_peak_range": (1.3, 1.8),
        "wear_rate": 0.001,
        "desc": "枯水期低负荷，约2000N",
    },
    "normal": {
        "name": "正常工况",
        "base_load": 5000.0,
        "load_noise": 300.0,
        "load_min": 500.0,
        "load_max": 20000.0,
        "load_peak_prob": 0.003,
        "load_peak_range": (1.5, 2.5),
        "wear_rate": 0.003,
        "desc": "常规水量，约5000N",
    },
    "heavy": {
        "name": "重载工况",
        "base_load": 10000.0,
        "load_noise": 600.0,
        "load_min": 2000.0,
        "load_max": 40000.0,
        "load_peak_prob": 0.01,
        "load_peak_range": (1.8, 3.0),
        "wear_rate": 0.008,
        "desc": "汛期高负荷，约10000N",
    },
    "extreme": {
        "name": "极端工况",
        "base_load": 18000.0,
        "load_noise": 1200.0,
        "load_min": 5000.0,
        "load_max": 60000.0,
        "load_peak_prob": 0.03,
        "load_peak_range": (2.0, 4.0),
        "wear_rate": 0.02,
        "desc": "洪水冲击+满载，约18000N，加速磨损",
    },
}

SPEED_PROFILES: Dict[str, Dict] = {
    "low": {
        "name": "低速",
        "base_speed": 6.0,
        "speed_noise": 0.4,
        "speed_min": 1.0,
        "speed_max": 15.0,
        "speed_surge_prob": 0.0,
        "desc": "枯水期慢转，约6 RPM",
    },
    "normal": {
        "name": "正常转速",
        "base_speed": 15.0,
        "speed_noise": 0.8,
        "speed_min": 2.0,
        "speed_max": 50.0,
        "speed_surge_prob": 0.0,
        "desc": "常规水流，约15 RPM",
    },
    "high": {
        "name": "高速",
        "base_speed": 30.0,
        "speed_noise": 1.5,
        "speed_min": 5.0,
        "speed_max": 80.0,
        "speed_surge_prob": 0.0,
        "desc": "大流量，约30 RPM",
    },
    "surge": {
        "name": "波动转速",
        "base_speed": 20.0,
        "speed_noise": 3.0,
        "speed_min": 1.0,
        "speed_max": 90.0,
        "speed_surge_prob": 0.02,
        "speed_surge_range": (0.2, 1.8),
        "desc": "水流不稳定，转速大幅波动",
    },
}


@dataclass
class BearingSimulator:
    bearing_id: int
    modbus_addr: int
    bearing_code: str
    position: str
    load_profile: str = "normal"
    speed_profile: str = "normal"

    base_temp: float = 35.0
    base_film: float = 3.5

    wear_accumulated: float = 0.0
    start_time: float = field(default_factory=time.time)

    def __post_init__(self):
        if self.load_profile not in LOAD_PROFILES:
            raise ValueError(f"未知载荷工况: {self.load_profile}")
        if self.speed_profile not in SPEED_PROFILES:
            raise ValueError(f"未知转速工况: {self.speed_profile}")

    @property
    def load_cfg(self) -> Dict:
        return LOAD_PROFILES[self.load_profile]

    @property
    def speed_cfg(self) -> Dict:
        return SPEED_PROFILES[self.speed_profile]

    @property
    def wear_rate(self) -> float:
        return self.load_cfg["wear_rate"]

    def generate_data(self, elapsed_hours: float) -> dict:
        wear = self.wear_accumulated + self.wear_rate * elapsed_hours
        wear_factor = 1.0 + wear * 0.005

        t = time.time()
        daily_cycle = math.sin(t / 86400 * 2 * math.pi) * 5.0
        temp = self.base_temp + daily_cycle + random.gauss(0, 1.5) + wear * 0.02

        water_flow = max(0.3, 1.0 + 0.3 * math.sin(t / 3600 * 2 * math.pi))
        load = (
            self.load_cfg["base_load"] * water_flow * wear_factor
            + random.gauss(0, self.load_cfg["load_noise"])
        )
        if random.random() < self.load_cfg["load_peak_prob"]:
            peak_range = self.load_cfg["load_peak_range"]
            load *= random.uniform(*peak_range)
        load = max(self.load_cfg["load_min"], min(self.load_cfg["load_max"], load))
        temp += (load / self.load_cfg["base_load"] - 1.0) * 5.0

        speed = (
            self.speed_cfg["base_speed"] * math.sqrt(water_flow)
            * (1.0 - wear * 0.002)
        )
        speed += random.gauss(0, self.speed_cfg["speed_noise"])
        if self.speed_cfg.get("speed_surge_prob", 0) > 0:
            if random.random() < self.speed_cfg["speed_surge_prob"]:
                speed *= random.uniform(*self.speed_cfg["speed_surge_range"])
        speed = max(self.speed_cfg["speed_min"], min(self.speed_cfg["speed_max"], speed))

        ehl_factor = (
            (max(20, temp) / 40.0) ** -0.5
            * (speed / self.speed_cfg["base_speed"]) ** 0.7
            * (load / self.load_cfg["base_load"]) ** -0.3
        )
        film = self.base_film * ehl_factor
        film -= wear * 0.015
        film += random.gauss(0, 0.15)

        if random.random() < 0.005:
            film *= random.uniform(0.1, 0.4)
            temp += random.uniform(5, 15)

        temp = max(15, min(100, temp))
        film = max(0.05, min(8.0, film))

        self.wear_accumulated = wear

        return {
            "bearing_id": self.bearing_id,
            "bearing_code": self.bearing_code,
            "position": self.position,
            "load_profile": self.load_profile,
            "speed_profile": self.speed_profile,
            "temperature": round(temp, 4),
            "radial_load": round(load, 4),
            "rotational_speed": round(speed, 4),
            "oil_film_thickness": round(film, 6),
            "wear_accumulated_um": round(wear, 6),
            "elapsed_hours": round(elapsed_hours, 4),
            "timestamp": datetime.now().isoformat(),
        }


def encode_float32(value: float) -> bytes:
    return struct.pack(">f", value)


def build_modbus_tcp_packet(transaction_id: int, unit_id: int, values: list) -> bytes:
    quantity = len(values)
    pdu = bytes([unit_id, 0x10, 0x00, 0x00, (quantity >> 8) & 0xFF, quantity & 0xFF, quantity * 2])
    for v in values:
        pdu += encode_float32(v)
    mbap = struct.pack(">HHHB", transaction_id, 0x0000, len(pdu), unit_id)
    return mbap + pdu


def send_modbus_tcp(host: str, port: int, unit_id: int, transaction_id: int, values: list):
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.settimeout(3.0)
    try:
        sock.connect((host, port))
        packet = build_modbus_tcp_packet(transaction_id, unit_id, values)
        sock.sendall(packet)
        return True
    except Exception as e:
        print(f"[ERR] Modbus发送失败: {e}")
        return False
    finally:
        sock.close()


def print_status(bearing: BearingSimulator, data: dict, idx: int, total: int):
    bar_len = 30
    load_pct = min(1.0, data["radial_load"] / LOAD_PROFILES[bearing.load_profile]["load_max"])
    filled = int(bar_len * load_pct)
    bar = "#" * filled + "-" * (bar_len - filled)

    ts = datetime.now().strftime("%H:%M:%S")
    print(
        f"[{ts}] [{idx}/{total}] {bearing.bearing_code} "
        f"载荷={data['radial_load']:>7.0f}N [{bar}] "
        f"转速={data['rotational_speed']:>5.1f}RPM "
        f"温度={data['temperature']:>5.1f}°C "
        f"油膜={data['oil_film_thickness']:>5.2f}μm "
        f"磨损={data['wear_accumulated_um']:>6.3f}μm "
        f"工况={bearing.load_profile}/{bearing.speed_profile}"
    )


def parse_args():
    p = argparse.ArgumentParser(description="水转筒车轴承Modbus传感器模拟器")
    p.add_argument("--host", default="127.0.0.1", help="Modbus TCP服务端地址")
    p.add_argument("--port", type=int, default=5020, help="Modbus TCP端口")
    p.add_argument("--bearing-id", type=int, default=1, help="轴承ID（单轴承模式）")
    p.add_argument("--interval", type=float, default=3600, help="上报间隔秒数（默认3600，即1小时）")
    p.add_argument(
        "--load-profile",
        choices=list(LOAD_PROFILES.keys()),
        default="normal",
        help="载荷工况: " + ", ".join(f"{k}={v['desc']}" for k, v in LOAD_PROFILES.items()),
    )
    p.add_argument(
        "--speed-profile",
        choices=list(SPEED_PROFILES.keys()),
        default="normal",
        help="转速工况: " + ", ".join(f"{k}={v['desc']}" for k, v in SPEED_PROFILES.items()),
    )
    p.add_argument("--count", type=int, default=0, help="运行次数（0为无限循环）")
    p.add_argument("--once", action="store_true", help="只发送一次数据后退出")
    p.add_argument("--list-profiles", action="store_true", help="列出所有工况配置")
    p.add_argument("--fast", action="store_true", help="快速模式：间隔1秒模拟1小时数据（加速仿真）")
    p.add_argument("--json", action="store_true", help="输出JSON格式数据到stdout")
    return p.parse_args()


def list_profiles():
    print("=" * 70)
    print("可用载荷工况 (--load-profile)")
    print("=" * 70)
    for k, v in LOAD_PROFILES.items():
        print(f"  {k:<8} {v['name']:<10} 基准={v['base_load']:>6.0f}N 磨损率={v['wear_rate']:.4f}  {v['desc']}")
    print()
    print("=" * 70)
    print("可用转速工况 (--speed-profile)")
    print("=" * 70)
    for k, v in SPEED_PROFILES.items():
        print(f"  {k:<8} {v['name']:<10} 基准={v['base_speed']:>5.1f}RPM  {v['desc']}")
    print()


def main():
    args = parse_args()

    if args.list_profiles:
        list_profiles()
        return 0

    interval = 1.0 if args.fast else args.interval
    sim_speed = 3600.0 if args.fast else 1.0

    print("=" * 70)
    print("水转筒车轴承Modbus传感器模拟器")
    print("=" * 70)
    print(f"  Modbus目标:      {args.host}:{args.port}")
    print(f"  轴承ID:          {args.bearing_id}")
    print(f"  载荷工况:        {args.load_profile} - {LOAD_PROFILES[args.load_profile]['desc']}")
    print(f"  转速工况:        {args.speed_profile} - {SPEED_PROFILES[args.speed_profile]['desc']}")
    print(f"  上报间隔:        {interval}s {'(快速模式: 1s≈1h)' if args.fast else ''}")
    print(f"  运行次数:        {'无限' if args.count == 0 and not args.once else (1 if args.once else args.count)}")
    print("=" * 70)
    print()

    bearing = BearingSimulator(
        bearing_id=args.bearing_id,
        modbus_addr=args.bearing_id * 10,
        bearing_code=f"SIM-{args.bearing_id:03d}",
        position="模拟主轴轴承",
        load_profile=args.load_profile,
        speed_profile=args.speed_profile,
    )

    txn_id = 1
    count = 0
    start = time.time()

    try:
        while True:
            elapsed_hours = (time.time() - start) * sim_speed / 3600.0
            data = bearing.generate_data(elapsed_hours)

            if args.json:
                print(json.dumps(data, ensure_ascii=False))
                sys.stdout.flush()
            else:
                print_status(bearing, data, count + 1, args.count if args.count > 0 else 9999)

            values = [
                float(data["temperature"]),
                float(data["radial_load"]),
                float(data["rotational_speed"]),
                float(data["oil_film_thickness"]),
                float(args.bearing_id),
            ]
            send_modbus_tcp(args.host, args.port, bearing.modbus_addr & 0xFF, txn_id, values)
            txn_id = (txn_id + 1) & 0xFFFF

            count += 1
            if args.once:
                break
            if args.count > 0 and count >= args.count:
                break

            time.sleep(interval)

    except KeyboardInterrupt:
        print()
        print(f"[INFO] 用户中止，共发送 {count} 次数据")

    print(f"[DONE] 模拟完成，累计发送 {count} 次，累计磨损 {bearing.wear_accumulated:.3f}μm")
    return 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
"""
古代水转筒车轴承传感器模拟器
通过 Modbus TCP 协议模拟上报轴承温度、径向载荷、转速、润滑油膜厚度数据
"""

import struct
import socket
import time
import random
import math
import json
import argparse
import threading
from dataclasses import dataclass
from datetime import datetime


@dataclass
class BearingSimulator:
    bearing_id: int
    modbus_addr: int
    bearing_code: str
    position: str

    base_temp: float = 35.0
    base_load: float = 5000.0
    base_speed: float = 15.0
    base_film: float = 3.5

    wear_accumulated: float = 0.0
    wear_rate: float = 0.003

    last_time: float = None

    def generate_data(self, elapsed_hours: float) -> dict:
        wear = self.wear_accumulated + self.wear_rate * elapsed_hours
        wear_factor = 1.0 + wear * 0.005

        t = time.time()
        daily_cycle = math.sin(t / 86400 * 2 * math.pi) * 5.0
        temp = self.base_temp + daily_cycle + random.gauss(0, 1.5) + wear * 0.02
        temp = max(15, min(85, temp))

        water_flow = max(0.3, 1.0 + 0.3 * math.sin(t / 3600 * 2 * math.pi))
        load = self.base_load * water_flow * wear_factor + random.gauss(0, 300)
        load = max(500, min(20000, load))

        speed = self.base_speed * math.sqrt(water_flow) * (1.0 - wear * 0.002)
        speed += random.gauss(0, 0.8)
        speed = max(2, min(50, speed))

        ehl_factor = (
            (temp / 40.0) ** -0.5
            * (speed / 15.0) ** 0.7
            * (load / 5000.0) ** -0.3
        )
        film = self.base_film * ehl_factor
        film -= wear * 0.015
        film += random.gauss(0, 0.15)
        film = max(0.1, min(8.0, film))

        if random.random() < 0.005:
            film *= random.uniform(0.1, 0.4)
            temp += random.uniform(5, 15)

        if random.random() < 0.003:
            load *= random.uniform(1.5, 2.5)

        self.wear_accumulated = wear

        return {
            "bearing_id": self.bearing_id,
            "bearing_code": self.bearing_code,
            "position": self.position,
            "temperature": round(temp, 4),
            "radial_load": round(load, 4),
            "rotational_speed": round(speed, 4),
            "oil_film_thickness": round(film, 6),
            "timestamp": datetime.now().isoformat(),
            "wear_accumulated_um": round(wear, 4),
        }


class ModbusTCPClient:
    def __init__(self, host: str, port: int):
        self.host = host
        self.port = port
        self.sock = None
        self.transaction_id = 0

    def connect(self):
        self.sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self.sock.settimeout(5)
        try:
            self.sock.connect((self.host, self.port))
            print(f"[Modbus] 已连接到 {self.host}:{self.port}")
            return True
        except Exception as e:
            print(f"[Modbus] 连接失败: {e}")
            return False

    def close(self):
        if self.sock:
            self.sock.close()
            self.sock = None

    def _build_mbap(self, length: int, unit_id: int = 1) -> bytes:
        self.transaction_id = (self.transaction_id + 1) % 65536
        header = struct.pack(">HHHB", self.transaction_id, 0, length, unit_id)
        return header

    def write_multiple_registers(
        self,
        start_addr: int,
        values: list,
        unit_id: int = 1,
    ) -> bool:
        float_bytes = b""
        for v in values:
            float_bytes += struct.pack(">f", float(v))

        num_regs = len(float_bytes) // 2
        byte_count = len(float_bytes)

        pdu = struct.pack(
            ">BHHB",
            0x10,
            start_addr,
            num_regs,
            byte_count,
        ) + float_bytes

        mbap = self._build_mbap(len(pdu) + 1, unit_id)
        request = mbap + pdu

        try:
            self.sock.sendall(request)
            response = self.sock.recv(256)
            if len(response) >= 12:
                return True
            return False
        except Exception as e:
            print(f"[Modbus] 写入失败: {e}")
            return False


class SensorSimulator:
    def __init__(self, config: dict):
        self.host = config.get("modbus_host", "localhost")
        self.port = config.get("modbus_port", 5020)
        self.interval = config.get("interval_seconds", 60)
        self.api_url = config.get("api_url", "http://localhost:8080")
        self.use_modbus = config.get("use_modbus", True)
        self.use_api = config.get("use_api", False)
        self.verbose = config.get("verbose", True)

        self.bearings = []
        self.modbus_client = ModbusTCPClient(self.host, self.port)
        self.running = False
        self.start_time = time.time()

        for i, b in enumerate(config.get("bearings", [])):
            sim = BearingSimulator(
                bearing_id=b.get("id", i + 1),
                modbus_addr=b.get("modbus_addr", i * 10),
                bearing_code=b.get("code", f"BR-{i+1:03d}"),
                position=b.get("position", f"位置{i+1}"),
                base_temp=b.get("base_temp", 35.0),
                base_load=b.get("base_load", 5000.0),
                base_speed=b.get("base_speed", 15.0),
                base_film=b.get("base_film", 3.5),
                wear_rate=b.get("wear_rate", 0.003),
            )
            self.bearings.append(sim)

    def _send_via_modbus(self, bearing: BearingSimulator, data: dict) -> bool:
        if self.modbus_client.sock is None:
            if not self.modbus_client.connect():
                time.sleep(5)
                return False

        values = [
            data["temperature"],
            data["radial_load"],
            data["rotational_speed"],
            data["oil_film_thickness"],
        ]

        success = self.modbus_client.write_multiple_registers(
            bearing.modbus_addr, values
        )
        if not success:
            self.modbus_client.close()
        return success

    def _send_via_api(self, data: dict) -> bool:
        try:
            import urllib.request

            payload = {
                "time": data["timestamp"],
                "bearing_id": data["bearing_id"],
                "temperature": data["temperature"],
                "radial_load": data["radial_load"],
                "rotational_speed": data["rotational_speed"],
                "oil_film_thickness": data["oil_film_thickness"],
                "source": "simulator",
            }
            req = urllib.request.Request(
                f"{self.api_url}/api/v1/sensor-data",
                data=json.dumps(payload).encode("utf-8"),
                headers={"Content-Type": "application/json"},
                method="POST",
            )
            with urllib.request.urlopen(req, timeout=5) as resp:
                return resp.status == 201
        except Exception as e:
            print(f"[API] 发送失败: {e}")
            return False

    def _single_cycle(self):
        elapsed = (time.time() - self.start_time) / 3600.0

        for bearing in self.bearings:
            data = bearing.generate_data(elapsed)

            modbus_ok = False
            api_ok = False

            if self.use_modbus:
                modbus_ok = self._send_via_modbus(bearing, data)

            if self.use_api:
                api_ok = self._send_via_api(data)

            if self.verbose:
                status_parts = []
                if self.use_modbus:
                    status_parts.append(f"Modbus={'✓' if modbus_ok else '✗'}")
                if self.use_api:
                    status_parts.append(f"API={'✓' if api_ok else '✗'}")
                status = " | ".join(status_parts)

                print(
                    f"[{datetime.now().strftime('%H:%M:%S')}] "
                    f"{bearing.bearing_code} ({bearing.position}) "
                    f"温度={data['temperature']:.2f}°C "
                    f"载荷={data['radial_load']:.1f}N "
                    f"转速={data['rotational_speed']:.2f}RPM "
                    f"油膜={data['oil_film_thickness']:.4f}μm "
                    f"磨损={data['wear_accumulated_um']:.3f}μm "
                    f"[{status}]"
                )

    def run(self):
        self.running = True
        print("=" * 80)
        print("  古代水转筒车轴承传感器模拟器")
        print(f"  Modbus服务器: {self.host}:{self.port}")
        print(f"  API服务器: {self.api_url}")
        print(f"  上报间隔: {self.interval}秒")
        print(f"  模拟轴承数量: {len(self.bearings)}")
        print("=" * 80)

        for b in self.bearings:
            print(f"    - {b.bearing_code}: {b.position}, Modbus地址={b.modbus_addr}")
        print()

        try:
            while self.running:
                cycle_start = time.time()
                self._single_cycle()
                elapsed = time.time() - cycle_start
                sleep_time = max(0, self.interval - elapsed)
                time.sleep(sleep_time)
        except KeyboardInterrupt:
            print("\n\n模拟器已停止")
        finally:
            self.running = False
            self.modbus_client.close()


DEFAULT_CONFIG = {
    "modbus_host": "localhost",
    "modbus_port": 5020,
    "api_url": "http://localhost:8080",
    "interval_seconds": 60,
    "use_modbus": True,
    "use_api": False,
    "verbose": True,
    "bearings": [
        {
            "id": 1,
            "modbus_addr": 0,
            "code": "NRW-001-BR-A",
            "position": "主轴上轴承",
            "base_temp": 36.0,
            "base_load": 6500.0,
            "base_speed": 12.0,
            "base_film": 3.2,
            "wear_rate": 0.004,
        },
        {
            "id": 2,
            "modbus_addr": 10,
            "code": "NRW-001-BR-B",
            "position": "主轴下轴承",
            "base_temp": 38.0,
            "base_load": 7200.0,
            "base_speed": 12.0,
            "base_film": 2.8,
            "wear_rate": 0.005,
        },
        {
            "id": 3,
            "modbus_addr": 20,
            "code": "NRW-002-BR-A",
            "position": "主轴轴承",
            "base_temp": 33.0,
            "base_load": 4800.0,
            "base_speed": 18.0,
            "base_film": 4.0,
            "wear_rate": 0.0025,
        },
    ],
}


def main():
    parser = argparse.ArgumentParser(
        description="古代水转筒车轴承传感器模拟器 (Modbus TCP)"
    )
    parser.add_argument(
        "--modbus-host", default="localhost", help="Modbus TCP服务器地址"
    )
    parser.add_argument(
        "--modbus-port", type=int, default=5020, help="Modbus TCP服务器端口"
    )
    parser.add_argument(
        "--api-url", default="http://localhost:8080", help="后端API地址"
    )
    parser.add_argument(
        "--interval", type=int, default=60, help="上报间隔秒数（默认60秒）"
    )
    parser.add_argument(
        "--fast", action="store_true", help="快速模式，间隔1秒（用于测试）"
    )
    parser.add_argument(
        "--use-api", action="store_true", help="同时通过REST API发送数据"
    )
    parser.add_argument(
        "--no-modbus", action="store_true", help="禁用Modbus TCP（仅使用API）"
    )
    parser.add_argument(
        "--config", type=str, help="JSON配置文件路径"
    )
    parser.add_argument(
        "--quiet", action="store_true", help="静默模式，减少输出"
    )

    args = parser.parse_args()

    config = dict(DEFAULT_CONFIG)

    if args.config:
        try:
            with open(args.config, "r", encoding="utf-8") as f:
                user_config = json.load(f)
                config.update(user_config)
        except Exception as e:
            print(f"加载配置文件失败: {e}")

    config["modbus_host"] = args.modbus_host
    config["modbus_port"] = args.modbus_port
    config["api_url"] = args.api_url
    config["interval_seconds"] = 1 if args.fast else args.interval
    config["use_modbus"] = not args.no_modbus
    config["use_api"] = args.use_api
    config["verbose"] = not args.quiet

    if not config["use_modbus"] and not config["use_api"]:
        print("错误: 必须启用至少一种数据发送方式 (Modbus或API)")
        return

    sim = SensorSimulator(config)
    sim.run()


if __name__ == "__main__":
    main()

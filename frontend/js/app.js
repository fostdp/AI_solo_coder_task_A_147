const App = {
    state: {
        noriaWheels: [],
        bearings: [],
        selectedNoria: null,
        selectedBearing: null,
        bearingStatuses: [],
    },

    init() {
        this._setupNavigation();
        this._initCharts();
        this._initBearingPanel();
        this._initOilFilmView();
        this._init3DView();
        this._setupEventListeners();
        this._loadNoriaWheels();
        this._startClock();
        this._startDataRefresh();
    },

    _setupNavigation() {
        document.querySelectorAll(".nav-btn").forEach((btn) => {
            btn.addEventListener("click", () => {
                const view = btn.dataset.view;
                this._switchView(view);
            });
        });
    },

    _switchView(viewName) {
        document.querySelectorAll(".nav-btn").forEach((b) => {
            b.classList.toggle("active", b.dataset.view === viewName);
        });

        document.querySelectorAll(".view-section").forEach((s) => {
            s.classList.toggle("active", s.id === `view-${viewName}`);
        });

        if (viewName === "3d") {
            setTimeout(() => {
                if (Waterwheel3D.camera && Waterwheel3D.renderer) {
                    const container = document.getElementById("three-scene");
                    Waterwheel3D._onResize(container);
                }
            }, 100);
        }

        if (viewName === "bearing" && this.state.selectedBearing) {
            this._loadBearingDetail(this.state.selectedBearing);
        }

        if (viewName === "oilfilm" && this.state.selectedBearing) {
            this._loadOilFilmData(this.state.selectedBearing);
        }

        if (viewName === "prediction" && this.state.selectedBearing) {
            this._loadLifePrediction(this.state.selectedBearing);
        }
    },

    _initCharts() {
        Charts.initRealtime("realtime-chart");
        Charts.initWearHistory("wear-history-chart");
        Charts.initReliability("reliability-chart");
        Charts.initPDF("pdf-chart");
        Charts.initWearTrend("weartrend-chart");
    },

    _initBearingPanel() {
        BearingPanel.init("bearing-canvas");
    },

    _initOilFilmView() {
        OilFilmView.init("oilfilm-canvas");
    },

    _init3DView() {
        setTimeout(() => {
            Waterwheel3D.init("three-scene", {
                diameter: 8.5,
                onBearingClick: (bearing) => {
                    this._selectBearing(bearing);
                    this._switchView("bearing");
                },
                onBearingHover: (bearing) => {
                    const info = document.getElementById("three-info");
                    if (info) {
                        const status = this.state.bearingStatuses.find(
                            (s) => s.bearing_id === bearing.id
                        );
                        const health = status?.health_status || "unknown";
                        info.innerHTML = `<strong>${bearing.bearing_code}</strong> - ${bearing.position} [${health}]`;
                    }
                },
            });
        }, 500);
    },

    _setupEventListeners() {
        document.getElementById("noria-select").addEventListener("change", (e) => {
            const id = parseInt(e.target.value);
            if (id) {
                this._selectNoria(id);
            }
        });

        document.getElementById("bearing-select-profile").addEventListener("change", (e) => {
            const id = parseInt(e.target.value);
            const bearing = this.state.bearings.find((b) => b.id === id);
            if (bearing) this._selectBearing(bearing);
        });

        document.getElementById("bearing-select-film").addEventListener("change", (e) => {
            const id = parseInt(e.target.value);
            const bearing = this.state.bearings.find((b) => b.id === id);
            if (bearing) {
                this.state.selectedBearing = bearing;
                this._loadOilFilmData(bearing);
            }
        });

        document.getElementById("bearing-select-pred").addEventListener("change", (e) => {
            const id = parseInt(e.target.value);
            const bearing = this.state.bearings.find((b) => b.id === id);
            if (bearing) {
                this.state.selectedBearing = bearing;
                this._loadLifePrediction(bearing);
            }
        });

        document.getElementById("show-wear").addEventListener("change", (e) => {
            BearingPanel.setOption("showWear", e.target.checked);
        });

        document.getElementById("show-load").addEventListener("change", (e) => {
            BearingPanel.setOption("showLoad", e.target.checked);
        });

        document.getElementById("calc-now").addEventListener("click", async () => {
            if (!this.state.selectedBearing) return;
            try {
                await API.triggerCalculation(this.state.selectedBearing.id);
                this._loadBearingDetail(this.state.selectedBearing);
                this._refreshOverview();
            } catch (err) {
                console.error("触发计算失败:", err);
            }
        });

        document.getElementById("auto-rotate").addEventListener("change", (e) => {
            Waterwheel3D.setAutoRotate(e.target.checked);
        });

        document.getElementById("speed-factor").addEventListener("input", (e) => {
            const val = parseFloat(e.target.value);
            Waterwheel3D.setRotationSpeed(val);
            document.getElementById("speed-value").textContent = `${val.toFixed(1)}x`;
        });

        document.getElementById("reset-view").addEventListener("click", () => {
            Waterwheel3D.resetView();
        });

        document.getElementById("film-view-type").addEventListener("change", (e) => {
            OilFilmView.setViewType(e.target.value);
        });

        document.getElementById("refresh-film").addEventListener("click", () => {
            if (this.state.selectedBearing) {
                this._loadOilFilmData(this.state.selectedBearing);
            }
        });

        document.getElementById("refresh-pred").addEventListener("click", () => {
            if (this.state.selectedBearing) {
                this._loadLifePrediction(this.state.selectedBearing);
            }
        });

        document.getElementById("alert-filter").addEventListener("change", () => this._loadAlerts());
        document.getElementById("alert-limit").addEventListener("change", () => this._loadAlerts());
        document.getElementById("refresh-alerts").addEventListener("click", () => this._loadAlerts());
    },

    async _loadNoriaWheels() {
        try {
            const wheels = await API.getNoriaWheels();
            this.state.noriaWheels = wheels;

            const select = document.getElementById("noria-select");
            select.innerHTML = wheels
                .map((w) => `<option value="${w.id}">${w.name}</option>`)
                .join("");

            if (wheels.length > 0) {
                this._selectNoria(wheels[0].id);
            }
        } catch (err) {
            console.error("加载筒车列表失败:", err);
            document.getElementById("noria-select").innerHTML =
                '<option value="">加载失败</option>';
        }
    },

    async _selectNoria(noriaId) {
        this.state.selectedNoria = this.state.noriaWheels.find((w) => w.id === noriaId);
        this._updateNoriaInfo();

        try {
            const bearings = await API.getBearings(noriaId);
            this.state.bearings = bearings;
            this._updateBearingsList();
            this._updateBearingSelects();

            if (bearings.length > 0) {
                this._selectBearing(bearings[0]);
            }

            Waterwheel3D.setBearings(bearings);
        } catch (err) {
            console.error("加载轴承列表失败:", err);
        }

        this._refreshOverview();
    },

    _updateNoriaInfo() {
        const info = document.getElementById("noria-info");
        const w = this.state.selectedNoria;
        if (!w) {
            info.innerHTML = '<p class="info-label">选择筒车查看详情</p>';
            return;
        }

        info.innerHTML = `
            <p><strong>${w.name}</strong></p>
            <p>位置: ${w.location || "-"}</p>
            <p>直径: ${w.diameter} 米 | 水斗: ${w.buckets} 个</p>
            <p style="color: #5a7296; font-size: 12px; margin-top: 8px;">${w.description || ""}</p>
        `;
    },

    _updateBearingsList() {
        const list = document.getElementById("bearings-list");

        const items = this.state.bearings.map((b) => {
            const status = this.state.bearingStatuses.find((s) => s.bearing_id === b.id);
            const health = status?.health_status || "unknown";
            const wear = status?.total_wear_microm;
            const temp = status?.temperature;
            const film = status?.oil_film_thickness;
            const isSelected = this.state.selectedBearing?.id === b.id;

            return `
                <div class="bearing-item ${isSelected ? "selected" : ""}" data-bearing-id="${b.id}">
                    <div class="bearing-item-header">
                        <span class="bearing-code">${b.bearing_code}</span>
                        <span class="health-badge ${health}">${this._translateHealth(health)}</span>
                    </div>
                    <div class="bearing-position">${b.position}</div>
                    <div class="bearing-metrics">
                        <div class="metric">
                            <span class="metric-label">温度</span>
                            <span class="metric-value">${temp ? temp.toFixed(1) + "°C" : "--"}</span>
                        </div>
                        <div class="metric">
                            <span class="metric-label">磨损</span>
                            <span class="metric-value">${wear ? wear.toFixed(2) + "μm" : "--"}</span>
                        </div>
                        <div class="metric">
                            <span class="metric-label">油膜</span>
                            <span class="metric-value">${film ? film.toFixed(2) + "μm" : "--"}</span>
                        </div>
                        <div class="metric">
                            <span class="metric-label">剩余</span>
                            <span class="metric-value">${status?.predicted_rul_hours ? status.predicted_rul_hours.toFixed(0) + "h" : "--"}</span>
                        </div>
                    </div>
                </div>
            `;
        });

        list.innerHTML = items.join("");

        list.querySelectorAll(".bearing-item").forEach((item) => {
            item.addEventListener("click", () => {
                const id = parseInt(item.dataset.bearingId);
                const bearing = this.state.bearings.find((b) => b.id === id);
                if (bearing) this._selectBearing(bearing);
            });
        });
    },

    _updateBearingSelects() {
        const options = this.state.bearings
            .map((b) => `<option value="${b.id}">${b.bearing_code} - ${b.position}</option>`)
            .join("");

        ["bearing-select-profile", "bearing-select-film", "bearing-select-pred"].forEach((id) => {
            const sel = document.getElementById(id);
            if (sel) sel.innerHTML = options;
        });
    },

    _selectBearing(bearing) {
        this.state.selectedBearing = bearing;

        ["bearing-select-profile", "bearing-select-film", "bearing-select-pred"].forEach((id) => {
            const sel = document.getElementById(id);
            if (sel) sel.value = bearing.id;
        });

        this._updateBearingsList();
        this._loadBearingDetail(bearing);
    },

    async _loadBearingDetail(bearing) {
        BearingPanel.setBearing(bearing);

        const infoRows = document.getElementById("bearing-info-rows");
        infoRows.innerHTML = `
            <div class="info-row"><span class="info-row-label">轴承编号</span><span class="info-row-value">${bearing.bearing_code}</span></div>
            <div class="info-row"><span class="info-row-label">安装位置</span><span class="info-row-value">${bearing.position}</span></div>
            <div class="info-row"><span class="info-row-label">轴承类型</span><span class="info-row-value">${bearing.bearing_type}</span></div>
            <div class="info-row"><span class="info-row-label">材质</span><span class="info-row-value">${bearing.material}</span></div>
            <div class="info-row"><span class="info-row-label">内径/外径/宽度</span><span class="info-row-value">${bearing.inner_diameter}/${bearing.outer_diameter}/${bearing.width} mm</span></div>
            <div class="info-row"><span class="info-row-label">硬度</span><span class="info-row-value">${bearing.hardness_hv} HV</span></div>
            <div class="info-row"><span class="info-row-label">额定寿命</span><span class="info-row-value">${bearing.rated_life_hours.toLocaleString()} h</span></div>
            <div class="info-row"><span class="info-row-label">磨损阈值</span><span class="info-row-value">${bearing.wear_limit_microm} μm</span></div>
            <div class="info-row"><span class="info-row-label">润滑方式</span><span class="info-row-value">${bearing.lubrication_type}</span></div>
            <div class="info-row"><span class="info-row-label">润滑油粘度</span><span class="info-row-value">${bearing.oil_viscosity_pas} Pa·s</span></div>
        `;

        try {
            const [wear, sensor, history] = await Promise.all([
                API.getLatestWearResult(bearing.id).catch(() => null),
                API.getLatestSensorData(bearing.id).catch(() => null),
                API.getWearHistory(bearing.id).catch(() => []),
            ]);

            BearingPanel.setWearResult(wear);
            BearingPanel.setSensorData(sensor);

            if (wear) {
                const ratio = Math.min(100, (wear.total_wear_microm / bearing.wear_limit_microm) * 100);
                document.getElementById("wear-progress-text").textContent =
                    `${wear.total_wear_microm.toFixed(2)} / ${bearing.wear_limit_microm} μm`;
                document.getElementById("wear-progress-fill").style.width = `${ratio}%`;
            }

            if (history && history.length > 0) {
                Charts.updateWearHistory(history);
            }

            if (sensor) {
                this._updateLiveData(sensor, wear, bearing);
            }
        } catch (err) {
            console.error("加载轴承详情失败:", err);
        }
    },

    async _loadOilFilmData(bearing) {
        OilFilmView.setBearing(bearing);

        try {
            const data = await API.getOilFilmMap(bearing.id);
            OilFilmView.setFilmData(data);
            OilFilmView.renderLegend("colormap-legend");
            this._updateOilFilmStats();
        } catch (err) {
            console.error("加载油膜数据失败:", err);
        }
    },

    _updateOilFilmStats() {
        const stats = OilFilmView.getStats();
        const lub = OilFilmView.getLubricationStatus();

        const statsEl = document.getElementById("oilfilm-stats");
        if (stats) {
            statsEl.innerHTML = `
                <div class="oilfilm-stat"><span class="oilfilm-stat-label">最小值</span><span class="oilfilm-stat-value">${stats.min} μm</span></div>
                <div class="oilfilm-stat"><span class="oilfilm-stat-label">最大值</span><span class="oilfilm-stat-value">${stats.max} μm</span></div>
                <div class="oilfilm-stat"><span class="oilfilm-stat-label">平均值</span><span class="oilfilm-stat-value">${stats.avg} μm</span></div>
                <div class="oilfilm-stat"><span class="oilfilm-stat-label">标准差</span><span class="oilfilm-stat-value">${stats.std} μm</span></div>
                <div class="oilfilm-stat"><span class="oilfilm-stat-label">低于阈值(0.5μm)</span><span class="oilfilm-stat-value">${stats.belowThreshold}%</span></div>
            `;
        }

        const lubEl = document.getElementById("lubrication-status");
        if (lub) {
            lubEl.className = `lubrication-status ${lub.level}`;
            lubEl.innerHTML = `
                <div class="lubrication-title">${lub.title}</div>
                <div class="lubrication-desc">${lub.desc}</div>
            `;
        }
    },

    async _loadLifePrediction(bearing) {
        try {
            const pred = await API.getLatestLifePrediction(bearing.id);
            const history = await API.getWearHistory(bearing.id, 100);

            if (pred) {
                document.getElementById("pred-rul").textContent = `${pred.predicted_rul_hours.toFixed(0)} 小时`;
                document.getElementById("pred-reliability").textContent = `${((pred.reliability || 0) * 100).toFixed(2)}%`;
                document.getElementById("pred-failure").textContent = `${((pred.failure_probability || 0) * 100).toFixed(2)}%`;
                document.getElementById("pred-running").textContent = `${pred.running_hours.toFixed(0)} 小时`;
                document.getElementById("weibull-shape").textContent = pred.weibull_shape.toFixed(4);
                document.getElementById("weibull-scale").textContent = pred.weibull_scale.toFixed(0);
                document.getElementById("fatigue-damage").textContent = `${((pred.fatigue_damage || 0) * 100).toFixed(2)}%`;

                Charts.updateReliability(
                    pred.weibull_shape,
                    pred.weibull_scale,
                    pred.running_hours,
                    pred.predicted_rul_hours
                );
                Charts.updatePDF(pred.weibull_shape, pred.weibull_scale, pred.running_hours);
            }

            if (history && history.length > 0) {
                Charts.updateWearTrend(history);
            }
        } catch (err) {
            console.error("加载寿命预测失败:", err);
        }
    },

    _updateLiveData(sensor, wear, bearing) {
        const format = (v, unit, decimals = 2) =>
            v != null ? `${v.toFixed(decimals)}` : "--";

        document.getElementById("live-temp").textContent = format(sensor?.temperature, null, 1);
        document.getElementById("live-load").textContent = format(sensor?.radial_load, null, 0);
        document.getElementById("live-speed").textContent = format(sensor?.rotational_speed, null, 1);
        document.getElementById("live-film").textContent = format(sensor?.oil_film_thickness, null, 3);
        document.getElementById("live-wear").textContent = format(wear?.total_wear_microm, null, 2);
        document.getElementById("live-rul").textContent = "--";

        if (sensor) {
            Charts.updateRealtime(sensor);
        }

        if (Waterwheel3D && bearing) {
            Waterwheel3D.setWheelRPM(sensor?.rotational_speed || 15);
        }
    },

    async _refreshOverview() {
        try {
            const [statuses, alerts] = await Promise.all([
                API.getBearingStatuses(),
                API.getRecentAlerts(10),
            ]);

            this.state.bearingStatuses = statuses;
            document.getElementById("stat-bearings").textContent = statuses.length;

            const criticalCount = alerts.filter((a) => a.alert_level === "critical").length;
            document.getElementById("stat-alerts").textContent = criticalCount;

            this._updateBearingsList();
            this._renderRecentAlerts(alerts);

            statuses.forEach((s) => {
                Waterwheel3D.updateBearingHealth(s.bearing_id, s.health_status);
            });

            if (this.state.selectedBearing) {
                const status = statuses.find((s) => s.bearing_id === this.state.selectedBearing.id);
                if (status?.last_data_time) {
                    try {
                        const sensor = await API.getLatestSensorData(this.state.selectedBearing.id);
                        const wear = await API.getLatestWearResult(this.state.selectedBearing.id);
                        this._updateLiveData(sensor, wear, this.state.selectedBearing);
                    } catch {}
                }
            }
        } catch (err) {
            console.error("刷新总览失败:", err);
        }
    },

    _renderRecentAlerts(alerts) {
        const list = document.getElementById("recent-alerts");

        if (!alerts || alerts.length === 0) {
            list.innerHTML = '<p class="loading-text">暂无告警记录</p>';
            return;
        }

        list.innerHTML = alerts
            .slice(0, 10)
            .map((a) => {
                const time = new Date(a.alert_time).toLocaleString("zh-CN", {
                    month: "2-digit",
                    day: "2-digit",
                    hour: "2-digit",
                    minute: "2-digit",
                });

                return `
                    <div class="alert-item ${a.alert_level}">
                        <div class="alert-item-header">
                            <span class="alert-level ${a.alert_level}">${this._translateAlertLevel(a.alert_level)}</span>
                            <span class="alert-time">${time}</span>
                        </div>
                        <div class="alert-type">${this._translateAlertType(a.alert_type)}</div>
                        <div class="alert-message">${a.alert_message}</div>
                    </div>
                `;
            })
            .join("");
    },

    async _loadAlerts() {
        const filter = document.getElementById("alert-filter").value;
        const limit = parseInt(document.getElementById("alert-limit").value);

        try {
            const alerts = await API.getRecentAlerts(limit);

            const filtered = filter === "all" ? alerts : alerts.filter((a) => a.alert_level === filter);

            const tbody = document.getElementById("alerts-table-body");

            if (filtered.length === 0) {
                tbody.innerHTML = '<tr><td colspan="8" class="loading-text">暂无告警记录</td></tr>';
                return;
            }

            tbody.innerHTML = filtered
                .map((a) => {
                    const time = new Date(a.alert_time).toLocaleString("zh-CN");
                    const bearing = this.state.bearings.find((b) => b.id === a.bearing_id);
                    const statusClass = a.resolved ? "resolved" : a.acknowledged ? "acknowledged" : "pending";
                    const statusText = a.resolved ? "已解决" : a.acknowledged ? "已确认" : "待处理";

                    return `
                        <tr>
                            <td>${time}</td>
                            <td>${bearing?.bearing_code || `#${a.bearing_id}`}</td>
                            <td>${this._translateAlertType(a.alert_type)}</td>
                            <td><span class="alert-badge ${a.alert_level}">${this._translateAlertLevel(a.alert_level)}</span></td>
                            <td>${a.alert_message}</td>
                            <td>${a.threshold_value != null ? a.threshold_value.toFixed(3) : "-"}</td>
                            <td>${a.actual_value != null ? a.actual_value.toFixed(3) : "-"}</td>
                            <td>${statusText}</td>
                        </tr>
                    `;
                })
                .join("");
        } catch (err) {
            console.error("加载告警失败:", err);
        }
    },

    _translateHealth(status) {
        const map = { normal: "正常", warning: "警告", critical: "严重", unknown: "未知" };
        return map[status] || status;
    },

    _translateAlertLevel(level) {
        const map = { warning: "警告", critical: "严重", info: "信息" };
        return map[level] || level;
    },

    _translateAlertType(type) {
        const map = {
            wear_warning: "磨损预警",
            wear_exceeded: "磨损超限",
            oil_film_rupture: "油膜破裂",
            temp_high: "温度过高",
        };
        return map[type] || type;
    },

    _startClock() {
        const update = () => {
            const el = document.getElementById("stat-time");
            if (el) {
                el.textContent = new Date().toLocaleTimeString("zh-CN");
            }
        };
        update();
        setInterval(update, 1000);
    },

    _startDataRefresh() {
        setInterval(() => this._refreshOverview(), 15000);
        setInterval(() => this._loadAlerts(), 30000);
    },
};

document.addEventListener("DOMContentLoaded", () => App.init());

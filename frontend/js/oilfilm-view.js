const OilFilmView = {
    canvas: null,
    ctx: null,
    bearing: null,
    filmData: null,
    gridSizeX: 64,
    gridSizeY: 32,
    viewType: "circular",
    minValue: 0,
    maxValue: 5,
    animating: false,
    time: 0,

    init(canvasId) {
        this.canvas = document.getElementById(canvasId);
        if (!this.canvas) {
            console.error(`未找到Canvas ${canvasId}`);
            return;
        }

        this.ctx = this.canvas.getContext("2d");
        this._resizeCanvas();
        window.addEventListener("resize", () => this._resizeCanvas());

        this.animate();
    },

    _resizeCanvas() {
        if (!this.canvas) return;

        const rect = this.canvas.getBoundingClientRect();
        const dpr = window.devicePixelRatio || 1;

        this.canvas.width = rect.width * dpr;
        this.canvas.height = rect.height * dpr;
        this.ctx.scale(dpr, dpr);

        this._width = rect.width;
        this._height = rect.height;
    },

    setBearing(bearing) {
        this.bearing = bearing;
    },

    setFilmData(data) {
        if (!data) return;

        this.gridSizeX = data.grid_size_x || 64;
        this.gridSizeY = data.grid_size_y || 32;
        this.filmData = data.data || [];

        let min = Infinity;
        let max = -Infinity;
        for (let y = 0; y < this.filmData.length; y++) {
            for (let x = 0; x < this.filmData[y].length; x++) {
                const v = this.filmData[y][x];
                if (v < min) min = v;
                if (v > max) max = v;
            }
        }
        this.minValue = Math.max(0, min - 0.5);
        this.maxValue = max + 0.5;
    },

    setViewType(type) {
        this.viewType = type;
    },

    getStats() {
        if (!this.filmData) return null;

        let min = Infinity;
        let max = -Infinity;
        let sum = 0;
        let count = 0;

        for (let y = 0; y < this.filmData.length; y++) {
            for (let x = 0; x < this.filmData[y].length; x++) {
                const v = this.filmData[y][x];
                min = Math.min(min, v);
                max = Math.max(max, v);
                sum += v;
                count++;
            }
        }

        const avg = sum / count;

        let variance = 0;
        for (let y = 0; y < this.filmData.length; y++) {
            for (let x = 0; x < this.filmData[y].length; x++) {
                variance += Math.pow(this.filmData[y][x] - avg, 2);
            }
        }
        const std = Math.sqrt(variance / count);

        let belowThreshold = 0;
        const threshold = 0.5;
        for (let y = 0; y < this.filmData.length; y++) {
            for (let x = 0; x < this.filmData[y].length; x++) {
                if (this.filmData[y][x] < threshold) {
                    belowThreshold++;
                }
            }
        }
        const belowPercent = (belowThreshold / count) * 100;

        return {
            min: min.toFixed(4),
            max: max.toFixed(4),
            avg: avg.toFixed(4),
            std: std.toFixed(4),
            belowThreshold: belowPercent.toFixed(2),
        };
    },

    getLubricationStatus() {
        const stats = this.getStats();
        if (!stats) return { level: "unknown", title: "无数据", desc: "暂无油膜数据" };

        const avg = parseFloat(stats.avg);
        const below = parseFloat(stats.belowThreshold);

        if (avg < 0.5 || below > 20) {
            return {
                level: "critical",
                title: "边界润滑/干摩擦",
                desc: "油膜厚度严重不足，存在干摩擦风险，磨损将急剧加速，建议立即停机检查润滑系统。",
            };
        } else if (avg < 1.0 || below > 5) {
            return {
                level: "warning",
                title: "混合润滑",
                desc: "油膜厚度偏低，部分区域处于混合润滑状态，存在金属接触风险，建议关注磨损趋势。",
            };
        } else if (avg < 2.0) {
            return {
                level: "normal",
                title: "部分弹流润滑",
                desc: "油膜厚度适中，处于部分弹流润滑状态，轴承运行正常，建议定期检查。",
            };
        } else {
            return {
                level: "good",
                title: "全膜弹流润滑",
                desc: "油膜厚度充足，处于完全弹流润滑状态，金属表面完全被油膜隔离，润滑状态优良。",
            };
        }
    },

    render() {
        if (!this.ctx || !this._width || !this._height) return;

        const ctx = this.ctx;
        const W = this._width;
        const H = this._height;

        ctx.clearRect(0, 0, W, H);
        this._drawBackground(ctx, W, H);

        if (!this.filmData || this.filmData.length === 0) {
            this._drawPlaceholder(ctx, W, H);
            return;
        }

        switch (this.viewType) {
            case "circular":
                this._drawCircularView(ctx, W, H);
                break;
            case "rectangular":
                this._drawRectangularView(ctx, W, H);
                break;
            case "3d":
                this._draw3DView(ctx, W, H);
                break;
            default:
                this._drawCircularView(ctx, W, H);
        }
    },

    _drawBackground(ctx, W, H) {
        const gradient = ctx.createRadialGradient(W / 2, H / 2, 0, W / 2, H / 2, Math.max(W, H));
        gradient.addColorStop(0, "#1a2d47");
        gradient.addColorStop(1, "#0a1628");
        ctx.fillStyle = gradient;
        ctx.fillRect(0, 0, W, H);
    },

    _drawPlaceholder(ctx, W, H) {
        ctx.fillStyle = "rgba(143, 164, 194, 0.5)";
        ctx.font = "18px -apple-system, 'PingFang SC', sans-serif";
        ctx.textAlign = "center";
        ctx.textBaseline = "middle";
        ctx.fillText("请选择轴承并加载油膜数据", W / 2, H / 2);
    },

    _drawCircularView(ctx, W, H) {
        const cx = W / 2;
        const cy = H / 2;
        const maxRadius = Math.min(W, H) / 2 - 80;
        const innerRadius = maxRadius * 0.45;
        const outerRadius = maxRadius;

        const radialSteps = this.gridSizeY;
        const angularSteps = this.gridSizeX;

        this.time += 0.005;

        for (let r = 0; r < radialSteps - 1; r++) {
            for (let a = 0; a < angularSteps; a++) {
                const rRatio1 = r / (radialSteps - 1);
                const rRatio2 = (r + 1) / (radialSteps - 1);
                const radius1 = innerRadius + (outerRadius - innerRadius) * rRatio1;
                const radius2 = innerRadius + (outerRadius - innerRadius) * rRatio2;

                const a1 = (a / angularSteps) * Math.PI * 2;
                const a2 = ((a + 1) / angularSteps) * Math.PI * 2;

                const filmValue = this.filmData[r]?.[a] || 0;
                const color = ColorMap.jet(filmValue, this.minValue, this.maxValue);

                ctx.fillStyle = ColorMap.toRgb(color);
                ctx.beginPath();

                const wobble = Math.sin(this.time + r * 0.3 + a * 0.1) * 0.5;

                ctx.moveTo(
                    cx + Math.cos(a1) * (radius1 + wobble),
                    cy + Math.sin(a1) * (radius1 + wobble)
                );
                ctx.arc(cx, cy, radius1 + wobble, a1, a2);
                ctx.arc(cx, cy, radius2 + wobble, a2, a1, true);
                ctx.closePath();
                ctx.fill();
            }
        }

        ctx.strokeStyle = "rgba(255, 255, 255, 0.3)";
        ctx.lineWidth = 2;
        ctx.beginPath();
        ctx.arc(cx, cy, innerRadius, 0, Math.PI * 2);
        ctx.stroke();

        ctx.beginPath();
        ctx.arc(cx, cy, outerRadius, 0, Math.PI * 2);
        ctx.stroke();

        ctx.fillStyle = "#1a2d47";
        ctx.beginPath();
        ctx.arc(cx, cy, innerRadius - 5, 0, Math.PI * 2);
        ctx.fill();

        ctx.fillStyle = "#4fc3f7";
        ctx.font = "bold 14px -apple-system, 'PingFang SC', sans-serif";
        ctx.textAlign = "center";
        ctx.textBaseline = "middle";
        ctx.fillText("轴承剖面", cx, cy);

        if (this.bearing) {
            ctx.font = "11px 'SF Mono', Consolas, monospace";
            ctx.fillStyle = "#8fa4c2";
            ctx.fillText(this.bearing.bearing_code, cx, cy + 20);
        }

        for (let i = 0; i < 8; i++) {
            const angle = (i / 8) * Math.PI * 2;
            ctx.strokeStyle = "rgba(255, 255, 255, 0.15)";
            ctx.lineWidth = 1;
            ctx.beginPath();
            ctx.moveTo(cx + Math.cos(angle) * innerRadius, cy + Math.sin(angle) * innerRadius);
            ctx.lineTo(cx + Math.cos(angle) * outerRadius, cy + Math.sin(angle) * outerRadius);
            ctx.stroke();
        }

        for (let i = 1; i <= 3; i++) {
            const r = innerRadius + (outerRadius - innerRadius) * (i / 4);
            ctx.strokeStyle = "rgba(255, 255, 255, 0.1)";
            ctx.beginPath();
            ctx.arc(cx, cy, r, 0, Math.PI * 2);
            ctx.stroke();
        }
    },

    _drawRectangularView(ctx, W, H) {
        const padding = 60;
        const viewW = W - padding * 2;
        const viewH = H - padding * 2;

        const cellW = viewW / this.gridSizeX;
        const cellH = viewH / this.gridSizeY;

        this.time += 0.003;

        for (let y = 0; y < this.gridSizeY; y++) {
            for (let x = 0; x < this.gridSizeX; x++) {
                const value = this.filmData[y]?.[x] || 0;
                const color = ColorMap.jet(value, this.minValue, this.maxValue);

                const px = padding + x * cellW;
                const py = padding + y * cellH;
                const wobble = Math.sin(this.time + x * 0.2 + y * 0.15) * 0.5;

                ctx.fillStyle = ColorMap.toRgb(color);
                ctx.fillRect(px, py + wobble, cellW + 0.5, cellH + 0.5);
            }
        }

        ctx.strokeStyle = "rgba(79, 195, 247, 0.5)";
        ctx.lineWidth = 2;
        ctx.strokeRect(padding, padding, viewW, viewH);

        ctx.fillStyle = "#8fa4c2";
        ctx.font = "12px -apple-system, 'PingFang SC', sans-serif";
        ctx.textAlign = "center";
        ctx.fillText("周向角度 (0° - 360°)", W / 2, H - 25);

        ctx.save();
        ctx.translate(20, H / 2);
        ctx.rotate(-Math.PI / 2);
        ctx.fillText("径向位置 (内 - 外)", 0, 0);
        ctx.restore();

        for (let i = 0; i <= 8; i++) {
            const x = padding + (viewW / 8) * i;
            ctx.strokeStyle = "rgba(255, 255, 255, 0.1)";
            ctx.lineWidth = 1;
            ctx.beginPath();
            ctx.moveTo(x, padding);
            ctx.lineTo(x, padding + viewH);
            ctx.stroke();

            ctx.fillStyle = "#5a7296";
            ctx.font = "10px 'SF Mono', Consolas, monospace";
            ctx.textAlign = "center";
            ctx.fillText(`${i * 45}°`, x, padding - 8);
        }

        for (let i = 0; i <= 4; i++) {
            const y = padding + (viewH / 4) * i;
            ctx.strokeStyle = "rgba(255, 255, 255, 0.1)";
            ctx.beginPath();
            ctx.moveTo(padding, y);
            ctx.lineTo(padding + viewW, y);
            ctx.stroke();

            const label = i === 0 ? "内圈" : i === 4 ? "外圈" : `${i * 25}%`;
            ctx.fillStyle = "#5a7296";
            ctx.font = "10px 'SF Mono', Consolas, monospace";
            ctx.textAlign = "right";
            ctx.fillText(label, padding - 8, y + 3);
        }
    },

    _draw3DView(ctx, W, H) {
        const cx = W / 2;
        const baseY = H * 0.7;
        const viewSize = Math.min(W, H) * 0.35;

        this.time += 0.008;
        const rotateAngle = this.time * 0.3;

        const cellSize = (viewSize * 2) / Math.max(this.gridSizeX, this.gridSizeY);

        for (let y = 0; y < this.gridSizeY; y += 2) {
            for (let x = 0; x < this.gridSizeX; x += 2) {
                const value = this.filmData[y]?.[x] || 0;
                const color = ColorMap.jet(value, this.minValue, this.maxValue);

                const normalizedX = (x / this.gridSizeX) - 0.5;
                const normalizedY = (y / this.gridSizeY) - 0.5;

                const cos = Math.cos(rotateAngle);
                const sin = Math.sin(rotateAngle);
                const rotX = normalizedX * cos - normalizedY * sin;
                const rotY = normalizedX * sin + normalizedY * cos;

                const heightValue = ((value - this.minValue) / (this.maxValue - this.minValue)) * 60;

                const screenX = cx + rotX * viewSize;
                const screenY = baseY - 60 + rotY * viewSize * 0.5 - heightValue;

                const depth = rotY * 0.5 + 0.5;
                const shade = 0.5 + depth * 0.5;

                ctx.fillStyle = `rgb(${Math.floor(color.r * shade)}, ${Math.floor(color.g * shade)}, ${Math.floor(color.b * shade)})`;

                const size = cellSize * (0.8 + depth * 0.4);
                ctx.fillRect(screenX - size / 2, screenY, size, 2 + heightValue * 0.5);

                const topColor = ColorMap.toRgb(color);
                ctx.fillStyle = topColor;
                ctx.fillRect(screenX - size / 2, screenY - 2, size, 3);
            }
        }

        ctx.strokeStyle = "rgba(79, 195, 247, 0.4)";
        ctx.lineWidth = 1;

        const drawAxisLine = (x1, y1, x2, y2) => {
            ctx.beginPath();
            ctx.moveTo(x1, y1);
            ctx.lineTo(x2, y2);
            ctx.stroke();
        };

        const axisLen = viewSize * 0.8;
        drawAxisLine(cx, baseY, cx + axisLen * Math.cos(rotateAngle), baseY - axisLen * 0.5 * Math.sin(rotateAngle));
        drawAxisLine(cx, baseY, cx - axisLen * Math.sin(rotateAngle), baseY - axisLen * 0.5 * Math.cos(rotateAngle));
        drawAxisLine(cx, baseY, cx, baseY - axisLen);

        ctx.fillStyle = "#8fa4c2";
        ctx.font = "11px -apple-system, 'PingFang SC', sans-serif";
        ctx.textAlign = "center";
        ctx.fillText("油膜厚度三维曲面", cx, baseY + 30);
        ctx.fillStyle = "#5a7296";
        ctx.font = "10px 'SF Mono', Consolas, monospace";
        ctx.fillText(`范围: ${this.minValue.toFixed(2)} - ${this.maxValue.toFixed(2)} μm`, cx, baseY + 48);
    },

    renderLegend(containerId) {
        const container = document.getElementById(containerId);
        if (!container) return;

        container.innerHTML = "";

        const bar = document.createElement("div");
        bar.className = "colormap-bar";
        container.appendChild(bar);

        const labels = document.createElement("div");
        labels.className = "colormap-labels";

        const steps = 6;
        for (let i = 0; i < steps; i++) {
            const span = document.createElement("span");
            const value = this.minValue + ((this.maxValue - this.minValue) * i) / (steps - 1);
            span.textContent = value.toFixed(2);
            labels.appendChild(span);
        }

        container.appendChild(labels);

        const title = document.createElement("p");
        title.style.cssText = "margin-top: 8px; font-size: 11px; color: #5a7296; text-align: center;";
        title.textContent = "油膜厚度 (μm)";
        container.appendChild(title);
    },

    animate() {
        this.animating = true;
        const loop = () => {
            if (!this.animating) return;
            this.render();
            requestAnimationFrame(loop);
        };
        loop();
    },

    destroy() {
        this.animating = false;
    },
};

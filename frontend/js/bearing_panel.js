const BearingPanel = {
    canvas: null,
    ctx: null,
    bearing: null,
    wearResult: null,
    sensorData: null,
    options: {
        showWear: true,
        showLoad: true,
    },
    animationFrame: 0,
    animating: false,
    offscreenCanvas: null,
    offscreenCtx: null,
    metalPattern: null,
    wearPattern: null,
    innerPattern: null,
    lastRenderedState: null,
    shaftAngle: 0,

    init(canvasId) {
        this.canvas = document.getElementById(canvasId);
        if (!this.canvas) {
            console.error(`未找到Canvas ${canvasId}`);
            return;
        }

        this.ctx = this.canvas.getContext("2d");
        this._resizeCanvas();
        window.addEventListener("resize", () => this._resizeCanvas());

        this._initOffscreenCanvases();

        this.animate();
    },

    _initOffscreenCanvases() {
        this.offscreenCanvas = document.createElement("canvas");
        this.offscreenCanvas.width = 512;
        this.offscreenCanvas.height = 512;
        this.offscreenCtx = this.offscreenCanvas.getContext("2d");

        this._buildMetalPattern();
        this._buildInnerPattern();
    },

    _buildMetalPattern() {
        const size = 64;
        const c = document.createElement("canvas");
        c.width = size;
        c.height = size;
        const ctx = c.getContext("2d");

        const grad = ctx.createLinearGradient(0, 0, size, size);
        grad.addColorStop(0, "#9e9e9e");
        grad.addColorStop(0.2, "#bdbdbd");
        grad.addColorStop(0.5, "#a0a0a0");
        grad.addColorStop(0.8, "#757575");
        grad.addColorStop(1, "#8a8a8a");
        ctx.fillStyle = grad;
        ctx.fillRect(0, 0, size, size);

        for (let i = 0; i < 200; i++) {
            const x = Math.random() * size;
            const y = Math.random() * size;
            const brightness = 120 + Math.random() * 60;
            ctx.fillStyle = `rgba(${brightness}, ${brightness}, ${brightness}, 0.3)`;
            ctx.fillRect(x, y, 1 + Math.random() * 2, 1);
        }

        for (let angle = 0; angle < Math.PI * 2; angle += Math.PI / 16) {
            ctx.strokeStyle = "rgba(100, 100, 100, 0.15)";
            ctx.lineWidth = 0.5;
            ctx.beginPath();
            ctx.moveTo(size / 2, size / 2);
            ctx.lineTo(
                size / 2 + Math.cos(angle) * size,
                size / 2 + Math.sin(angle) * size
            );
            ctx.stroke();
        }

        this.metalPattern = this.ctx.createPattern(c, "repeat");
    },

    _buildInnerPattern() {
        const size = 48;
        const c = document.createElement("canvas");
        c.width = size;
        c.height = size;
        const ctx = c.getContext("2d");

        const grad = ctx.createLinearGradient(0, 0, 0, size);
        grad.addColorStop(0, "#d8d8d8");
        grad.addColorStop(0.3, "#f0f0f0");
        grad.addColorStop(0.7, "#e8e8e8");
        grad.addColorStop(1, "#c8c8c8");
        ctx.fillStyle = grad;
        ctx.fillRect(0, 0, size, size);

        for (let i = 0; i < 100; i++) {
            const x = Math.random() * size;
            const y = Math.random() * size;
            const b = 200 + Math.random() * 55;
            ctx.fillStyle = `rgba(${b}, ${b}, ${b}, 0.25)`;
            ctx.fillRect(x, y, 1, 1 + Math.random());
        }

        this.innerPattern = this.ctx.createPattern(c, "repeat");
    },

    _buildWearPattern(wearRatio) {
        const size = 64;
        const c = document.createElement("canvas");
        c.width = size;
        c.height = size;
        const ctx = c.getContext("2d");

        const wearColor = ColorMap.heatColor(wearRatio);
        ctx.fillStyle = ColorMap.toRgba(wearColor, 0.6);
        ctx.fillRect(0, 0, size, size);

        for (let i = 0; i < 150; i++) {
            const x = Math.random() * size;
            const y = Math.random() * size;
            const intensity = 0.2 + Math.random() * 0.6;
            ctx.fillStyle = ColorMap.toRgba(wearColor, intensity);
            const w = 1 + Math.random() * 4;
            const h = 1 + Math.random() * 2;
            ctx.fillRect(x, y, w, h);
        }

        for (let angle = 0; angle < Math.PI * 2; angle += Math.PI / 12) {
            ctx.strokeStyle = ColorMap.toRgba(wearColor, 0.15);
            ctx.lineWidth = 0.5;
            ctx.beginPath();
            ctx.moveTo(size / 2, size / 2);
            ctx.lineTo(
                size / 2 + Math.cos(angle) * size,
                size / 2 + Math.sin(angle) * size
            );
            ctx.stroke();
        }

        this.wearPattern = this.ctx.createPattern(c, "repeat");
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

        if (this.metalPattern) this._buildMetalPattern();
        if (this.innerPattern) this._buildInnerPattern();
    },

    setBearing(bearing) {
        this.bearing = bearing;
    },

    setWearResult(result) {
        this.wearResult = result;
        if (result && this.bearing) {
            const wearRatio = Math.min(1, result.total_wear_microm / (this.bearing.wear_limit_microm || 100));
            this._buildWearPattern(wearRatio);
        }
    },

    setSensorData(data) {
        this.sensorData = data;
    },

    setOption(key, value) {
        this.options[key] = value;
    },

    render() {
        if (!this.ctx || !this._width || !this._height) return;

        const ctx = this.ctx;
        const W = this._width;
        const H = this._height;

        ctx.clearRect(0, 0, W, H);

        this._drawBackground(ctx, W, H);

        if (!this.bearing) {
            this._drawPlaceholder(ctx, W, H);
            return;
        }

        const cx = W / 2;
        const cy = H / 2;
        const scale = Math.min(W, H) / (this.bearing.outer_diameter * 3);

        this._drawBearingCrossSection(ctx, cx, cy, scale);

        if (this.options.showLoad) {
            this._drawLoadArrows(ctx, cx, cy, scale);
        }

        if (this.options.showWear) {
            this._drawWearIndicator(ctx, cx, cy, scale);
        }

        this._drawLabels(ctx, cx, cy, scale);

        this.shaftAngle += 0.03;
    },

    _drawBackground(ctx, W, H) {
        const gradient = ctx.createRadialGradient(W / 2, H / 2, 0, W / 2, H / 2, Math.max(W, H));
        gradient.addColorStop(0, "#1a2d47");
        gradient.addColorStop(1, "#0a1628");
        ctx.fillStyle = gradient;
        ctx.fillRect(0, 0, W, H);

        ctx.strokeStyle = "rgba(79, 195, 247, 0.05)";
        ctx.lineWidth = 1;
        const gridSize = 40;

        for (let x = 0; x < W; x += gridSize) {
            ctx.beginPath();
            ctx.moveTo(x, 0);
            ctx.lineTo(x, H);
            ctx.stroke();
        }
        for (let y = 0; y < H; y += gridSize) {
            ctx.beginPath();
            ctx.moveTo(0, y);
            ctx.lineTo(W, y);
            ctx.stroke();
        }
    },

    _drawPlaceholder(ctx, W, H) {
        ctx.fillStyle = "rgba(143, 164, 194, 0.5)";
        ctx.font = "18px -apple-system, 'PingFang SC', sans-serif";
        ctx.textAlign = "center";
        ctx.textBaseline = "middle";
        ctx.fillText("请选择要查看的轴承", W / 2, H / 2);
    },

    _drawBearingCrossSection(ctx, cx, cy, scale) {
        const b = this.bearing;
        const innerR = (b.inner_diameter / 2) * scale;
        const outerR = (b.outer_diameter / 2) * scale;
        const widthPx = b.width * scale * 0.8;

        let wearRatio = 0;
        if (this.wearResult && b.wear_limit_microm > 0) {
            wearRatio = Math.min(1, this.wearResult.total_wear_microm / b.wear_limit_microm);
        }

        const wearDepth = wearRatio * 8;

        ctx.save();
        ctx.translate(cx, cy);

        if (this.metalPattern) {
            ctx.save();
            ctx.translate(-outerR, -widthPx / 2);
            ctx.fillStyle = this.metalPattern;
            ctx.beginPath();
            ctx.roundRect(0, 0, outerR * 2, widthPx, 8);
            ctx.fill();
            ctx.restore();
        } else {
            const outerGradient = ctx.createLinearGradient(0, -outerR, 0, outerR);
            outerGradient.addColorStop(0, "#9e9e9e");
            outerGradient.addColorStop(0.3, "#bdbdbd");
            outerGradient.addColorStop(0.7, "#757575");
            outerGradient.addColorStop(1, "#616161");
            ctx.fillStyle = outerGradient;
            ctx.beginPath();
            ctx.roundRect(-outerR, -widthPx / 2, outerR * 2, widthPx, 8);
            ctx.fill();
        }

        ctx.strokeStyle = "#424242";
        ctx.lineWidth = 2;
        ctx.beginPath();
        ctx.roundRect(-outerR, -widthPx / 2, outerR * 2, widthPx, 8);
        ctx.stroke();

        let innerX1 = -innerR;
        let innerX2 = innerR;
        const innerY1 = -widthPx / 2 + 10;
        const innerY2 = widthPx / 2 - 10;

        if (wearRatio > 0) {
            ctx.save();

            if (this.wearPattern) {
                const patternTransform = new DOMMatrix();
                patternTransform.rotateSelf(this.shaftAngle * 180 / Math.PI);
                this.wearPattern.setTransform(patternTransform);

                ctx.fillStyle = this.wearPattern;
            } else {
                const wearColor = ColorMap.heatColor(wearRatio);
                const wearGradient = ctx.createRadialGradient(0, 0, innerR - wearDepth, 0, 0, innerR);
                wearGradient.addColorStop(0, ColorMap.toRgba(wearColor, 0.9));
                wearGradient.addColorStop(1, ColorMap.toRgba(wearColor, 0.4));
                ctx.fillStyle = wearGradient;
            }

            ctx.beginPath();
            ctx.ellipse(0, 0, innerR + wearDepth / 2, widthPx / 2 - 5, 0, 0, Math.PI * 2);
            ctx.fill();

            ctx.restore();
        }

        ctx.save();
        ctx.rotate(this.shaftAngle * 0.3);

        if (this.innerPattern) {
            ctx.save();
            ctx.translate(innerX1 + wearDepth, innerY1);
            ctx.fillStyle = this.innerPattern;
            ctx.beginPath();
            ctx.roundRect(0, 0, (innerX2 - innerX1) - wearDepth * 2, innerY2 - innerY1, 4);
            ctx.fill();
            ctx.restore();
        } else {
            const innerGradient = ctx.createLinearGradient(0, innerY1, 0, innerY2);
            innerGradient.addColorStop(0, "#e0e0e0");
            innerGradient.addColorStop(0.5, "#f5f5f5");
            innerGradient.addColorStop(1, "#bdbdbd");
            ctx.fillStyle = innerGradient;
            ctx.beginPath();
            ctx.roundRect(innerX1 + wearDepth, innerY1, (innerX2 - innerX1) - wearDepth * 2, innerY2 - innerY1, 4);
            ctx.fill();
        }

        ctx.strokeStyle = "#9e9e9e";
        ctx.lineWidth = 1.5;
        ctx.beginPath();
        ctx.roundRect(innerX1 + wearDepth, innerY1, (innerX2 - innerX1) - wearDepth * 2, innerY2 - innerY1, 4);
        ctx.stroke();

        if (this.options.showWear && wearRatio > 0) {
            ctx.strokeStyle = ColorMap.toRgb(ColorMap.heatColor(wearRatio));
            ctx.lineWidth = 3;
            ctx.setLineDash([5, 3]);
            ctx.beginPath();
            ctx.roundRect(innerX1, innerY1 - 2, innerX2 - innerX1, innerY2 - innerY1 + 4, 4);
            ctx.stroke();
            ctx.setLineDash([]);
        }

        ctx.restore();

        let filmThickness = 3;
        if (this.sensorData) {
            filmThickness = this.sensorData.oil_film_thickness || 3;
        }
        const filmAlpha = Math.min(0.8, filmThickness / 5);

        const filmColor = ColorMap.oilFilm(filmThickness / 8);

        ctx.save();
        ctx.rotate(this.shaftAngle * 0.15);

        const filmWidth = 4;
        for (let y = innerY1 + 2; y < innerY2 - 2; y += 3) {
            const shimmer = Math.sin(y * 0.1 + this.shaftAngle * 2) * 0.2;
            ctx.fillStyle = ColorMap.toRgba(filmColor, Math.max(0.1, filmAlpha + shimmer));
            ctx.fillRect(innerX1 - filmWidth, y, filmWidth, 2);
            ctx.fillRect(innerX2, y, filmWidth, 2);
        }

        ctx.restore();

        ctx.fillStyle = "#424242";
        ctx.beginPath();
        ctx.arc(0, 0, 5, 0, Math.PI * 2);
        ctx.fill();

        ctx.strokeStyle = "#66bb6a";
        ctx.lineWidth = 2;
        ctx.beginPath();
        ctx.arc(0, 0, 5, 0, Math.PI * 2);
        ctx.stroke();

        ctx.save();
        ctx.rotate(this.shaftAngle);

        const numSpokes = 4;
        for (let i = 0; i < numSpokes; i++) {
            const angle = (i / numSpokes) * Math.PI * 2;
            ctx.strokeStyle = `rgba(102, 187, 106, ${0.3 + 0.3 * Math.abs(Math.cos(angle + this.shaftAngle))})`;
            ctx.lineWidth = 1;
            ctx.beginPath();
            ctx.moveTo(0, 0);
            ctx.lineTo(
                Math.cos(angle) * (innerR - 10),
                Math.sin(angle) * (innerR - 10)
            );
            ctx.stroke();
        }

        ctx.restore();

        ctx.restore();
    },

    _drawLoadArrows(ctx, cx, cy, scale) {
        if (!this.sensorData || !this.sensorData.radial_load) return;

        const load = this.sensorData.radial_load;
        const maxLoad = 20000;
        const loadRatio = Math.min(1, load / maxLoad);
        const arrowLength = 30 + loadRatio * 60;

        ctx.save();
        ctx.translate(cx, cy);

        const arrowColor = loadRatio > 0.8 ? "#ef5350" : loadRatio > 0.5 ? "#ffa726" : "#66bb6a";

        ctx.strokeStyle = arrowColor;
        ctx.fillStyle = arrowColor;
        ctx.lineWidth = 3;

        const b = this.bearing;
        const outerR = (b.outer_diameter / 2) * scale;

        const angles = [0, Math.PI / 6, -Math.PI / 6];

        angles.forEach((angle) => {
            const startX = Math.cos(angle) * (outerR + 20);
            const startY = Math.sin(angle) * (outerR + 20);
            const endX = Math.cos(angle) * (outerR + 20 + arrowLength);
            const endY = Math.sin(angle) * (outerR + 20 + arrowLength);

            ctx.beginPath();
            ctx.moveTo(startX, startY);
            ctx.lineTo(endX, endY);
            ctx.stroke();

            const headLength = 10;
            const headAngle = Math.PI / 6;
            ctx.beginPath();
            ctx.moveTo(endX, endY);
            ctx.lineTo(
                endX - headLength * Math.cos(angle - headAngle),
                endY - headLength * Math.sin(angle - headAngle)
            );
            ctx.lineTo(
                endX - headLength * Math.cos(angle + headAngle),
                endY - headLength * Math.sin(angle + headAngle)
            );
            ctx.closePath();
            ctx.fill();
        });

        ctx.font = "12px 'SF Mono', Consolas, monospace";
        ctx.textAlign = "center";
        ctx.fillStyle = arrowColor;
        ctx.fillText(`载荷: ${load.toFixed(0)} N`, 0, -outerR - 50);

        ctx.restore();
    },

    _drawWearIndicator(ctx, cx, cy, scale) {
        if (!this.bearing || !this.wearResult) return;

        const b = this.bearing;
        const totalWear = this.wearResult.total_wear_microm || 0;
        const wearLimit = b.wear_limit_microm || 100;
        const wearRatio = Math.min(1, totalWear / wearLimit);

        const barWidth = 200;
        const barHeight = 12;
        const barX = cx - barWidth / 2;
        const barY = this._height - 60;

        ctx.save();

        ctx.fillStyle = "rgba(10, 22, 40, 0.8)";
        ctx.fillRect(barX - 10, barY - 30, barWidth + 20, 60);
        ctx.strokeStyle = "rgba(79, 195, 247, 0.3)";
        ctx.lineWidth = 1;
        ctx.strokeRect(barX - 10, barY - 30, barWidth + 20, 60);

        ctx.font = "11px -apple-system, 'PingFang SC', sans-serif";
        ctx.textAlign = "left";
        ctx.fillStyle = "#8fa4c2";
        ctx.fillText("磨损深度进度", barX - 5, barY - 12);

        ctx.fillStyle = "#1a2d47";
        ctx.beginPath();
        ctx.roundRect(barX, barY, barWidth, barHeight, 6);
        ctx.fill();

        const fillWidth = barWidth * wearRatio;
        const wearColor = ColorMap.heatColor(wearRatio);
        const wearGradient = ctx.createLinearGradient(barX, barY, barX + barWidth, barY);
        wearGradient.addColorStop(0, "#66bb6a");
        wearGradient.addColorStop(0.5, "#ffa726");
        wearGradient.addColorStop(1, "#ef5350");

        ctx.fillStyle = wearGradient;
        ctx.beginPath();
        ctx.roundRect(barX, barY, fillWidth, barHeight, 6);
        ctx.fill();

        const warnX = barX + barWidth * 0.7;
        const critX = barX + barWidth * 0.9;

        ctx.strokeStyle = "#ffa726";
        ctx.lineWidth = 2;
        ctx.beginPath();
        ctx.moveTo(warnX, barY - 2);
        ctx.lineTo(warnX, barY + barHeight + 2);
        ctx.stroke();

        ctx.strokeStyle = "#ef5350";
        ctx.beginPath();
        ctx.moveTo(critX, barY - 2);
        ctx.lineTo(critX, barY + barHeight + 2);
        ctx.stroke();

        ctx.font = "bold 12px 'SF Mono', Consolas, monospace";
        ctx.textAlign = "center";
        ctx.fillStyle = ColorMap.toRgb(wearColor);
        ctx.fillText(`${totalWear.toFixed(2)} / ${wearLimit} μm (${(wearRatio * 100).toFixed(1)}%)`, cx, barY + 26);

        ctx.restore();
    },

    _drawLabels(ctx, cx, cy, scale) {
        if (!this.bearing) return;

        const b = this.bearing;
        const innerR = (b.inner_diameter / 2) * scale;
        const outerR = (b.outer_diameter / 2) * scale;

        ctx.save();
        ctx.translate(cx, cy);

        ctx.font = "11px 'SF Mono', Consolas, monospace";
        ctx.fillStyle = "#8fa4c2";
        ctx.strokeStyle = "rgba(143, 164, 194, 0.5)";
        ctx.lineWidth = 1;
        ctx.setLineDash([3, 3]);

        ctx.beginPath();
        ctx.moveTo(0, 0);
        ctx.lineTo(outerR + 60, 0);
        ctx.stroke();

        ctx.beginPath();
        ctx.moveTo(0, 0);
        ctx.lineTo(innerR + 30, 40);
        ctx.stroke();

        ctx.setLineDash([]);

        ctx.textAlign = "left";
        ctx.fillStyle = "#4fc3f7";
        ctx.fillText(`外径: ${b.outer_diameter} mm`, outerR + 10, -5);
        ctx.fillText(`内径: ${b.inner_diameter} mm`, innerR + 10, 45);
        ctx.fillText(`宽度: ${b.width} mm`, outerR + 10, 15);

        const titleY = -(outerR + 80);
        ctx.font = "bold 14px -apple-system, 'PingFang SC', sans-serif";
        ctx.textAlign = "center";
        ctx.fillStyle = "#e8eef7";
        ctx.fillText(`${b.bearing_code} - ${b.position}`, 0, titleY);

        ctx.font = "11px -apple-system, 'PingFang SC', sans-serif";
        ctx.fillStyle = "#5a7296";
        ctx.fillText(`${b.bearing_type} | 材质: ${b.material}`, 0, titleY + 18);

        ctx.restore();
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

const Charts = {
    realtimeChart: null,
    wearHistoryChart: null,
    reliabilityChart: null,
    pdfChart: null,
    wearTrendChart: null,

    initRealtime(canvasId) {
        const ctx = document.getElementById(canvasId);
        if (!ctx) return;

        this.realtimeChart = new Chart(ctx, {
            type: "line",
            data: {
                labels: [],
                datasets: [
                    {
                        label: "温度 (°C)",
                        data: [],
                        borderColor: "#ef5350",
                        backgroundColor: "rgba(239, 83, 80, 0.1)",
                        yAxisID: "y",
                        tension: 0.4,
                        fill: true,
                        borderWidth: 2,
                        pointRadius: 0,
                    },
                    {
                        label: "转速 (RPM)",
                        data: [],
                        borderColor: "#66bb6a",
                        backgroundColor: "rgba(102, 187, 106, 0.1)",
                        yAxisID: "y1",
                        tension: 0.4,
                        fill: true,
                        borderWidth: 2,
                        pointRadius: 0,
                    },
                    {
                        label: "油膜 (μm)",
                        data: [],
                        borderColor: "#4fc3f7",
                        backgroundColor: "rgba(79, 195, 247, 0.1)",
                        yAxisID: "y2",
                        tension: 0.4,
                        fill: true,
                        borderWidth: 2,
                        pointRadius: 0,
                    },
                ],
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                interaction: {
                    mode: "index",
                    intersect: false,
                },
                plugins: {
                    legend: {
                        labels: {
                        color: "#8fa4c2",
                        font: { size: 11 },
                        boxWidth: 12,
                    },
                },
                },
                scales: {
                    x: {
                        ticks: {
                        color: "#5a7296",
                        font: { size: 10 },
                        maxTicksLimit: 8,
                    },
                    grid: { color: "rgba(42, 67, 101, 0.3)" },
                },
                y: {
                    type: "linear",
                    display: true,
                    position: "left",
                    title: {
                        display: true,
                        text: "温度 (°C)",
                        color: "#ef5350",
                    },
                    ticks: { color: "#5a7296" },
                    grid: { color: "rgba(42, 67, 101, 0.3)" },
                },
                y1: {
                    type: "linear",
                    display: true,
                    position: "right",
                    title: {
                        display: true,
                        text: "转速 (RPM)",
                        color: "#66bb6a",
                    },
                    ticks: { color: "#5a7296" },
                    grid: { drawOnChartArea: false },
                },
                y2: {
                    type: "linear",
                    display: false,
                    position: "right",
                    title: {
                        display: true,
                        text: "油膜 (μm)",
                        color: "#4fc3f7",
                    },
                    ticks: { color: "#5a7296" },
                    grid: { drawOnChartArea: false },
                },
            },
        });
    },

    updateRealtime(dataPoint) {
        if (!this.realtimeChart) return;

        const time = new Date(dataPoint.time || Date.now()).toLocaleTimeString("zh-CN", {
            hour: "2-digit",
            minute: "2-digit",
            second: "2-digit",
        });

        this.realtimeChart.data.labels.push(time);
        this.realtimeChart.data.datasets[0].data.push(dataPoint.temperature ?? null);
        this.realtimeChart.data.datasets[1].data.push(dataPoint.rotational_speed ?? null);
        this.realtimeChart.data.datasets[2].data.push(dataPoint.oil_film_thickness ?? null);

        const maxPoints = 30;
        if (this.realtimeChart.data.labels.length > maxPoints) {
            this.realtimeChart.data.labels.shift();
            this.realtimeChart.data.datasets.forEach((ds) => ds.data.shift());
        }

        this.realtimeChart.update("none");
    },

    initWearHistory(canvasId) {
        const ctx = document.getElementById(canvasId);
        if (!ctx) return;

        this.wearHistoryChart = new Chart(ctx, {
            type: "line",
            data: {
                labels: [],
                datasets: [
                    {
                        label: "累计磨损 (μm)",
                        data: [],
                        borderColor: "#ffa726",
                        backgroundColor: "rgba(255, 167, 38, 0.1)",
                        fill: true,
                        tension: 0.3,
                        borderWidth: 2,
                        pointRadius: 3,
                        pointBackgroundColor: "#ffa726",
                    },
                    {
                        label: "磨损率 (μm/h)",
                        data: [],
                        borderColor: "#ab47bc",
                        backgroundColor: "rgba(171, 71, 188, 0.1)",
                        fill: false,
                        tension: 0.3,
                        borderWidth: 2,
                        yAxisID: "y1",
                        pointRadius: 3,
                        pointBackgroundColor: "#ab47bc",
                    },
                ],
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        labels: {
                            color: "#8fa4c2",
                            font: { size: 11 },
                            boxWidth: 12,
                        },
                    },
                },
                scales: {
                    x: {
                        ticks: {
                            color: "#5a7296",
                            font: { size: 10 },
                            maxTicksLimit: 6,
                        },
                        grid: { color: "rgba(42, 67, 101, 0.3)" },
                    },
                    y: {
                        title: {
                            display: true,
                            text: "累计磨损 (μm)",
                            color: "#ffa726",
                        },
                        ticks: { color: "#5a7296" },
                        grid: { color: "rgba(42, 67, 101, 0.3)" },
                        beginAtZero: true,
                    },
                    y1: {
                        position: "right",
                        title: {
                            display: true,
                            text: "磨损率 (μm/h)",
                            color: "#ab47bc",
                        },
                        ticks: { color: "#5a7296" },
                        grid: { drawOnChartArea: false },
                        beginAtZero: true,
                    },
                },
            },
        });
    },

    updateWearHistory(history) {
        if (!this.wearHistoryChart || !history) return;

        const sorted = [...history].reverse();

        this.wearHistoryChart.data.labels = sorted.map((w) =>
            new Date(w.calculated_at).toLocaleDateString("zh-CN", {
                month: "2-digit",
                day: "2-digit",
                hour: "2-digit",
            })
        );

        this.wearHistoryChart.data.datasets[0].data = sorted.map((w) => w.total_wear_microm ?? null);
        this.wearHistoryChart.data.datasets[1].data = sorted.map((w) => w.wear_rate_microm_per_hour ?? null);

        this.wearHistoryChart.update();
    },

    initReliability(canvasId) {
        const ctx = document.getElementById(canvasId);
        if (!ctx) return;

        this.reliabilityChart = new Chart(ctx, {
            type: "line",
            data: {
                labels: [],
                datasets: [
                    {
                        label: "可靠度 R(t)",
                        data: [],
                        borderColor: "#4fc3f7",
                        backgroundColor: "rgba(79, 195, 247, 0.15)",
                        fill: true,
                        tension: 0.3,
                        borderWidth: 2,
                        pointRadius: 0,
                    },
                    {
                        label: "失效概率 F(t)",
                        data: [],
                        borderColor: "#ef5350",
                        backgroundColor: "rgba(239, 83, 80, 0.1)",
                        fill: true,
                        tension: 0.3,
                        borderWidth: 2,
                        pointRadius: 0,
                    },
                ],
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        labels: {
                            color: "#8fa4c2",
                            font: { size: 11 },
                            boxWidth: 12,
                        },
                    },
                    tooltip: {
                        callbacks: {
                            label: (context) => {
                                return `${context.dataset.label}: ${(context.parsed.y * 100).toFixed(2)}%`;
                            },
                        },
                    },
                },
                scales: {
                    x: {
                    title: {
                        display: true,
                        text: "运行时间 (小时)",
                        color: "#8fa4c2",
                    },
                    ticks: { color: "#5a7296" },
                    grid: { color: "rgba(42, 67, 101, 0.3)" },
                },
                    y: {
                    title: {
                        display: true,
                        text: "概率",
                        color: "#8fa4c2",
                    },
                    ticks: {
                        color: "#5a7296",
                        callback: (v) => `${(v * 100).toFixed(0)}%`,
                    },
                    min: 0,
                    max: 1,
                    grid: { color: "rgba(42, 67, 101, 0.3)" },
                },
            },
        });
    },

    updateReliability(shape, scale, runningHours, predictedRUL) {
        if (!this.reliabilityChart) return;

        const labels = [];
        const reliabilityData = [];
        const failureData = [];

        const maxTime = Math.max(runningHours + predictedRUL * 1.5, scale * 1.5);
        const step = maxTime / 100;

        for (let t = 0; t <= maxTime; t += step) {
            labels.push(t.toFixed(0));
            const R = Math.exp(-Math.pow(t / scale, shape));
            reliabilityData.push(R);
            failureData.push(1 - R);
        }

        this.reliabilityChart.data.labels = labels;
        this.reliabilityChart.data.datasets[0].data = reliabilityData;
        this.reliabilityChart.data.datasets[1].data = failureData;

        this.reliabilityChart.update();
    },

    initPDF(canvasId) {
        const ctx = document.getElementById(canvasId);
        if (!ctx) return;

        this.pdfChart = new Chart(ctx, {
            type: "line",
            data: {
                labels: [],
                datasets: [
                    {
                        label: "概率密度 f(t)",
                        data: [],
                        borderColor: "#ab47bc",
                        backgroundColor: "rgba(171, 71, 188, 0.2)",
                        fill: true,
                        tension: 0.4,
                        borderWidth: 2,
                        pointRadius: 0,
                    },
                ],
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        labels: {
                            color: "#8fa4c2",
                            font: { size: 11 },
                            boxWidth: 12,
                        },
                    },
                },
                scales: {
                    x: {
                        title: {
                            display: true,
                            text: "失效时间 (小时)",
                            color: "#8fa4c2",
                        },
                        ticks: { color: "#5a7296" },
                        grid: { color: "rgba(42, 67, 101, 0.3)" },
                    },
                    y: {
                        title: {
                            display: true,
                            text: "f(t)",
                            color: "#8fa4c2",
                        },
                        ticks: { color: "#5a7296" },
                        grid: { color: "rgba(42, 67, 101, 0.3)" },
                        beginAtZero: true,
                    },
                },
            },
        });
    },

    updatePDF(shape, scale, runningHours) {
        if (!this.pdfChart) return;

        const labels = [];
        const data = [];

        const maxTime = scale * 2;
        const step = maxTime / 100;

        let maxPDF = 0;

        for (let t = 0.01; t <= maxTime; t += step) {
            labels.push(t.toFixed(0));
            const pdf =
                (shape / scale) *
                Math.pow(t / scale, shape - 1) *
                Math.exp(-Math.pow(t / scale, shape));
            data.push(pdf);
            if (pdf > maxPDF) maxPDF = pdf;
        }

        this.pdfChart.data.labels = labels;
        this.pdfChart.data.datasets[0].data = data;
        this.pdfChart.update();
    },

    initWearTrend(canvasId) {
        const ctx = document.getElementById(canvasId);
        if (!ctx) return;

        this.wearTrendChart = new Chart(ctx, {
            type: "line",
            data: {
                labels: [],
                datasets: [
                    {
                        label: "磨损率趋势 (μm/h)",
                        data: [],
                        borderColor: "#ffa726",
                        backgroundColor: "rgba(255, 167, 38, 0.1)",
                        fill: true,
                        tension: 0.3,
                        borderWidth: 2,
                        pointRadius: 4,
                        pointBackgroundColor: "#ffa726",
                    },
                    {
                        label: "线性拟合",
                        data: [],
                        borderColor: "#ef5350",
                        borderDash: [5, 5],
                        backgroundColor: "transparent",
                        fill: false,
                        tension: 0,
                        borderWidth: 2,
                        pointRadius: 0,
                    },
                ],
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        labels: {
                            color: "#8fa4c2",
                            font: { size: 11 },
                            boxWidth: 12,
                        },
                    },
                },
                scales: {
                    x: {
                        ticks: {
                            color: "#5a7296",
                            font: { size: 10 },
                            maxTicksLimit: 8,
                        },
                        grid: { color: "rgba(42, 67, 101, 0.3)" },
                    },
                    y: {
                        title: {
                            display: true,
                            text: "磨损率 (μm/h)",
                            color: "#ffa726",
                        },
                        ticks: { color: "#5a7296" },
                        grid: { color: "rgba(42, 67, 101, 0.3)" },
                        beginAtZero: true,
                    },
                },
            },
        });
    },

    updateWearTrend(history) {
        if (!this.wearTrendChart || !history || history.length === 0) return;

        const sorted = [...history].reverse();
        const wearRates = sorted
            .map((w) => w.wear_rate_microm_per_hour)
            .filter((v) => v != null);

        if (wearRates.length === 0) return;

        const labels = sorted
            .map((w, i) => i + 1);

        const data = wearRates;

        let sumX = 0,
            sumY = 0,
            sumXY = 0,
            sumX2 = 0;
        const n = wearRates.length;

        for (let i = 0; i < n; i++) {
            const x = i + 1;
            const y = wearRates[i];
            sumX += x;
            sumY += y;
            sumXY += x * y;
            sumX2 += x * x;
        }

        const slope = (n * sumXY - sumX * sumY) / (n * sumX2 - sumX * sumX);
        const intercept = (sumY - slope * sumX) / n;

        const trend = labels.map((x) => slope * x + intercept);

        this.wearTrendChart.data.labels = labels;
        this.wearTrendChart.data.datasets[0].data = data;
        this.wearTrendChart.data.datasets[1].data = trend;

        this.wearTrendChart.update();
    },

    destroyAll() {
        Object.values(this).forEach((chart) => {
            if (chart && typeof chart.destroy === "function") {
                chart.destroy();
            }
        });
    },
};

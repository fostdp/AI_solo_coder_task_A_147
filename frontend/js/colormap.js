const ColorMap = {
    jet(value, min = 0, max = 1) {
        if (min === max) return { r: 0, g: 0, b: 255 };
        const t = Math.max(0, Math.min(1, (value - min) / (max - min)));

        let r, g, b;
        if (t < 0.125) {
            r = 0;
            g = 0;
            b = 128 + Math.round(127 * (t / 0.125));
        } else if (t < 0.375) {
            r = 0;
            g = Math.round(255 * ((t - 0.125) / 0.25));
            b = 255;
        } else if (t < 0.625) {
            r = Math.round(255 * ((t - 0.375) / 0.25));
            g = 255;
            b = Math.round(255 * (1 - (t - 0.375) / 0.25));
        } else if (t < 0.875) {
            r = 255;
            g = Math.round(255 * (1 - (t - 0.625) / 0.25));
            b = 0;
        } else {
            r = 255 - Math.round(128 * ((t - 0.875) / 0.125));
            g = 0;
            b = 0;
        }

        return { r, g, b };
    },

    viridis(value, min = 0, max = 1) {
        if (min === max) return { r: 68, g: 1, b: 84 };
        const t = Math.max(0, Math.min(1, (value - min) / (max - min)));

        const stops = [
            { t: 0.0, r: 68, g: 1, b: 84 },
            { t: 0.1, r: 72, g: 35, b: 116 },
            { t: 0.2, r: 67, g: 62, b: 133 },
            { t: 0.3, r: 57, g: 86, b: 140 },
            { t: 0.4, r: 47, g: 108, b: 142 },
            { t: 0.5, r: 38, g: 130, b: 142 },
            { t: 0.6, r: 31, g: 152, b: 138 },
            { t: 0.7, r: 34, g: 173, b: 129 },
            { t: 0.8, r: 68, g: 194, b: 112 },
            { t: 0.9, r: 121, g: 213, b: 84 },
            { t: 1.0, r: 199, g: 231, b: 61 },
        ];

        return this._interpolateStops(t, stops);
    },

    plasma(value, min = 0, max = 1) {
        if (min === max) return { r: 13, g: 8, b: 135 };
        const t = Math.max(0, Math.min(1, (value - min) / (max - min)));

        const stops = [
            { t: 0.0, r: 13, g: 8, b: 135 },
            { t: 0.25, r: 75, g: 3, b: 161 },
            { t: 0.5, r: 125, g: 3, b: 168 },
            { t: 0.75, r: 187, g: 54, b: 124 },
            { t: 1.0, r: 240, g: 249, b: 33 },
        ];

        return this._interpolateStops(t, stops);
    },

    oilFilm(value, min = 0, max = 1) {
        if (min === max) return { r: 100, g: 0, b: 0 };
        const t = Math.max(0, Math.min(1, (value - min) / (max - min)));

        const stops = [
            { t: 0.0, r: 80, g: 0, b: 0 },
            { t: 0.15, r: 180, g: 30, b: 30 },
            { t: 0.3, r: 255, g: 120, b: 40 },
            { t: 0.5, r: 255, g: 220, b: 80 },
            { t: 0.65, r: 120, g: 200, b: 100 },
            { t: 0.8, r: 60, g: 160, b: 200 },
            { t: 0.95, r: 40, g: 80, b: 180 },
            { t: 1.0, r: 20, g: 30, b: 100 },
        ];

        return this._interpolateStops(t, stops);
    },

    toHex(color) {
        const r = color.r.toString(16).padStart(2, "0");
        const g = color.g.toString(16).padStart(2, "0");
        const b = color.b.toString(16).padStart(2, "0");
        return `#${r}${g}${b}`;
    },

    toRgba(color, alpha = 1) {
        return `rgba(${color.r}, ${color.g}, ${color.b}, ${alpha})`;
    },

    toRgb(color) {
        return `rgb(${color.r}, ${color.g}, ${color.b})`;
    },

    _interpolateStops(t, stops) {
        if (t <= stops[0].t) return stops[0];
        if (t >= stops[stops.length - 1].t) return stops[stops.length - 1];

        for (let i = 0; i < stops.length - 1; i++) {
            const s1 = stops[i];
            const s2 = stops[i + 1];
            if (t >= s1.t && t <= s2.t) {
                const localT = (t - s1.t) / (s2.t - s1.t);
                return {
                    r: Math.round(s1.r + (s2.r - s1.r) * localT),
                    g: Math.round(s1.g + (s2.g - s1.g) * localT),
                    b: Math.round(s1.b + (s2.b - s1.b) * localT),
                };
            }
        }
        return stops[stops.length - 1];
    },

    heatColor(wearRatio) {
        if (wearRatio < 0.5) {
            const t = wearRatio / 0.5;
            return {
                r: Math.round(102 + t * 50),
                g: Math.round(187 - t * 20),
                b: Math.round(106 - t * 50),
            };
        } else if (wearRatio < 0.8) {
            const t = (wearRatio - 0.5) / 0.3;
            return {
                r: Math.round(152 + t * 103),
                g: Math.round(167 - t * 60),
                b: Math.round(56 - t * 18),
            };
        } else {
            const t = Math.min(1, (wearRatio - 0.8) / 0.2);
            return {
                r: Math.round(239 + t * 16),
                g: Math.round(83 - t * 33),
                b: Math.round(80 - t * 30),
            };
        }
    },
};

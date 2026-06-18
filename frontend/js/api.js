const API = {
    baseUrl: "http://localhost:8080/api/v1",

    async getNoriaWheels() {
        return this._request("/noria-wheels");
    },

    async getBearings(noriaWheelId = null) {
        const url = noriaWheelId
            ? `/bearings?noria_wheel_id=${noriaWheelId}`
            : "/bearings";
        return this._request(url);
    },

    async getBearingById(id) {
        return this._request(`/bearings/${id}`);
    },

    async getBearingStatuses() {
        return this._request("/bearings/status");
    },

    async getSensorData(bearingId, hours = 24) {
        return this._request(`/bearings/${bearingId}/sensor-data?hours=${hours}`);
    },

    async getLatestSensorData(bearingId) {
        return this._request(`/bearings/${bearingId}/sensor-data/latest`);
    },

    async postSensorData(data) {
        return this._request("/sensor-data", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(data),
        });
    },

    async getWearHistory(bearingId, limit = 100) {
        return this._request(`/bearings/${bearingId}/wear-history?limit=${limit}`);
    },

    async getLatestWearResult(bearingId) {
        return this._request(`/bearings/${bearingId}/wear/latest`);
    },

    async getLatestLifePrediction(bearingId) {
        return this._request(`/bearings/${bearingId}/life-prediction/latest`);
    },

    async getOilFilmMap(bearingId) {
        return this._request(`/bearings/${bearingId}/oil-film-map`);
    },

    async triggerCalculation(bearingId) {
        return this._request("/calculations/trigger", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ bearing_id: bearingId }),
        });
    },

    async getRecentAlerts(limit = 50) {
        return this._request(`/alerts/recent?limit=${limit}`);
    },

    async _request(url, options = {}) {
        try {
            const response = await fetch(this.baseUrl + url, {
                ...options,
                credentials: "include",
            });

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }

            const text = await response.text();
            if (!text) return null;

            try {
                return JSON.parse(text);
            } catch {
                return text;
            }
        } catch (error) {
            console.error(`API请求失败 ${url}:`, error);
            throw error;
        }
    },
};

import axios from "axios";
import {
    clearSession,
    getAccessToken,
    getRefreshToken,
    getSession,
    setSession,
} from "./tokenStorage";

const api = axios.create({
    baseURL: import.meta.env.VITE_API_URL,
    headers: {
        "Content-Type": "application/json",
    },
});

api.interceptors.request.use((config) => {
    const accessToken = getAccessToken();
    if (accessToken) {
        config.headers.Authorization = `Bearer ${accessToken}`;
    }
    return config;
});

let refreshPromise = null;

const requestNewAccessToken = async () => {
    const refreshToken = getRefreshToken();
    if (!refreshToken) {
        throw new Error("no refresh token available");
    }

    // Petición sin interceptores para no reentrar en este mismo flujo.
    const response = await axios.post(
        `${import.meta.env.VITE_API_URL}/api/auth/refresh`,
        { refresh_token: refreshToken }
    );

    const session = getSession();
    setSession({
        accessToken: response.data.access_token,
        refreshToken: response.data.refresh_token,
        user: session?.user ?? null,
    });

    return response.data.access_token;
};

api.interceptors.response.use(
    (response) => response,
    async (error) => {
        const originalRequest = error.config;
        const isAuthEndpoint =
            originalRequest?.url?.includes("/api/auth/login") ||
            originalRequest?.url?.includes("/api/auth/refresh");

        if (
            error.response?.status !== 401 ||
            isAuthEndpoint ||
            originalRequest._retry
        ) {
            return Promise.reject(error);
        }

        originalRequest._retry = true;

        try {
            refreshPromise = refreshPromise ?? requestNewAccessToken();
            const accessToken = await refreshPromise;
            refreshPromise = null;

            originalRequest.headers.Authorization = `Bearer ${accessToken}`;
            return api(originalRequest);
        } catch (refreshError) {
            refreshPromise = null;
            clearSession();
            window.dispatchEvent(new Event("auth:session-expired"));
            return Promise.reject(refreshError);
        }
    }
);

export default api;

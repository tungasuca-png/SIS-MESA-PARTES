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

// Sincronización de permisos tras un 403 (permiso insuficiente, sesión
// válida). Un 403 NO implica logout/redirect — solo indica que el estado
// local de permisos puede estar desactualizado (p. ej. alguien lo revocó
// desde Roles y Permisos mientras la app seguía abierta).
const PERMISSIONS_ENDPOINT = "/api/auth/me/permissions";
let permissionsRefreshPromise = null;

// Petición sin interceptores (mismo criterio que requestNewAccessToken):
// evita que un 403/401 de esta MISMA llamada vuelva a entrar en este flujo.
const requestMyPermissions = async () => {
    const accessToken = getAccessToken();
    const response = await axios.get(
        `${import.meta.env.VITE_API_URL}${PERMISSIONS_ENDPOINT}`,
        accessToken ? { headers: { Authorization: `Bearer ${accessToken}` } } : undefined
    );
    return response.data.permissions ?? [];
};

// Como máximo UNA sincronización activa a la vez: si ya hay una en curso,
// las siguientes llamadas (varios 403 casi simultáneos) reutilizan la misma
// promesa en vez de disparar otra petición.
function refreshPermissionsOnce() {
    if (!permissionsRefreshPromise) {
        permissionsRefreshPromise = requestMyPermissions()
            .then((permissions) => {
                window.dispatchEvent(
                    new CustomEvent("auth:permissions-refresh", { detail: { permissions } })
                );
            })
            .catch((refreshError) => {
                if (refreshError.response?.status === 401) {
                    // La sesión ya no es válida: mismo flujo que un 401
                    // normal, no un problema de permisos.
                    clearSession();
                    window.dispatchEvent(new Event("auth:session-expired"));
                }
                // 500/timeout/red: NO se tocan los permisos actuales — un
                // fallo de comunicación no demuestra que se perdió el permiso.
            })
            .finally(() => {
                permissionsRefreshPromise = null;
            });
    }
    return permissionsRefreshPromise;
}

api.interceptors.response.use(
    (response) => response,
    async (error) => {
        const originalRequest = error.config;
        const isAuthEndpoint =
            originalRequest?.url?.includes("/api/auth/login") ||
            originalRequest?.url?.includes("/api/auth/refresh");
        const isPermissionsEndpoint = originalRequest?.url?.includes(PERMISSIONS_ENDPOINT);

        if (error.response?.status === 403 && !isPermissionsEndpoint) {
            // Disparo en segundo plano: no se espera ni se reintenta la
            // petición original, solo se deja el estado sincronizado para
            // la siguiente interacción del usuario.
            refreshPermissionsOnce();
        }

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

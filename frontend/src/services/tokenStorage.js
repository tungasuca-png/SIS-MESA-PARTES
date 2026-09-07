const STORAGE_KEY = "mesa_partes_session";

export const getSession = () => {
    try {
        const raw = localStorage.getItem(STORAGE_KEY);
        return raw ? JSON.parse(raw) : null;
    } catch {
        return null;
    }
};

export const setSession = ({ accessToken, refreshToken, user }) => {
    try {
        localStorage.setItem(
            STORAGE_KEY,
            JSON.stringify({ accessToken, refreshToken, user })
        );
    } catch {
        // almacenamiento no disponible (modo privado, cuotas, etc.)
    }
};

export const clearSession = () => {
    try {
        localStorage.removeItem(STORAGE_KEY);
    } catch {
        // almacenamiento no disponible
    }
};

export const getAccessToken = () => getSession()?.accessToken ?? null;

export const getRefreshToken = () => getSession()?.refreshToken ?? null;

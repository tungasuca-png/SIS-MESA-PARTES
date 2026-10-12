import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { getMyPermissions, login as loginRequest, logout as logoutRequest, validateSession } from "../services/authService";
import { clearSession, getSession, setSession } from "../services/tokenStorage";
import { AuthContext } from "./auth-context";

// Permisos efectivos reales (Auth Service), nunca calculados desde
// user.role. Si la consulta falla, se degrada a []
// en vez de bloquear el login/restore — el backend sigue siendo la
// autoridad real en cada request, esto solo afecta qué se muestra.
async function fetchMyPermissions() {
    try {
        return await getMyPermissions();
    } catch {
        return [];
    }
}

export function AuthProvider({ children }) {
    const [user, setUser] = useState(null);
    const [permissions, setPermissions] = useState([]);
    const [loading, setLoading] = useState(true);
    const navigate = useNavigate();

    const login = useCallback(async (dni, password) => {
        const data = await loginRequest(dni, password);
        const userData = {
            id: data.user_id,
            username: data.username,
            email: data.email,
            role: data.role,
        };

        setSession({
            accessToken: data.access_token,
            refreshToken: data.refresh_token,
            user: userData,
        });
        setUser(userData);
        setPermissions(await fetchMyPermissions());

        return userData;
    }, []);

    const logout = useCallback(async () => {
        const session = getSession();

        try {
            if (session?.refreshToken) {
                await logoutRequest(session.refreshToken);
            }
        } catch {
            // El backend puede estar caído o el token ya inválido;
            // igual limpiamos la sesión local para no dejar al usuario atrapado.
        }

        clearSession();
        setUser(null);
        setPermissions([]);
        navigate("/login", { replace: true });
    }, [navigate]);

    useEffect(() => {
        const restoreSession = async () => {
            const session = getSession();
            if (!session?.accessToken || !session?.user) {
                setLoading(false);
                return;
            }

            // Sesión optimista mientras se confirma con el backend.
            setUser(session.user);

            try {
                await validateSession();
                // Los permisos NUNCA se persisten en localStorage: se piden
                // de nuevo a Auth Service en cada restauración de sesión,
                // para que una revocación real no quede congelada.
                setPermissions(await fetchMyPermissions());
            } catch {
                clearSession();
                setUser(null);
                setPermissions([]);
            } finally {
                setLoading(false);
            }
        };

        restoreSession();
    }, []);

    useEffect(() => {
        const handleSessionExpired = () => {
            setUser(null);
            setPermissions([]);
            navigate("/login", { replace: true });
        };

        window.addEventListener("auth:session-expired", handleSessionExpired);
        return () => window.removeEventListener("auth:session-expired", handleSessionExpired);
    }, [navigate]);

    useEffect(() => {
        // Disparado por api.js tras un 403 (ver requestMyPermissions/
        // refreshPermissionsOnce): NO implica logout, solo re-sincroniza
        // el estado local con lo que Auth Service diga en ese momento.
        const handlePermissionsRefresh = (event) => {
            setPermissions(event.detail?.permissions ?? []);
        };

        window.addEventListener("auth:permissions-refresh", handlePermissionsRefresh);
        return () => window.removeEventListener("auth:permissions-refresh", handlePermissionsRefresh);
    }, []);

    const isAuthenticated = Boolean(user);

    return (
        <AuthContext.Provider
            value={{
                user,
                permissions,
                isAuthenticated,
                loading,
                login,
                logout,
            }}
        >
            {children}
        </AuthContext.Provider>
    );
}

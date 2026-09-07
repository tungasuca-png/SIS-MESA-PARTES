import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { login as loginRequest, logout as logoutRequest, validateSession } from "../services/authService";
import { clearSession, getSession, setSession } from "../services/tokenStorage";
import { AuthContext } from "./auth-context";

export function AuthProvider({ children }) {
    const [user, setUser] = useState(null);
    const [loading, setLoading] = useState(true);
    const navigate = useNavigate();

    const login = useCallback(async (username, password) => {
        const data = await loginRequest(username, password);
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
            } catch {
                clearSession();
                setUser(null);
            } finally {
                setLoading(false);
            }
        };

        restoreSession();
    }, []);

    useEffect(() => {
        const handleSessionExpired = () => {
            setUser(null);
            navigate("/login", { replace: true });
        };

        window.addEventListener("auth:session-expired", handleSessionExpired);
        return () => window.removeEventListener("auth:session-expired", handleSessionExpired);
    }, [navigate]);

    const isAuthenticated = Boolean(user);

    return (
        <AuthContext.Provider
            value={{
                user,
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

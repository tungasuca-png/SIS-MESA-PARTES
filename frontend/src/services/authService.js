import api from "./api";

export const login = async (username, password) => {
    const response = await api.post("/api/auth/login", { username, password });
    return response.data;
};

export const logout = async (refreshToken) => {
    await api.post("/api/auth/logout", { refresh_token: refreshToken });
};

export const validateSession = async () => {
    const response = await api.get("/api/auth/validate");
    return response.data;
};

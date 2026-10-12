import { useAuth } from "./useAuth";

// Permisos efectivos REALES (Auth Service, via AuthContext.permissions), la
// única fuente de autorización del frontend: no existe ninguna matriz local
// por rol.
export function usePermissions() {
    const { permissions } = useAuth();
    const effectivePermissions = permissions ?? [];

    const can = (permission) => {
        if (!permission) return true;
        const required = Array.isArray(permission) ? permission : [permission];
        return required.some((code) => effectivePermissions.includes(code));
    };

    return { can, permissions: effectivePermissions };
}

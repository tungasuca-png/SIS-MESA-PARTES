import { useMemo } from "react";
import { useAuth } from "./useAuth";
import { getPermissionsForRole } from "../permissions/permissions";

export function usePermissions() {
    const { user } = useAuth();
    const permissions = useMemo(() => getPermissionsForRole(user?.role), [user?.role]);

    const can = (permission) => {
        if (!permission) return true;
        const required = Array.isArray(permission) ? permission : [permission];
        return required.some((code) => permissions.includes(code));
    };

    return { can, permissions };
}

import { useEffect, useMemo, useState } from "react";
import DashboardLayout from "../../../components/dashboard/DashboardLayout";
import Icon from "../../../components/dashboard/Icon";
import {
    getPermissions,
    getRolePermissions,
    getRoles,
    updateRolePermissions,
} from "../../../services/rolesService";
import { friendlyErrorMessage } from "../../../utils/apiErrors";
import "../../../components/dashboard/forms.css";
import "../../../components/dashboard/RecentExpedientes.css";
import "./RolesPermisos.css";

const ROLE_NOT_FOUND_MESSAGE = "El rol solicitado no existe.";

function capitalize(text) {
    if (!text) return text;
    return text.charAt(0).toUpperCase() + text.slice(1);
}

// Agrupa el catálogo real de permisos por su campo "modulo" (nunca una lista
// fija en el frontend): el orden de los grupos sigue el orden en que
// aparecen los módulos en la respuesta del backend.
function groupByModulo(permissions) {
    const map = new Map();
    for (const permission of permissions) {
        if (!map.has(permission.modulo)) {
            map.set(permission.modulo, []);
        }
        map.get(permission.modulo).push(permission);
    }
    return map;
}

function symmetricDifferenceSize(setA, setB) {
    let count = 0;
    for (const value of setA) {
        if (!setB.has(value)) count += 1;
    }
    for (const value of setB) {
        if (!setA.has(value)) count += 1;
    }
    return count;
}

function RolesPermisos() {
    const [roles, setRoles] = useState([]);
    const [permissions, setPermissions] = useState([]);
    const [selectedRoleId, setSelectedRoleId] = useState("");
    const [activeModule, setActiveModule] = useState("");

    const [assignedIds, setAssignedIds] = useState(new Set());
    const [originalIds, setOriginalIds] = useState(new Set());

    const [loadingCatalog, setLoadingCatalog] = useState(true);
    const [loadingRolePermissions, setLoadingRolePermissions] = useState(false);
    const [saving, setSaving] = useState(false);

    const [accessDenied, setAccessDenied] = useState(false);
    const [error, setError] = useState("");
    const [roleError, setRoleError] = useState("");
    const [success, setSuccess] = useState("");

    const [pendingRoleId, setPendingRoleId] = useState(null);

    // Catálogo (roles + permisos): una sola vez al entrar a la página.
    useEffect(() => {
        let active = true;

        async function loadCatalog() {
            setLoadingCatalog(true);
            setError("");
            setAccessDenied(false);
            try {
                const [rolesData, permissionsData] = await Promise.all([getRoles(), getPermissions()]);
                if (!active) return;
                setRoles(rolesData);
                setPermissions(permissionsData);
                if (rolesData.length > 0) {
                    setSelectedRoleId(rolesData[0].id);
                }
            } catch (err) {
                if (!active) return;
                if (err.response?.status === 403) {
                    setAccessDenied(true);
                } else {
                    setError(friendlyErrorMessage(err, ROLE_NOT_FOUND_MESSAGE));
                }
            } finally {
                if (active) setLoadingCatalog(false);
            }
        }

        loadCatalog();
        return () => {
            active = false;
        };
    }, []);

    // Permisos del rol seleccionado: se recarga por completo cada vez que
    // cambia selectedRoleId, para no arrastrar la selección del rol anterior.
    useEffect(() => {
        if (!selectedRoleId) return;
        let active = true;

        async function loadRolePermissions() {
            setLoadingRolePermissions(true);
            setRoleError("");
            setSuccess("");
            try {
                const rolePermissions = await getRolePermissions(selectedRoleId);
                if (!active) return;
                const ids = new Set(rolePermissions.map((permission) => permission.id));
                setOriginalIds(ids);
                setAssignedIds(new Set(ids));
            } catch (err) {
                if (!active) return;
                setRoleError(friendlyErrorMessage(err, ROLE_NOT_FOUND_MESSAGE));
                setOriginalIds(new Set());
                setAssignedIds(new Set());
            } finally {
                if (active) setLoadingRolePermissions(false);
            }
        }

        loadRolePermissions();
        return () => {
            active = false;
        };
    }, [selectedRoleId]);

    const groupedPermissions = useMemo(() => groupByModulo(permissions), [permissions]);
    const moduleNames = useMemo(() => [...groupedPermissions.keys()], [groupedPermissions]);
    const visibleModules = activeModule ? [activeModule] : moduleNames;

    const changesCount = useMemo(
        () => symmetricDifferenceSize(originalIds, assignedIds),
        [originalIds, assignedIds]
    );
    const hasChanges = changesCount > 0;

    const selectedRole = roles.find((role) => role.id === selectedRoleId) || null;

    const handleRoleChange = (event) => {
        const newRoleId = event.target.value;
        if (newRoleId === selectedRoleId) return;

        if (hasChanges) {
            setPendingRoleId(newRoleId);
            return;
        }
        setSelectedRoleId(newRoleId);
    };

    const confirmDiscardAndSwitch = () => {
        setSelectedRoleId(pendingRoleId);
        setPendingRoleId(null);
    };

    const cancelRoleSwitch = () => {
        setPendingRoleId(null);
    };

    const togglePermission = (permissionId) => {
        setSuccess("");
        setAssignedIds((current) => {
            const next = new Set(current);
            if (next.has(permissionId)) {
                next.delete(permissionId);
            } else {
                next.add(permissionId);
            }
            return next;
        });
    };

    const handleCancelChanges = () => {
        setAssignedIds(new Set(originalIds));
        setSuccess("");
        setRoleError("");
    };

    const handleSave = async () => {
        setSaving(true);
        setRoleError("");
        setSuccess("");
        try {
            const result = await updateRolePermissions(selectedRoleId, [...assignedIds]);
            const confirmedIds = new Set(result.permission_ids || []);
            setOriginalIds(confirmedIds);
            setAssignedIds(confirmedIds);
            setSuccess("Permisos actualizados correctamente.");
        } catch (err) {
            setRoleError(friendlyErrorMessage(err, ROLE_NOT_FOUND_MESSAGE));
        } finally {
            setSaving(false);
        }
    };

    if (accessDenied) {
        return (
            <DashboardLayout title="Roles y permisos">
                <section className="dp-panel rp-denied">
                    <h2 className="dp-panel-title">Acceso denegado</h2>
                    <p>No tienes permisos para administrar roles y permisos.</p>
                </section>
            </DashboardLayout>
        );
    }

    return (
        <DashboardLayout title="Roles y permisos">
            <section className="dp-panel rp-panel">
                <div className="rp-header">
                    <h2 className="dp-panel-title">Roles y permisos</h2>
                    <p className="rp-subtitle">Administra los permisos asignados a cada rol.</p>
                </div>

                {loadingCatalog && <p className="dp-table-empty">Cargando roles... Cargando permisos...</p>}

                {!loadingCatalog && error && <p className="dp-form-error">{error}</p>}

                {!loadingCatalog && !error && (
                    <>
                        <div className="rp-role-select">
                            <label htmlFor="rp-role">Rol</label>
                            <select
                                id="rp-role"
                                value={selectedRoleId}
                                onChange={handleRoleChange}
                                disabled={saving}
                            >
                                {roles.map((role) => (
                                    <option key={role.id} value={role.id}>
                                        {role.nombre}
                                    </option>
                                ))}
                            </select>
                            {selectedRole?.descripcion && (
                                <p className="rp-role-description">{selectedRole.descripcion}</p>
                            )}
                        </div>

                        {loadingRolePermissions && <p className="dp-table-empty">Cargando permisos...</p>}

                        {!loadingRolePermissions && roleError && <p className="dp-form-error">{roleError}</p>}

                        {!loadingRolePermissions && !roleError && (
                            <>
                                <div className="rp-summary">
                                    <span className="rp-counter">
                                        {assignedIds.size} de {permissions.length} permisos asignados
                                    </span>
                                    {hasChanges && (
                                        <span className="rp-pending-badge">
                                            {changesCount} cambio{changesCount === 1 ? "" : "s"} pendiente
                                            {changesCount === 1 ? "" : "s"}
                                        </span>
                                    )}
                                </div>

                                {moduleNames.length > 1 && (
                                    <div className="rp-module-tabs">
                                        <button
                                            type="button"
                                            className={`rp-module-tab ${activeModule === "" ? "rp-module-tab--active" : ""}`}
                                            onClick={() => setActiveModule("")}
                                        >
                                            Todos
                                        </button>
                                        {moduleNames.map((modulo) => (
                                            <button
                                                key={modulo}
                                                type="button"
                                                className={`rp-module-tab ${activeModule === modulo ? "rp-module-tab--active" : ""}`}
                                                onClick={() => setActiveModule(modulo)}
                                            >
                                                {capitalize(modulo)}
                                            </button>
                                        ))}
                                    </div>
                                )}

                                <div className="rp-permission-list">
                                    {visibleModules.map((modulo) => (
                                        <div key={modulo} className="rp-permission-group">
                                            <h3 className="rp-permission-group-title">{capitalize(modulo)}</h3>
                                            {groupedPermissions.get(modulo).map((permission) => (
                                                <label key={permission.id} className="rp-permission-row">
                                                    <span className="rp-toggle">
                                                        <input
                                                            type="checkbox"
                                                            checked={assignedIds.has(permission.id)}
                                                            onChange={() => togglePermission(permission.id)}
                                                            disabled={saving}
                                                        />
                                                        <span className="rp-toggle-track">
                                                            <span className="rp-toggle-thumb" />
                                                        </span>
                                                    </span>
                                                    <span className="rp-permission-text">
                                                        <span className="rp-permission-name">{permission.nombre}</span>
                                                        <span className="rp-permission-code">{permission.codigo}</span>
                                                        {permission.descripcion && (
                                                            <span className="rp-permission-desc">{permission.descripcion}</span>
                                                        )}
                                                    </span>
                                                </label>
                                            ))}
                                        </div>
                                    ))}
                                </div>

                                {success && <p className="rp-success">{success}</p>}

                                <div className="dp-form-actions rp-actions">
                                    <button
                                        type="button"
                                        className="dp-btn-secondary"
                                        onClick={handleCancelChanges}
                                        disabled={!hasChanges || saving}
                                    >
                                        Cancelar
                                    </button>
                                    <button
                                        type="button"
                                        className="dp-btn-primary"
                                        onClick={handleSave}
                                        disabled={!hasChanges || saving}
                                    >
                                        {saving ? "Guardando..." : "Guardar cambios"}
                                    </button>
                                </div>
                            </>
                        )}
                    </>
                )}
            </section>

            {pendingRoleId && (
                <div className="dp-modal-overlay" onClick={cancelRoleSwitch}>
                    <div
                        className="dp-modal"
                        role="dialog"
                        aria-modal="true"
                        aria-label="Cambios sin guardar"
                        onClick={(event) => event.stopPropagation()}
                    >
                        <div className="dp-modal-header">
                            <h2>Cambios sin guardar</h2>
                            <button
                                type="button"
                                className="dp-modal-close"
                                onClick={cancelRoleSwitch}
                                aria-label="Cerrar"
                            >
                                <Icon name="close" size={18} />
                            </button>
                        </div>
                        <p>Tienes cambios sin guardar. ¿Deseas descartarlos?</p>
                        <div className="dp-form-actions">
                            <button type="button" className="dp-btn-secondary" onClick={cancelRoleSwitch}>
                                Cancelar
                            </button>
                            <button type="button" className="dp-btn-primary" onClick={confirmDiscardAndSwitch}>
                                Descartar cambios
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </DashboardLayout>
    );
}

export default RolesPermisos;

import { useEffect, useMemo, useState } from "react";
import DashboardLayout from "../../../components/dashboard/DashboardLayout";
import Icon from "../../../components/dashboard/Icon";
import { useAuth } from "../../../hooks/useAuth";
import { usePermissions } from "../../../hooks/usePermissions";
import { createUsuario, listUsuarios, updateUsuarioRol } from "../../../services/usuariosService";
import { getRoles } from "../../../services/rolesService";
import { getMyPermissions } from "../../../services/authService";
import { extractBackendMessage, friendlyErrorMessage } from "../../../utils/apiErrors";
import "../../../components/dashboard/forms.css";
import "../../../components/dashboard/RecentExpedientes.css";
import "./Usuarios.css";

const USERS_NOT_FOUND_MESSAGE = "El usuario no fue encontrado.";

const EMPTY_CREATE_FORM = {
    username: "",
    email: "",
    password: "",
    confirmPassword: "",
    nombres: "",
    apellidos: "",
    dni: "",
    telefono: "",
    direccion: "",
    roleId: "",
};

// Mapea la respuesta de error de crear un usuario a un mensaje fijo en
// español. Mismo criterio que roleChangeErrorMessage: nunca el texto crudo
// del backend para el 403, y nunca cerrar sesión.
function createUserErrorMessage(err) {
    if (!err?.response) {
        return "No se pudo conectar con el servidor. Verifica tu conexión e intenta nuevamente.";
    }
    const { status, data } = err.response;
    switch (status) {
        case 400:
            return extractBackendMessage(data) || "Los datos ingresados no son válidos.";
        case 403:
            return "El usuario no tiene el permiso requerido";
        case 404:
            return "El rol seleccionado no existe.";
        case 409:
            return extractBackendMessage(data) || "El usuario, el email o el DNI ya están registrados.";
        default:
            return "Ocurrió un error al crear el usuario. Inténtalo nuevamente.";
    }
}

// Validación de frontend (UX temprana) — nunca reemplaza la del backend,
// que sigue siendo la única fuente real de verdad (username único, rol
// existente/activo, etc.).
function validateCreateForm(form) {
    const errors = {};
    if (!form.username.trim()) {
        errors.username = "El username es obligatorio.";
    } else if (form.username.trim().length < 3) {
        errors.username = "El username debe tener al menos 3 caracteres.";
    }
    if (!form.email.trim()) {
        errors.email = "El email es obligatorio.";
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email.trim())) {
        errors.email = "El email no tiene un formato válido.";
    }
    if (!form.password) {
        errors.password = "La contraseña es obligatoria.";
    } else if (form.password.length < 8) {
        errors.password = "La contraseña debe tener al menos 8 caracteres.";
    }
    if (form.confirmPassword !== form.password) {
        errors.confirmPassword = "Las contraseñas no coinciden.";
    }
    if (!form.nombres.trim()) {
        errors.nombres = "Los nombres son obligatorios.";
    }
    if (!form.apellidos.trim()) {
        errors.apellidos = "Los apellidos son obligatorios.";
    }
    if (!form.dni.trim()) {
        errors.dni = "El DNI es obligatorio.";
    } else if (!/^[0-9]{8}$/.test(form.dni.trim())) {
        errors.dni = "El DNI debe tener 8 dígitos numéricos.";
    }
    if (!form.roleId) {
        errors.roleId = "Debes seleccionar un rol.";
    }
    return errors;
}

// Mapea la respuesta de error de un cambio de rol a un mensaje fijo en
// español. Deliberadamente NO reutiliza friendlyErrorMessage para el 403:
// esa función muestra el texto crudo del backend ("permiso insuficiente"),
// pero esta pantalla exige mostrar exactamente "El usuario no tiene el
// permiso requerido" (mismo texto que PermissionRoute) y nunca cerrar sesión.
function roleChangeErrorMessage(err) {
    if (!err?.response) {
        return "No se pudo conectar con el servidor. Verifica tu conexión e intenta nuevamente.";
    }
    const { status, data } = err.response;
    switch (status) {
        case 400:
            return extractBackendMessage(data) || "Los datos ingresados no son válidos.";
        case 403:
            return "El usuario no tiene el permiso requerido";
        case 404:
            return "El usuario o el rol no existe.";
        default:
            return "Ocurrió un error al actualizar el rol. Inténtalo nuevamente.";
    }
}

function nombreCompleto(usuario) {
    return [usuario.nombres, usuario.apellidos].filter(Boolean).join(" ").trim() || "(sin nombre)";
}

function Usuarios() {
    const { user } = useAuth();
    const { can } = usePermissions();
    const canChangeRole = can("usuarios.change_role");

    const [usuarios, setUsuarios] = useState([]);
    const [roles, setRoles] = useState([]);
    const [loadingCatalog, setLoadingCatalog] = useState(true);
    const [accessDenied, setAccessDenied] = useState(false);
    const [loadError, setLoadError] = useState("");

    const [searchInput, setSearchInput] = useState("");
    const [search, setSearch] = useState("");
    // Se incrementa tras crear un usuario para forzar un nuevo GET
    // /api/usuarios (sin window.location.reload()) — la lista siempre
    // refleja el backend real, nunca un dato insertado a mano en el estado.
    const [reloadToken, setReloadToken] = useState(0);

    // Rol seleccionado (pendiente de guardar) por usuario, distinto del rol
    // actual real hasta que se confirme la respuesta del backend.
    const [pendingRoleByUserId, setPendingRoleByUserId] = useState({});
    const [savingUserId, setSavingUserId] = useState(null);
    const [rowError, setRowError] = useState({});
    const [rowSuccess, setRowSuccess] = useState({});

    const [showCreateModal, setShowCreateModal] = useState(false);
    const [createForm, setCreateForm] = useState(EMPTY_CREATE_FORM);
    const [createFieldErrors, setCreateFieldErrors] = useState({});
    const [createSubmitError, setCreateSubmitError] = useState("");
    const [creating, setCreating] = useState(false);
    const [createdMessage, setCreatedMessage] = useState("");

    // Debounce simple: espera a que el usuario deje de escribir antes de
    // consultar al backend (GET /api/usuarios?q=...), sin filtrar en el
    // cliente ni mantener una copia propia del catálogo.
    useEffect(() => {
        const timeout = setTimeout(() => setSearch(searchInput.trim()), 300);
        return () => clearTimeout(timeout);
    }, [searchInput]);

    useEffect(() => {
        let active = true;

        async function loadCatalog() {
            setLoadingCatalog(true);
            setLoadError("");
            setAccessDenied(false);
            try {
                // getRoles() exige "roles.view" (permiso propio de Roles ->
                // Permisos, distinto de "usuarios.change_role"): solo se pide
                // cuando realmente se va a poder editar, para que un usuario
                // con "usuarios.view" pero sin "roles.view" pueda ver la
                // lista igual (rol actual como texto, sin selector).
                const [usuariosData, rolesData] = await Promise.all([
                    listUsuarios({ q: search }),
                    canChangeRole ? getRoles() : Promise.resolve([]),
                ]);
                if (!active) return;
                setUsuarios(usuariosData.usuarios || []);
                setRoles(rolesData);
            } catch (err) {
                if (!active) return;
                if (err.response?.status === 403) {
                    setAccessDenied(true);
                } else {
                    setLoadError(friendlyErrorMessage(err, USERS_NOT_FOUND_MESSAGE));
                }
            } finally {
                if (active) setLoadingCatalog(false);
            }
        }

        loadCatalog();
        return () => {
            active = false;
        };
    }, [search, canChangeRole, reloadToken]);

    const roleIdByName = useMemo(() => {
        const map = new Map();
        for (const role of roles) {
            map.set(role.nombre, role.id);
        }
        return map;
    }, [roles]);

    const handleRoleSelect = (userId, roleId) => {
        setPendingRoleByUserId((current) => ({ ...current, [userId]: roleId }));
        setRowError((current) => ({ ...current, [userId]: "" }));
        setRowSuccess((current) => ({ ...current, [userId]: "" }));
    };

    const handleSave = async (usuario) => {
        const currentRoleId = roleIdByName.get(usuario.tipo_usuario) || "";
        const selectedRoleId = pendingRoleByUserId[usuario.id] ?? currentRoleId;
        if (!selectedRoleId || selectedRoleId === currentRoleId) return;

        setSavingUserId(usuario.id);
        setRowError((current) => ({ ...current, [usuario.id]: "" }));
        setRowSuccess((current) => ({ ...current, [usuario.id]: "" }));
        try {
            const result = await updateUsuarioRol(usuario.id, selectedRoleId);

            setUsuarios((current) =>
                current.map((item) =>
                    item.id === usuario.id ? { ...item, tipo_usuario: result.role_nombre } : item
                )
            );
            setPendingRoleByUserId((current) => {
                const next = { ...current };
                delete next[usuario.id];
                return next;
            });
            setRowSuccess((current) => ({ ...current, [usuario.id]: "Rol actualizado correctamente." }));

            // Caso especial: el ADMIN cambia su PROPIO rol. El JWT ya emitido
            // puede seguir mostrando el rol anterior hasta el próximo login,
            // pero los permisos reales (BD) ya cambiaron — se resincronizan
            // con el mismo mecanismo que ya usa AuthContext tras un 403
            // (evento "auth:permissions-refresh"), sin tocar localStorage ni
            // el JWT manualmente.
            if (user?.id === usuario.id) {
                try {
                    const permissions = await getMyPermissions();
                    window.dispatchEvent(
                        new CustomEvent("auth:permissions-refresh", { detail: { permissions } })
                    );
                } catch {
                    // Si esta resincronización puntual falla, el interceptor
                    // de api.js ya vuelve a intentarlo ante el próximo 403.
                }
            }
        } catch (err) {
            setRowError((current) => ({ ...current, [usuario.id]: roleChangeErrorMessage(err) }));
        } finally {
            setSavingUserId(null);
        }
    };

    const openCreateModal = () => {
        setCreateForm(EMPTY_CREATE_FORM);
        setCreateFieldErrors({});
        setCreateSubmitError("");
        setShowCreateModal(true);
    };

    const closeCreateModal = () => {
        if (creating) return;
        setShowCreateModal(false);
    };

    const updateCreateField = (field) => (event) => {
        setCreateForm((current) => ({ ...current, [field]: event.target.value }));
        setCreateFieldErrors((current) => ({ ...current, [field]: "" }));
    };

    const handleCreateSubmit = async (event) => {
        event.preventDefault();
        const errors = validateCreateForm(createForm);
        setCreateFieldErrors(errors);
        if (Object.keys(errors).length > 0) return;

        setCreating(true);
        setCreateSubmitError("");
        try {
            await createUsuario({
                username: createForm.username.trim(),
                email: createForm.email.trim(),
                password: createForm.password,
                roleId: createForm.roleId,
                nombres: createForm.nombres.trim(),
                apellidos: createForm.apellidos.trim(),
                dni: createForm.dni.trim(),
                telefono: createForm.telefono.trim(),
                direccion: createForm.direccion.trim(),
            });
            setShowCreateModal(false);
            setCreateForm(EMPTY_CREATE_FORM);
            setCreatedMessage("Usuario creado correctamente.");
            setReloadToken((token) => token + 1);
        } catch (err) {
            setCreateSubmitError(createUserErrorMessage(err));
        } finally {
            setCreating(false);
        }
    };

    if (accessDenied) {
        return (
            <DashboardLayout title="Usuarios">
                <section className="dp-panel us-denied">
                    <h2 className="dp-panel-title">Acceso denegado</h2>
                    <p>No tienes permisos para administrar usuarios.</p>
                </section>
            </DashboardLayout>
        );
    }

    return (
        <DashboardLayout title="Usuarios">
            <section className="dp-panel us-panel">
                <div className="us-header">
                    <div>
                        <h2 className="dp-panel-title">Usuarios</h2>
                        <p className="us-subtitle">Administra los usuarios y asigna su rol.</p>
                    </div>
                    {canChangeRole && (
                        <button type="button" className="dp-btn-primary" onClick={openCreateModal}>
                            + Nuevo usuario
                        </button>
                    )}
                </div>

                <div className="us-search">
                    <input
                        type="text"
                        placeholder="Buscar usuario..."
                        value={searchInput}
                        onChange={(event) => {
                            setSearchInput(event.target.value);
                            setCreatedMessage("");
                        }}
                    />
                </div>

                {createdMessage && <p className="us-banner-success">{createdMessage}</p>}

                {loadingCatalog && <p className="dp-table-empty">Cargando usuarios... Cargando roles...</p>}

                {!loadingCatalog && loadError && <p className="dp-form-error">{loadError}</p>}

                {!loadingCatalog && !loadError && usuarios.length === 0 && (
                    <p className="dp-table-empty">No se encontraron usuarios.</p>
                )}

                {!loadingCatalog && !loadError && usuarios.length > 0 && (
                    <div className="dp-table-wrapper">
                        <table className="dp-table">
                            <thead>
                                <tr>
                                    <th>Nombre</th>
                                    <th>Correo</th>
                                    <th>Estado</th>
                                    <th>Rol actual</th>
                                    {canChangeRole && <th>Acción</th>}
                                </tr>
                            </thead>
                            <tbody>
                                {usuarios.map((usuario) => {
                                    const currentRoleId = roleIdByName.get(usuario.tipo_usuario) || "";
                                    const selectedRoleId = pendingRoleByUserId[usuario.id] ?? currentRoleId;
                                    const hasChange = Boolean(selectedRoleId) && selectedRoleId !== currentRoleId;
                                    const saving = savingUserId === usuario.id;

                                    return (
                                        <tr key={usuario.id}>
                                            <td>{nombreCompleto(usuario)}</td>
                                            <td>{usuario.correo || "—"}</td>
                                            <td>
                                                <span
                                                    className={
                                                        usuario.activo ? "us-badge us-badge--activo" : "us-badge us-badge--inactivo"
                                                    }
                                                >
                                                    {usuario.activo ? "Activo" : "Inactivo"}
                                                </span>
                                            </td>
                                            <td>
                                                {canChangeRole ? (
                                                    <select
                                                        value={selectedRoleId}
                                                        onChange={(event) => handleRoleSelect(usuario.id, event.target.value)}
                                                        disabled={saving || roles.length === 0}
                                                    >
                                                        {roles.map((role) => (
                                                            <option key={role.id} value={role.id}>
                                                                {role.nombre}
                                                            </option>
                                                        ))}
                                                    </select>
                                                ) : (
                                                    <span>{usuario.tipo_usuario}</span>
                                                )}
                                            </td>
                                            {canChangeRole && (
                                                <td>
                                                    <div className="us-row-action">
                                                        <button
                                                            type="button"
                                                            className="dp-btn-primary"
                                                            onClick={() => handleSave(usuario)}
                                                            disabled={!hasChange || saving}
                                                        >
                                                            {saving ? "Guardando..." : "Guardar"}
                                                        </button>
                                                        {rowError[usuario.id] && (
                                                            <p className="dp-form-error us-row-message">{rowError[usuario.id]}</p>
                                                        )}
                                                        {rowSuccess[usuario.id] && (
                                                            <p className="us-success us-row-message">{rowSuccess[usuario.id]}</p>
                                                        )}
                                                    </div>
                                                </td>
                                            )}
                                        </tr>
                                    );
                                })}
                            </tbody>
                        </table>
                    </div>
                )}
            </section>

            {showCreateModal && (
                <div className="dp-modal-overlay" onClick={closeCreateModal}>
                    <div
                        className="dp-modal us-create-modal"
                        role="dialog"
                        aria-modal="true"
                        aria-label="Nuevo usuario"
                        onClick={(event) => event.stopPropagation()}
                    >
                        <div className="dp-modal-header">
                            <h2>Nuevo usuario</h2>
                            <button
                                type="button"
                                className="dp-modal-close"
                                onClick={closeCreateModal}
                                aria-label="Cerrar"
                                disabled={creating}
                            >
                                <Icon name="close" size={18} />
                            </button>
                        </div>

                        <form className="dp-form" onSubmit={handleCreateSubmit}>
                            <div className="dp-form-group">
                                <label htmlFor="cu-username">Username *</label>
                                <input
                                    id="cu-username"
                                    type="text"
                                    value={createForm.username}
                                    onChange={updateCreateField("username")}
                                    disabled={creating}
                                    autoComplete="off"
                                />
                                {createFieldErrors.username && (
                                    <p className="dp-form-error">{createFieldErrors.username}</p>
                                )}
                            </div>

                            <div className="dp-form-group">
                                <label htmlFor="cu-email">Email *</label>
                                <input
                                    id="cu-email"
                                    type="email"
                                    value={createForm.email}
                                    onChange={updateCreateField("email")}
                                    disabled={creating}
                                    autoComplete="off"
                                />
                                {createFieldErrors.email && <p className="dp-form-error">{createFieldErrors.email}</p>}
                            </div>

                            <div className="us-create-row">
                                <div className="dp-form-group">
                                    <label htmlFor="cu-password">Contraseña *</label>
                                    <input
                                        id="cu-password"
                                        type="password"
                                        value={createForm.password}
                                        onChange={updateCreateField("password")}
                                        disabled={creating}
                                        autoComplete="new-password"
                                    />
                                    {createFieldErrors.password && (
                                        <p className="dp-form-error">{createFieldErrors.password}</p>
                                    )}
                                </div>

                                <div className="dp-form-group">
                                    <label htmlFor="cu-confirm-password">Confirmar contraseña *</label>
                                    <input
                                        id="cu-confirm-password"
                                        type="password"
                                        value={createForm.confirmPassword}
                                        onChange={updateCreateField("confirmPassword")}
                                        disabled={creating}
                                        autoComplete="new-password"
                                    />
                                    {createFieldErrors.confirmPassword && (
                                        <p className="dp-form-error">{createFieldErrors.confirmPassword}</p>
                                    )}
                                </div>
                            </div>

                            <div className="us-create-row">
                                <div className="dp-form-group">
                                    <label htmlFor="cu-nombres">Nombres *</label>
                                    <input
                                        id="cu-nombres"
                                        type="text"
                                        value={createForm.nombres}
                                        onChange={updateCreateField("nombres")}
                                        disabled={creating}
                                    />
                                    {createFieldErrors.nombres && (
                                        <p className="dp-form-error">{createFieldErrors.nombres}</p>
                                    )}
                                </div>

                                <div className="dp-form-group">
                                    <label htmlFor="cu-apellidos">Apellidos *</label>
                                    <input
                                        id="cu-apellidos"
                                        type="text"
                                        value={createForm.apellidos}
                                        onChange={updateCreateField("apellidos")}
                                        disabled={creating}
                                    />
                                    {createFieldErrors.apellidos && (
                                        <p className="dp-form-error">{createFieldErrors.apellidos}</p>
                                    )}
                                </div>
                            </div>

                            <div className="us-create-row">
                                <div className="dp-form-group">
                                    <label htmlFor="cu-dni">DNI (credencial de ingreso)</label>
                                    <input
                                        id="cu-dni"
                                        type="text"
                                        inputMode="numeric"
                                        maxLength={8}
                                        value={createForm.dni}
                                        onChange={updateCreateField("dni")}
                                        disabled={creating}
                                    />
                                    {createFieldErrors.dni && (
                                        <p className="dp-form-error">{createFieldErrors.dni}</p>
                                    )}
                                </div>

                                <div className="dp-form-group">
                                    <label htmlFor="cu-telefono">Teléfono</label>
                                    <input
                                        id="cu-telefono"
                                        type="text"
                                        value={createForm.telefono}
                                        onChange={updateCreateField("telefono")}
                                        disabled={creating}
                                    />
                                </div>
                            </div>

                            <div className="dp-form-group">
                                <label htmlFor="cu-direccion">Dirección</label>
                                <input
                                    id="cu-direccion"
                                    type="text"
                                    value={createForm.direccion}
                                    onChange={updateCreateField("direccion")}
                                    disabled={creating}
                                />
                            </div>

                            <div className="dp-form-group">
                                <label htmlFor="cu-rol">Rol *</label>
                                <select
                                    id="cu-rol"
                                    value={createForm.roleId}
                                    onChange={updateCreateField("roleId")}
                                    disabled={creating || roles.length === 0}
                                >
                                    <option value="">Selecciona un rol...</option>
                                    {roles.map((role) => (
                                        <option key={role.id} value={role.id}>
                                            {role.nombre}
                                        </option>
                                    ))}
                                </select>
                                {createFieldErrors.roleId && (
                                    <p className="dp-form-error">{createFieldErrors.roleId}</p>
                                )}
                            </div>

                            {createSubmitError && <p className="dp-form-error">{createSubmitError}</p>}

                            <div className="dp-form-actions">
                                <button
                                    type="button"
                                    className="dp-btn-secondary"
                                    onClick={closeCreateModal}
                                    disabled={creating}
                                >
                                    Cancelar
                                </button>
                                <button type="submit" className="dp-btn-primary" disabled={creating}>
                                    {creating ? "Creando..." : "Crear usuario"}
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}
        </DashboardLayout>
    );
}

export default Usuarios;

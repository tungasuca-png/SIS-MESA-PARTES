import { useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import RoleLayout from "../../components/portal/RoleLayout";
import Icon from "../../components/dashboard/Icon";
import EstadoEstudianteBadge from "../../components/estudiantes/EstadoEstudianteBadge";
import EstudianteFormModal from "../../components/estudiantes/EstudianteFormModal";
import CambiarEstadoModal from "../../components/estudiantes/CambiarEstadoModal";
import NominaPeriodo from "../../components/estudiantes/NominaPeriodo";
import { usePermissions } from "../../hooks/usePermissions";
import { searchEstudiantes } from "../../services/estudiantesService";
import {
    BUSQUEDA_MAX,
    ESTADOS_ESTUDIANTE,
    ESTUDIANTES_PAGE_SIZE,
    FILTRO_MAX,
    ORDENES_ESTUDIANTES,
    tipoDocumentoLabel,
} from "../../constants/estudiantes";
import { validarBusqueda } from "../../utils/estudianteForm";
import { estudianteError } from "../../utils/estudiantesErrors";
import { enmascararDocumento } from "../../utils/privacidad";
import "../../components/dashboard/RecentExpedientes.css";
import "../../components/dashboard/forms.css";
import "../Expedientes/expedientesList.css";
import "./estudiantes.css";

const FILTROS_VACIOS = { estado: "", nivel: "", grado: "", seccion: "", turno: "", orden: "" };

// Nivel/grado/sección/turno: el backend todavía no tiene catálogos (son
// columnas de texto temporales hasta Matrícula), así que se filtran con
// texto libre por coincidencia exacta — no se inventan listas de valores.
const FILTROS_TEXTO = [
    { key: "nivel", label: "Nivel", placeholder: "Nivel" },
    { key: "grado", label: "Grado", placeholder: "Grado" },
    { key: "seccion", label: "Sección", placeholder: "Sección" },
    { key: "turno", label: "Turno", placeholder: "Turno" },
];

const criteriosDe = (q, filtros) => ({
    q: q.trim(),
    ...Object.fromEntries(Object.entries(filtros).map(([k, v]) => [k, v.trim()])),
});

// Directorio del Maestro de Estudiantes (FASE E-06). Búsqueda, filtros,
// paginación y orden los resuelve el backend (GET /api/estudiantes, orden
// fijo apellidos/nombres/id); aquí no se filtra ni se ordena nada.
function EstudiantesList() {
    const navigate = useNavigate();
    // ?q= viene del buscador de la barra superior.
    const [searchParams] = useSearchParams();
    const qInicial = searchParams.get("q") || "";
    const [vista, setVista] = useState("directorio");
    const { can } = usePermissions();
    const canCreate = can("estudiantes.create");
    const canUpdate = can("estudiantes.update");
    const canChangeEstado = can("estudiantes.change_estado");

    const [searchInput, setSearchInput] = useState(qInicial);
    const [filtrosInput, setFiltrosInput] = useState(FILTROS_VACIOS);
    // Criterios aplicados y página en UN estado: cambiar un criterio vuelve
    // a la página 1 en la misma actualización (una sola consulta).
    const [consulta, setConsulta] = useState(() => ({ criterios: criteriosDe(qInicial, FILTROS_VACIOS), page: 1 }));
    const { criterios, page } = consulta;
    const setPage = (updater) => setConsulta((current) => ({ ...current, page: updater(current.page) }));
    const [reloadToken, setReloadToken] = useState(0);

    // Resultado de la última consulta, marcado con la clave de la solicitud
    // que lo produjo: "cargando" = la clave actual todavía no tiene respuesta.
    const [resultado, setResultado] = useState({ key: null, items: [], total: 0, error: null });

    const [showCreate, setShowCreate] = useState(false);
    const [editando, setEditando] = useState(null);
    const [cambiandoEstado, setCambiandoEstado] = useState(null);
    const [mensaje, setMensaje] = useState("");

    // Nueva búsqueda desde la barra superior con la página ya abierta.
    const [qPrevia, setQPrevia] = useState(qInicial);
    if (qInicial !== qPrevia) {
        setQPrevia(qInicial);
        setSearchInput(qInicial);
        setVista("directorio");
    }

    // Debounce: una sola consulta cuando el usuario deja de escribir. Si los
    // criterios no cambiaron, se conserva el mismo objeto (sin re-consultar).
    useEffect(() => {
        const timeout = setTimeout(() => {
            const next = criteriosDe(searchInput, filtrosInput);
            setConsulta((current) =>
                JSON.stringify(current.criterios) === JSON.stringify(next) ? current : { criterios: next, page: 1 }
            );
        }, 300);
        return () => clearTimeout(timeout);
    }, [searchInput, filtrosInput]);

    const busquedaInvalida = validarBusqueda(criterios.q);
    const requestKey = JSON.stringify({ criterios, page, reloadToken });

    useEffect(() => {
        // Búsqueda inválida (p. ej. 1 carácter): no se consulta al backend.
        if (busquedaInvalida) return undefined;
        let active = true;
        searchEstudiantes({ ...criterios, page, pageSize: ESTUDIANTES_PAGE_SIZE })
            .then((data) => {
                if (active) {
                    setResultado({ key: requestKey, items: data.estudiantes || [], total: data.total || 0, error: null });
                }
            })
            .catch((err) => {
                if (active) setResultado({ key: requestKey, items: [], total: 0, error: estudianteError(err) });
            });
        return () => {
            active = false;
        };
    }, [criterios, page, requestKey, busquedaInvalida]);

    const loading = !busquedaInvalida && resultado.key !== requestKey;
    const vigente = !busquedaInvalida && !loading;
    const items = vigente ? resultado.items : [];
    const total = vigente ? resultado.total : 0;
    const error = vigente ? resultado.error : null;

    const totalPages = Math.max(1, Math.ceil(total / ESTUDIANTES_PAGE_SIZE));
    const hayCriterios = Object.entries(criterios).some(([k, v]) => k !== "orden" && Boolean(v));

    const reemplazar = (estudiante) => {
        setResultado((current) => ({
            ...current,
            items: current.items.map((item) => (item.id === estudiante.id ? { ...item, ...estudiante } : item)),
        }));
    };

    const recargar = () => {
        setEditando(null);
        setCambiandoEstado(null);
        setReloadToken((token) => token + 1);
    };

    if (error?.kind === "forbidden") {
        return (
            <RoleLayout title="Estudiantes">
                <section className="dp-panel" style={{ textAlign: "center", padding: "40px 20px" }}>
                    <h2 className="dp-panel-title">Acceso denegado</h2>
                    <p>No tienes permiso para consultar estudiantes.</p>
                </section>
            </RoleLayout>
        );
    }

    return (
        <RoleLayout title="Estudiantes">
            <section className="dp-panel">
                <div className="dp-list-header">
                    <div>
                        <h2 className="dp-panel-title">Estudiantes</h2>
                        <p className="est-subtitle">Directorio institucional de estudiantes.</p>
                    </div>
                    {canCreate && (
                        <button type="button" className="dp-btn-primary" onClick={() => setShowCreate(true)}>
                            Registrar estudiante
                        </button>
                    )}
                </div>

                <div className="est-segmented est-vista" role="tablist" aria-label="Vista del directorio">
                    <button type="button" role="tab" aria-selected={vista === "directorio"} className={vista === "directorio" ? "is-active" : ""} onClick={() => setVista("directorio")}>
                        Directorio
                    </button>
                    <button type="button" role="tab" aria-selected={vista === "nomina"} className={vista === "nomina" ? "is-active" : ""} onClick={() => setVista("nomina")}>
                        Por periodo y sección
                    </button>
                </div>

                {vista === "nomina" && <NominaPeriodo />}
                {vista === "directorio" && (
                <>

                <div className="est-search">
                    <input
                        type="search"
                        aria-label="Buscar estudiantes"
                        placeholder="Buscar por código, SIAGIE, documento, nombres o apellidos..."
                        maxLength={BUSQUEDA_MAX}
                        value={searchInput}
                        onChange={(event) => {
                            setSearchInput(event.target.value);
                            setMensaje("");
                        }}
                    />
                    {busquedaInvalida && <p className="est-hint">{busquedaInvalida}</p>}
                </div>

                <div className="dp-filters est-filters">
                    <select
                        aria-label="Filtrar por estado"
                        value={filtrosInput.estado}
                        onChange={(event) => setFiltrosInput((f) => ({ ...f, estado: event.target.value }))}
                    >
                        <option value="">Todos los estados</option>
                        {ESTADOS_ESTUDIANTE.map((item) => (
                            <option key={item.value} value={item.value}>
                                {item.label}
                            </option>
                        ))}
                    </select>
                    {FILTROS_TEXTO.map((filtro) => (
                        <input
                            key={filtro.key}
                            type="text"
                            aria-label={`Filtrar por ${filtro.label.toLowerCase()}`}
                            placeholder={filtro.placeholder}
                            maxLength={FILTRO_MAX}
                            value={filtrosInput[filtro.key]}
                            onChange={(event) => setFiltrosInput((f) => ({ ...f, [filtro.key]: event.target.value }))}
                        />
                    ))}
                    <select
                        aria-label="Ordenar por"
                        value={filtrosInput.orden}
                        onChange={(event) => setFiltrosInput((f) => ({ ...f, orden: event.target.value }))}
                    >
                        {ORDENES_ESTUDIANTES.map((item) => (
                            <option key={item.value} value={item.value}>
                                Orden: {item.label}
                            </option>
                        ))}
                    </select>
                    {(searchInput || Object.values(filtrosInput).some(Boolean)) && (
                        <button
                            type="button"
                            className="dp-btn-secondary"
                            onClick={() => {
                                setSearchInput("");
                                setFiltrosInput(FILTROS_VACIOS);
                            }}
                        >
                            Limpiar
                        </button>
                    )}
                </div>

                {mensaje && <p className="est-banner-success">{mensaje}</p>}

                {loading && <p className="dp-table-empty">Cargando estudiantes...</p>}
                {!loading && error && <p className="dp-form-error">{error.message}</p>}
                {!loading && !error && !busquedaInvalida && items.length === 0 && (
                    <p className="dp-table-empty">
                        {hayCriterios
                            ? "No se encontraron estudiantes con esos criterios."
                            : "Todavía no hay estudiantes registrados."}
                    </p>
                )}

                {!loading && !error && items.length > 0 && (
                    <>
                        <div className="dp-table-wrapper">
                            <table className="dp-table">
                                <thead>
                                    <tr>
                                        <th>Código</th>
                                        <th>Estudiante</th>
                                        <th>Documento</th>
                                        <th>Grado</th>
                                        <th>Sección</th>
                                        <th>Estado</th>
                                        <th>Acciones</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {items.map((item) => (
                                        <tr key={item.id}>
                                            <td className="dp-table-code">{item.codigo}</td>
                                            <td>
                                                <span className="est-cell-main">{[item.apellidos, item.nombres].filter(Boolean).join(", ")}</span>
                                                {item.codigo_siagie && <span className="est-cell-sub">SIAGIE {item.codigo_siagie}</span>}
                                            </td>
                                            <td>
                                                {tipoDocumentoLabel(item.tipo_documento)}
                                                {item.numero_documento ? ` ${enmascararDocumento(item.numero_documento)}` : ""}
                                            </td>
                                            <td>{item.grado || "—"}</td>
                                            <td>{item.seccion || "—"}</td>
                                            <td>
                                                <EstadoEstudianteBadge estado={item.estado} />
                                            </td>
                                            <td>
                                                <div className="est-row-actions">
                                                    <Link
                                                        to={`/estudiantes/${item.id}`}
                                                        className="dp-table-action"
                                                        aria-label={`Ver ${item.codigo}`}
                                                    >
                                                        <Icon name="eye" size={16} />
                                                        Ver ficha
                                                    </Link>
                                                    <Link
                                                        to={`/estudiantes/${item.id}?tab=historial`}
                                                        className="dp-table-action"
                                                        aria-label={`Historial de ${item.codigo}`}
                                                    >
                                                        Historial
                                                    </Link>
                                                    {canUpdate && (
                                                        <button
                                                            type="button"
                                                            className="dp-table-action est-link-button"
                                                            aria-label={`Editar ${item.codigo}`}
                                                            onClick={() => setEditando(item)}
                                                        >
                                                            Editar
                                                        </button>
                                                    )}
                                                    {canChangeEstado && (
                                                        <button
                                                            type="button"
                                                            className="dp-table-action est-link-button"
                                                            aria-label={`Cambiar estado ${item.codigo}`}
                                                            onClick={() => setCambiandoEstado(item)}
                                                        >
                                                            {item.estado === "ACTIVO" ? "Inactivar" : "Activar"}
                                                        </button>
                                                    )}
                                                </div>
                                            </td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        </div>

                        <div className="dp-pagination">
                            <span>
                                Página {page} de {totalPages} · {total} estudiante{total === 1 ? "" : "s"}
                            </span>
                            <div className="dp-pagination-buttons">
                                <button
                                    type="button"
                                    className="dp-btn-secondary"
                                    disabled={page <= 1}
                                    onClick={() => setPage((current) => current - 1)}
                                >
                                    Anterior
                                </button>
                                <button
                                    type="button"
                                    className="dp-btn-secondary"
                                    disabled={page >= totalPages}
                                    onClick={() => setPage((current) => current + 1)}
                                >
                                    Siguiente
                                </button>
                            </div>
                        </div>
                    </>
                )}
                </>
                )}
            </section>

            {showCreate && (
                <EstudianteFormModal
                    mode="create"
                    onClose={() => setShowCreate(false)}
                    onSaved={(estudiante) => {
                        setShowCreate(false);
                        navigate(`/estudiantes/${estudiante.id}`, { state: { mensaje: "Estudiante registrado correctamente." } });
                    }}
                />
            )}

            {editando && (
                <EstudianteFormModal
                    mode="edit"
                    estudiante={editando}
                    onClose={() => setEditando(null)}
                    onReload={recargar}
                    onSaved={(estudiante) => {
                        reemplazar(estudiante);
                        setEditando(null);
                        setMensaje("Cambios guardados correctamente.");
                    }}
                />
            )}

            {cambiandoEstado && (
                <CambiarEstadoModal
                    estudiante={cambiandoEstado}
                    onClose={() => setCambiandoEstado(null)}
                    onReload={recargar}
                    onChanged={(estudiante) => {
                        reemplazar(estudiante);
                        setCambiandoEstado(null);
                        setMensaje("Estado actualizado correctamente.");
                    }}
                />
            )}
        </RoleLayout>
    );
}

export default EstudiantesList;

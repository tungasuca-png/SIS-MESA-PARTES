import { useEffect, useState } from "react";
import EstudiantePersonalFormModal from "./EstudiantePersonalFormModal";
import { usePermissions } from "../../hooks/usePermissions";
import { getEstudiantePersonal } from "../../services/estudiantesService";
import { SEXOS } from "../../constants/estudiantes";
import { estudianteError } from "../../utils/estudiantesErrors";

const valor = (texto) => texto || "No registrado";
const sexoLabel = (value) => SEXOS.find((item) => item.value === value)?.label ?? value;

// Sección "Datos personales" del detalle (FASE E-08). Se monta solo con
// estudiantes.personal.view (ver EstudianteDetail) y pide los datos a su
// propio endpoint (GET /api/estudiantes/:id/personal, auditado en el
// backend): nunca vienen de GetEstudiante ni del listado.
//
// Versión: el registro tiene UNA sola versión que cambia con cualquier
// edición. Para expected_version se usa la mayor conocida entre la de esta
// lectura y la del detalle (las acciones locales del detalle no tocan los
// datos personales, así que siguen vigentes); si otra persona cambió algo
// después, el backend responde version_conflict.
function EstudiantePersonalSection({ estudianteId, versionDetalle, onSaved, onReload }) {
    const { can } = usePermissions();
    const canUpdate = can("estudiantes.personal.update");

    const [estado, setEstado] = useState({ loading: true, personal: null, error: null });
    const [editando, setEditando] = useState(false);

    useEffect(() => {
        let active = true;
        getEstudiantePersonal(estudianteId)
            .then((data) => {
                if (active) setEstado({ loading: false, personal: data.personal, error: null });
            })
            .catch((err) => {
                if (active) setEstado({ loading: false, personal: null, error: estudianteError(err) });
            });
        return () => {
            active = false;
        };
    }, [estudianteId]);

    const { loading, personal, error } = estado;
    const expectedVersion = Math.max(personal?.version ?? 0, versionDetalle ?? 0);

    let contenido;
    if (loading) {
        contenido = <p className="dp-table-empty">Cargando datos personales...</p>;
    } else if (error) {
        contenido = (
            <p className="dp-form-error">
                {error.kind === "forbidden" ? "No tienes permiso para ver los datos personales." : error.message}
            </p>
        );
    } else {
        contenido = (
            <dl className="dp-detail-list">
                <div>
                    <dt>Fecha de nacimiento</dt>
                    <dd>{valor(personal.fecha_nacimiento)}</dd>
                </div>
                <div>
                    <dt>Sexo</dt>
                    <dd>{valor(sexoLabel(personal.sexo))}</dd>
                </div>
                <div>
                    <dt>País de nacimiento</dt>
                    <dd>{valor(personal.pais_nacimiento)}</dd>
                </div>
                <div>
                    <dt>Ubigeo de nacimiento</dt>
                    <dd>{valor(personal.ubigeo_nacimiento)}</dd>
                </div>
                <div>
                    <dt>Lengua materna</dt>
                    <dd>{valor(personal.lengua_materna)}</dd>
                </div>
                <div>
                    <dt>Segunda lengua</dt>
                    <dd>{valor(personal.segunda_lengua)}</dd>
                </div>
                <div>
                    <dt>Edad</dt>
                    <dd>{personal.edad != null ? `${personal.edad} años` : "No disponible"}</dd>
                </div>
                <div>
                    <dt>Domicilio</dt>
                    <dd>{valor(personal.domicilio_direccion)}</dd>
                </div>
                <div>
                    <dt>Departamento / provincia / distrito</dt>
                    <dd>
                        {[personal.domicilio_departamento, personal.domicilio_provincia, personal.domicilio_distrito]
                            .filter(Boolean)
                            .join(" / ") || "No registrado"}
                        {personal.domicilio_ubigeo ? ` (ubigeo ${personal.domicilio_ubigeo})` : ""}
                    </dd>
                </div>
                <div>
                    <dt>Correo</dt>
                    <dd>{valor(personal.correo)}</dd>
                </div>
                <div>
                    <dt>Teléfono</dt>
                    <dd>{valor(personal.telefono)}</dd>
                </div>
            </dl>
        );
    }

    return (
        <section className="dp-panel">
            <div className="dp-detail-header">
                <h3 className="dp-panel-title">Datos personales</h3>
                {canUpdate && personal && (
                    <button type="button" className="dp-btn-secondary" onClick={() => setEditando(true)}>
                        Editar datos personales
                    </button>
                )}
            </div>
            {contenido}

            {editando && (
                <EstudiantePersonalFormModal
                    estudianteId={estudianteId}
                    personal={personal}
                    expectedVersion={expectedVersion}
                    onClose={() => setEditando(false)}
                    onReload={onReload}
                    onSaved={(basico, normalizados) => {
                        // La respuesta no trae datos personales (editar no da
                        // lectura): se muestran los valores enviados, ya
                        // normalizados igual que en el backend.
                        setEstado((current) => ({
                            ...current,
                            // La edad se recalcula en el backend: si cambió la fecha de
                            // nacimiento, la edad mostrada ya no es válida.
                            personal: {
                                ...current.personal,
                                ...normalizados,
                                version: basico.version,
                                edad:
                                    normalizados.fecha_nacimiento === current.personal.fecha_nacimiento
                                        ? current.personal.edad
                                        : null,
                            },
                        }));
                        setEditando(false);
                        onSaved(basico);
                    }}
                />
            )}
        </section>
    );
}

export default EstudiantePersonalSection;

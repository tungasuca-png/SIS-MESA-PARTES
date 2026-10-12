import { useEffect, useState } from "react";
import { getMatriculasEstudiante } from "../../services/estudiantesService";
import { estudianteError } from "../../utils/estudiantesErrors";

const ESTADOS = {
    MATRICULADO: "Matriculado",
    RETIRADO: "Retirado",
    TRASLADADO: "Trasladado",
    CONCLUIDO: "Concluido",
};

const MOVIMIENTOS = {
    CAMBIO_SECCION: "Cambio de sección",
    CAMBIO_TURNO: "Cambio de turno",
    RETIRO: "Retiro",
    REINGRESO: "Reingreso",
    TRASLADO_SALIDA: "Traslado (salida)",
};

// Historial académico (modelo maestro, etapa 3): una matrícula por periodo,
// más reciente primero, con los movimientos dentro del periodo. Solo
// lectura (las operaciones de matrícula son una fase posterior). Mismo
// permiso que el detalle (estudiantes.view).
function EstudianteMatriculasSection({ estudianteId }) {
    const [estado, setEstado] = useState({ loading: true, matriculas: [], error: null });

    useEffect(() => {
        let active = true;
        getMatriculasEstudiante(estudianteId)
            .then((data) => {
                if (active) setEstado({ loading: false, matriculas: data.matriculas || [], error: null });
            })
            .catch((err) => {
                if (active) setEstado({ loading: false, matriculas: [], error: estudianteError(err) });
            });
        return () => {
            active = false;
        };
    }, [estudianteId]);

    const { loading, matriculas, error } = estado;

    return (
        <section className="dp-panel">
            <h3 className="dp-panel-title">Historial de matrícula</h3>
            {loading && <p className="dp-table-empty">Cargando matrículas...</p>}
            {!loading && error && <p className="dp-form-error">{error.message}</p>}
            {!loading && !error && matriculas.length === 0 && (
                <p className="dp-table-empty">Sin matrículas registradas.</p>
            )}
            {!loading && !error && matriculas.length > 0 && (
                <div className="dp-table-wrapper">
                    <table className="dp-table">
                        <thead>
                            <tr>
                                <th>Periodo</th>
                                <th>Nivel / grado / sección</th>
                                <th>Turno / aula</th>
                                <th>Estado</th>
                                <th>Movimientos</th>
                            </tr>
                        </thead>
                        <tbody>
                            {matriculas.map((m) => (
                                <tr key={m.id}>
                                    <td>{m.periodo?.anio}</td>
                                    <td>
                                        {m.seccion
                                            ? `${m.seccion.nivel} · ${m.seccion.grado} · ${m.seccion.seccion}`
                                            : "Sin sección asignada"}
                                    </td>
                                    <td>{m.seccion ? [m.seccion.turno, m.seccion.aula].filter(Boolean).join(" · ") || "—" : "—"}</td>
                                    <td>{ESTADOS[m.estado_matricula] ?? m.estado_matricula}</td>
                                    <td>
                                        {(m.movimientos || []).length === 0
                                            ? "—"
                                            : m.movimientos.map((mv, i) => (
                                                  <div key={i}>
                                                      {MOVIMIENTOS[mv.tipo] ?? mv.tipo} ({mv.fecha})
                                                  </div>
                                              ))}
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
        </section>
    );
}

export default EstudianteMatriculasSection;

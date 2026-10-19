import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import EstadoEstudianteBadge from "./EstadoEstudianteBadge";
import { listPosiblesDuplicados } from "../../services/estudiantesService";
import { tipoDocumentoLabel } from "../../constants/estudiantes";
import { estudianteError } from "../../utils/estudiantesErrors";
import { enmascararDocumento } from "../../utils/privacidad";

const PAGE_SIZE = 10;

const CRITERIOS = {
    MISMO_NOMBRE_Y_FECHA: "Mismo nombre completo y misma fecha de nacimiento",
    MISMO_NOMBRE_SIN_DOCUMENTO: "Mismo nombre completo y al menos uno sin documento",
};

// Vista "Posibles duplicados" del directorio (GET /api/estudiantes/duplicados,
// estudiantes.update). Solo para revisión humana: el sistema no fusiona ni
// borra registros; quien revisa abre cada ficha y corrige lo que corresponda.
function PosiblesDuplicados() {
    const [page, setPage] = useState(1);
    const [estado, setEstado] = useState({ key: null, grupos: [], total: 0, error: null });

    useEffect(() => {
        let active = true;
        listPosiblesDuplicados({ page, pageSize: PAGE_SIZE })
            .then((d) => active && setEstado({ key: page, grupos: d.grupos || [], total: d.total || 0, error: null }))
            .catch((err) => active && setEstado({ key: page, grupos: [], total: 0, error: estudianteError(err) }));
        return () => {
            active = false;
        };
    }, [page]);

    const loading = estado.key !== page;
    const totalPages = Math.max(1, Math.ceil(estado.total / PAGE_SIZE));

    return (
        <div className="est-duplicados">
            <p className="est-form-note">
                Grupos de registros que podrían ser el mismo estudiante. El sistema no fusiona ni borra nada: revisa cada ficha y corrige los datos si
                corresponde.
            </p>
            {loading && <p className="dp-table-empty">Buscando posibles duplicados...</p>}
            {!loading && estado.error && (
                <p className="dp-form-error">
                    {estado.error.kind === "forbidden" ? "No tienes permiso para revisar duplicados." : estado.error.message}
                </p>
            )}
            {!loading && !estado.error && estado.grupos.length === 0 && <p className="dp-table-empty">No se encontraron posibles duplicados.</p>}
            {!loading && !estado.error && estado.grupos.length > 0 && (
                <>
                    <ul className="est-card-list">
                        {estado.grupos.map((g, i) => (
                            <li key={`${g.criterio}-${i}`} className="est-card">
                                <p className="est-card-sub">{CRITERIOS[g.criterio] ?? g.criterio}</p>
                                <div className="dp-table-wrapper">
                                    <table className="dp-table">
                                        <thead>
                                            <tr>
                                                <th>Código</th>
                                                <th>Estudiante</th>
                                                <th>Documento</th>
                                                <th>Estado</th>
                                                <th>Acciones</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {(g.estudiantes || []).map((e) => (
                                                <tr key={e.id}>
                                                    <td className="dp-table-code">{e.codigo}</td>
                                                    <td>{e.nombre_completo}</td>
                                                    <td>
                                                        {tipoDocumentoLabel(e.tipo_documento)} {enmascararDocumento(e.numero_documento)}
                                                    </td>
                                                    <td>
                                                        <EstadoEstudianteBadge estado={e.estado} />
                                                    </td>
                                                    <td>
                                                        <Link to={`/estudiantes/${e.id}`} className="dp-table-action">
                                                            Ver ficha
                                                        </Link>
                                                    </td>
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                </div>
                            </li>
                        ))}
                    </ul>
                    <div className="dp-pagination">
                        <span>
                            Página {page} de {totalPages} · {estado.total} grupo{estado.total === 1 ? "" : "s"}
                        </span>
                        <div className="dp-pagination-buttons">
                            <button type="button" className="dp-btn-secondary" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
                                Anterior
                            </button>
                            <button type="button" className="dp-btn-secondary" disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)}>
                                Siguiente
                            </button>
                        </div>
                    </div>
                </>
            )}
        </div>
    );
}

export default PosiblesDuplicados;

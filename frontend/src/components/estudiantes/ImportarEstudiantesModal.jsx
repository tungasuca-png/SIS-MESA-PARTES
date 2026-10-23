import { useState } from "react";
import Icon from "../dashboard/Icon";
import {
    XLSX_MIME,
    confirmarImportacionEstudiantes,
    descargarPlantillaEstudiantes,
    previsualizarImportacionEstudiantes,
} from "../../services/estudiantesService";
import { estudianteError } from "../../utils/estudiantesErrors";
import { descargarBase64, formatearTamano } from "../../utils/descargas";
import "../dashboard/forms.css";
import "./importarEstudiantes.css";

// Importación Excel de estudiantes en dos etapas: 1) validar y previsualizar
// (el backend no escribe nada), 2) confirmar. El backend vuelve a validar el
// MISMO archivo (huella) al confirmar; aquí nunca se da nada por guardado
// antes de su respuesta. No se guarda nada en el navegador.

const MAX_BYTES = 1024 * 1024;

const RESULTADOS = {
    VALIDA: { label: "Válida", clase: "ok" },
    ERROR: { label: "Con errores", clase: "error" },
    DUPLICADA: { label: "Duplicada en el archivo", clase: "error" },
    YA_EXISTE: { label: "Ya registrado", clase: "neutro" },
    INSERTADA: { label: "Registrado", clase: "ok" },
    OMITIDA: { label: "Omitido", clase: "neutro" },
    RECHAZADA: { label: "Rechazada", clase: "error" },
};

const FILTROS_PREVIA = [
    { key: "todas", label: "Todas", incluye: () => true },
    { key: "validas", label: "Válidas", incluye: (f) => f.resultado === "VALIDA" },
    { key: "errores", label: "Con errores", incluye: (f) => f.resultado === "ERROR" || f.resultado === "DUPLICADA" },
    { key: "existentes", label: "Ya registrados", incluye: (f) => f.resultado === "YA_EXISTE" },
    { key: "avisos", label: "Con avisos", incluye: (f) => f.avisos?.length > 0 },
];

function validarArchivoLocal(archivo) {
    if (!archivo) return "Selecciona un archivo.";
    if (!archivo.name.toLowerCase().endsWith(".xlsx")) return "Solo se aceptan archivos Excel .xlsx.";
    if (archivo.size === 0) return "El archivo está vacío.";
    if (archivo.size > MAX_BYTES) return "El archivo supera el máximo de 1 MB.";
    return null;
}

function ResultadoChip({ resultado }) {
    const r = RESULTADOS[resultado] ?? { label: resultado, clase: "neutro" };
    return <span className={`imp-chip imp-chip--${r.clase}`}>{r.label}</span>;
}

function TablaFilas({ filas }) {
    if (filas.length === 0) return <p className="dp-table-empty">No hay filas en esta categoría.</p>;
    return (
        <div className="dp-table-wrapper imp-tabla">
            <table className="dp-table">
                <thead>
                    <tr>
                        <th>Fila</th>
                        <th>Documento</th>
                        <th>Apellidos y nombres</th>
                        <th>Estado / Nivel</th>
                        <th>Resultado</th>
                        <th>Detalle</th>
                    </tr>
                </thead>
                <tbody>
                    {filas.map((f) => (
                        <tr key={f.fila}>
                            <td>{f.fila}</td>
                            <td className="imp-nowrap">
                                {f.tipo_documento || "—"} {f.numero_documento}
                            </td>
                            <td>
                                {[f.apellido_paterno, f.apellido_materno].filter(Boolean).join(" ")}
                                {f.nombres ? `, ${f.nombres}` : ""}
                            </td>
                            <td className="imp-nowrap">
                                {f.estado || "—"} / {f.nivel || "—"}
                            </td>
                            <td>
                                <ResultadoChip resultado={f.resultado} />
                            </td>
                            <td className="imp-detalle">
                                {f.errores?.map((e) => (
                                    <p key={e} className="imp-error">{e}</p>
                                ))}
                                {f.avisos?.map((a) => (
                                    <p key={a} className="imp-aviso">{a}</p>
                                ))}
                            </td>
                        </tr>
                    ))}
                </tbody>
            </table>
        </div>
    );
}

function Resumen({ resumen, confirmada }) {
    const items = confirmada
        ? [
              { label: "Registrados", valor: resumen.insertadas, clase: "ok" },
              { label: "Omitidos (ya existían)", valor: resumen.omitidas, clase: "neutro" },
              { label: "Rechazados", valor: resumen.rechazadas, clase: "error" },
          ]
        : [
              { label: "Filas", valor: resumen.total, clase: "neutro" },
              { label: "Válidas", valor: resumen.validas, clase: "ok" },
              { label: "Con errores", valor: resumen.con_errores, clase: "error" },
              { label: "Duplicadas", valor: resumen.duplicadas, clase: "error" },
              { label: "Ya registrados", valor: resumen.ya_existen, clase: "neutro" },
          ];
    return (
        <div className="imp-resumen" role="status">
            {items.map((i) => (
                <div key={i.label} className={`imp-resumen-item imp-resumen-item--${i.clase}`}>
                    <strong>{i.valor ?? 0}</strong>
                    <span>{i.label}</span>
                </div>
            ))}
        </div>
    );
}

function ImportarEstudiantesModal({ onClose, onImported }) {
    const [archivo, setArchivo] = useState(null);
    const [previa, setPrevia] = useState(null);
    const [final, setFinal] = useState(null);
    const [filtro, setFiltro] = useState("todas");
    const [aceptado, setAceptado] = useState(false);
    const [ocupado, setOcupado] = useState(""); // "plantilla" | "validando" | "importando"
    const [error, setError] = useState("");
    const [arrastrando, setArrastrando] = useState(false);

    const elegir = (nuevo) => {
        // Un archivo nuevo reinicia todo el flujo.
        setPrevia(null);
        setFinal(null);
        setAceptado(false);
        setFiltro("todas");
        setError(validarArchivoLocal(nuevo) ?? "");
        setArchivo(nuevo ?? null);
    };

    const cerrar = () => {
        if (ocupado) return;
        if (final?.resumen?.insertadas > 0) onImported?.(final.resumen);
        onClose();
    };

    const plantilla = async () => {
        setOcupado("plantilla");
        setError("");
        try {
            const data = await descargarPlantillaEstudiantes();
            descargarBase64(data.contenido_base64, data.nombre, XLSX_MIME);
        } catch (err) {
            setError(estudianteError(err).message);
        } finally {
            setOcupado("");
        }
    };

    const validar = async () => {
        const local = validarArchivoLocal(archivo);
        if (local) {
            setError(local);
            return;
        }
        setOcupado("validando");
        setError("");
        try {
            setPrevia(await previsualizarImportacionEstudiantes(archivo));
        } catch (err) {
            setError(estudianteError(err).message);
        } finally {
            setOcupado("");
        }
    };

    const confirmar = async () => {
        if (!previa || !aceptado || ocupado) return;
        setOcupado("importando");
        setError("");
        try {
            const resultado = await confirmarImportacionEstudiantes(archivo, previa.huella);
            setFinal(resultado);
            setFiltro("todas");
        } catch (err) {
            const e = estudianteError(err);
            setError(e.message);
            // Si el archivo cambió, hay que volver a validarlo.
            if (e.code === "importacion_cambiada") setPrevia(null);
        } finally {
            setOcupado("");
        }
    };

    const datos = final ?? previa;
    const filtroActual = FILTROS_PREVIA.find((f) => f.key === filtro) ?? FILTROS_PREVIA[0];
    const filas = datos ? datos.filas.filter(final ? () => true : filtroActual.incluye) : [];
    const validas = previa?.resumen?.validas ?? 0;
    const noImportadas = previa ? previa.resumen.total - validas : 0;

    return (
        <div className="dp-modal-overlay" onClick={cerrar}>
            <div
                className="dp-modal est-modal imp-modal"
                role="dialog"
                aria-modal="true"
                aria-label="Importar estudiantes desde Excel"
                onClick={(event) => event.stopPropagation()}
            >
                <div className="dp-modal-header">
                    <h2>Importar estudiantes desde Excel</h2>
                    <button type="button" className="dp-modal-close" onClick={cerrar} aria-label="Cerrar" disabled={Boolean(ocupado)}>
                        <Icon name="close" size={18} />
                    </button>
                </div>

                <div className="dp-form imp-cuerpo">
                    {!final && (
                        <>
                            <ol className="imp-pasos">
                                <li>
                                    Descarga la plantilla oficial y completa una fila por estudiante.{" "}
                                    <button type="button" className="imp-link" onClick={plantilla} disabled={Boolean(ocupado)}>
                                        {ocupado === "plantilla" ? "Descargando…" : "Descargar plantilla"}
                                    </button>
                                </li>
                                <li>Selecciona el archivo y valídalo: nada se guarda hasta que confirmes.</li>
                            </ol>

                            <div
                                className={`imp-zona${arrastrando ? " imp-zona--activa" : ""}`}
                                onDragOver={(e) => {
                                    e.preventDefault();
                                    if (!ocupado) setArrastrando(true);
                                }}
                                onDragLeave={() => setArrastrando(false)}
                                onDrop={(e) => {
                                    e.preventDefault();
                                    setArrastrando(false);
                                    if (!ocupado) elegir(e.dataTransfer.files?.[0]);
                                }}
                            >
                                <Icon name="file" size={22} />
                                {archivo ? (
                                    <p className="imp-archivo">
                                        <strong>{archivo.name}</strong> <span>({formatearTamano(archivo.size)})</span>
                                    </p>
                                ) : (
                                    <p>Arrastra aquí el archivo .xlsx o</p>
                                )}
                                <label className="dp-btn-secondary imp-elegir">
                                    {archivo ? "Cambiar archivo" : "Seleccionar archivo"}
                                    <input
                                        type="file"
                                        accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                                        aria-label="Archivo Excel de estudiantes"
                                        disabled={Boolean(ocupado)}
                                        onChange={(e) => {
                                            elegir(e.target.files?.[0]);
                                            e.target.value = "";
                                        }}
                                    />
                                </label>
                                <p className="imp-nota">Máximo 1 MB. Formato: Nro, TIPO.DOC, DNI, APELLIDO PATERNO, APELLIDO MATERNO, NOMBRES, CONDICION, ESTADO, NIVEL.</p>
                            </div>
                        </>
                    )}

                    {error && (
                        <p className="dp-form-error" role="alert">
                            {error}
                        </p>
                    )}

                    {ocupado === "validando" && <p className="imp-progreso" aria-live="polite">Validando el archivo…</p>}
                    {ocupado === "importando" && <p className="imp-progreso" aria-live="polite">Importando estudiantes… no cierres esta ventana.</p>}

                    {datos && (
                        <>
                            {final && (
                                <p className="imp-final">
                                    Importación terminada: {final.resumen.insertadas} estudiante(s) registrado(s).
                                </p>
                            )}
                            <Resumen resumen={datos.resumen} confirmada={Boolean(final)} />
                            {!final && (
                                <div className="est-segmented imp-filtros" role="tablist" aria-label="Filtrar filas">
                                    {FILTROS_PREVIA.map((f) => (
                                        <button
                                            key={f.key}
                                            type="button"
                                            role="tab"
                                            aria-selected={filtro === f.key}
                                            className={filtro === f.key ? "is-active" : ""}
                                            onClick={() => setFiltro(f.key)}
                                        >
                                            {f.label}
                                        </button>
                                    ))}
                                </div>
                            )}
                            <TablaFilas filas={filas} />
                        </>
                    )}

                    {previa && !final && (
                        <label className="imp-confirmacion">
                            <input type="checkbox" checked={aceptado} disabled={validas === 0 || Boolean(ocupado)} onChange={(e) => setAceptado(e.target.checked)} />
                            <span>
                                {validas === 0
                                    ? "No hay filas válidas para importar. Corrige el archivo y vuelve a validarlo."
                                    : `Registrar ${validas} estudiante(s) nuevo(s).${
                                          noImportadas > 0 ? ` ${noImportadas} fila(s) no se importarán (errores, duplicadas o ya registradas).` : ""
                                      } Los estudiantes ya registrados no se modifican.`}
                            </span>
                        </label>
                    )}

                    <div className="dp-form-actions">
                        {final ? (
                            <button type="button" className="dp-btn-primary" onClick={cerrar}>
                                Cerrar
                            </button>
                        ) : (
                            <>
                                <button type="button" className="dp-btn-secondary" onClick={cerrar} disabled={Boolean(ocupado)}>
                                    Cancelar
                                </button>
                                {previa ? (
                                    <button type="button" className="dp-btn-primary" onClick={confirmar} disabled={!aceptado || validas === 0 || Boolean(ocupado)}>
                                        {ocupado === "importando" ? "Importando…" : `Importar ${validas} estudiante(s)`}
                                    </button>
                                ) : (
                                    <button type="button" className="dp-btn-primary" onClick={validar} disabled={Boolean(validarArchivoLocal(archivo)) || Boolean(ocupado)}>
                                        {ocupado === "validando" ? "Validando…" : "Validar archivo"}
                                    </button>
                                )}
                            </>
                        )}
                    </div>
                </div>
            </div>
        </div>
    );
}

export default ImportarEstudiantesModal;

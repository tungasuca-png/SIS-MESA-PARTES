import { useEffect, useMemo, useState } from "react";
import Icon from "../dashboard/Icon";
import {
    FORMATOS_IMPORTACION,
    MAX_BYTES_IMPORTACION,
    analizarDatos,
    confirmarDatos,
    getCamposDatos,
    guardarPlantillaDatos,
    listPlantillasDatos,
} from "../../services/datosEstudiantesService";
import { XLSX_MIME, descargarPlantillaEstudiantes } from "../../services/estudiantesService";
import { estudianteError } from "../../utils/estudiantesErrors";
import { descargarBase64, formatearTamano } from "../../utils/descargas";
import { descargarInforme, etiquetaCategoria } from "../../utils/informeDatos";
import "../dashboard/forms.css";
import "./importarEstudiantes.css";

// Importación universal de estudiantes (xlsx, ods, xls web, csv, json, xml…).
// Pasos: 1) archivo, 2) estructura y mapeo de columnas, 3) revisión de
// registros, 4) resultado. El backend detecta el formato, valida y es el único
// que escribe: la revisión NO guarda nada y la confirmación exige la huella de
// la revisión (mismo archivo, opciones y mapeo). Nada se guarda en el navegador.

const CLASE_CATEGORIA = {
    NUEVO: "ok",
    EXISTENTE: "neutro",
    DUPLICADO: "neutro",
    POR_CONFIRMAR: "aviso",
    CONFLICTO: "error",
    INVALIDO: "error",
};

const FILTROS = [
    { key: "todas", label: "Todas", incluye: () => true },
    { key: "nuevos", label: "Nuevos", incluye: (r) => r.categoria === "NUEVO" },
    { key: "confirmar", label: "Por confirmar", incluye: (r) => r.categoria === "POR_CONFIRMAR" },
    { key: "problemas", label: "Con errores", incluye: (r) => r.categoria === "INVALIDO" || r.categoria === "CONFLICTO" },
    { key: "existentes", label: "Ya registrados / duplicados", incluye: (r) => r.categoria === "EXISTENTE" || r.categoria === "DUPLICADO" },
    { key: "avisos", label: "Con avisos o correcciones", incluye: (r) => r.avisos?.length > 0 || r.correcciones?.length > 0 },
];

const CONFIANZA = { ALTA: "Reconocida", MEDIA: "Por confirmar", BAJA: "Sugerencia" };

function validarArchivo(archivo) {
    if (!archivo) return "Selecciona un archivo.";
    if (archivo.size === 0) return "El archivo está vacío.";
    if (archivo.size > MAX_BYTES_IMPORTACION) return "El archivo supera el máximo de 3 MB.";
    return null;
}

function Chip({ categoria }) {
    return <span className={`imp-chip imp-chip--${CLASE_CATEGORIA[categoria] ?? "neutro"}`}>{etiquetaCategoria(categoria)}</span>;
}

function opcionesDe(analisis) {
    return {
        hoja: analisis.hoja || "",
        ruta: analisis.ruta || "",
        separador: analisis.separador || "",
        codificacion: analisis.codificacion || "",
        fila_encabezado: analisis.fila_encabezado || 0,
    };
}

function mapeoInicial(analisis) {
    const out = {};
    for (const c of analisis.columnas) {
        if (c.campo && c.importable) out[c.columna] = c.campo;
    }
    return out;
}

function ImportarDatosModal({ onClose, onImported }) {
    const [paso, setPaso] = useState("archivo");
    const [archivo, setArchivo] = useState(null);
    const [campos, setCampos] = useState([]);
    const [plantillas, setPlantillas] = useState([]);
    const [plantillaId, setPlantillaId] = useState("");
    const [analisis, setAnalisis] = useState(null);
    const [opciones, setOpciones] = useState(null);
    const [mapeo, setMapeo] = useState({});
    const [previa, setPrevia] = useState(null);
    const [decisiones, setDecisiones] = useState({});
    const [excluir, setExcluir] = useState(false);
    const [final, setFinal] = useState(null);
    const [filtro, setFiltro] = useState("todas");
    const [ocupado, setOcupado] = useState("");
    const [error, setError] = useState("");
    const [arrastrando, setArrastrando] = useState(false);
    const [nombrePlantilla, setNombrePlantilla] = useState("");
    const [mensajePlantilla, setMensajePlantilla] = useState("");

    useEffect(() => {
        let activo = true;
        getCamposDatos()
            .then((data) => activo && setCampos(data.campos.filter((c) => c.importable)))
            .catch((err) => activo && setError(estudianteError(err).message));
        // Las plantillas son opcionales: si el servidor aún no las tiene, se sigue sin ellas.
        listPlantillasDatos("IMPORTACION")
            .then((data) => activo && setPlantillas(data.plantillas))
            .catch(() => {});
        return () => {
            activo = false;
        };
    }, []);

    const grupos = useMemo(() => {
        const out = {};
        for (const c of campos) (out[c.grupo] ||= []).push(c);
        return out;
    }, [campos]);
    const etiqueta = (clave) => campos.find((c) => c.clave === clave)?.etiqueta ?? clave;

    const reiniciar = (nuevo) => {
        setArchivo(nuevo ?? null);
        setAnalisis(null);
        setOpciones(null);
        setMapeo({});
        setPrevia(null);
        setFinal(null);
        setDecisiones({});
        setExcluir(false);
        setFiltro("todas");
        setMensajePlantilla("");
        setPaso("archivo");
        setError(nuevo ? validarArchivo(nuevo) ?? "" : "");
    };

    const cerrar = () => {
        if (ocupado) return;
        if (final?.resumen?.insertados > 0) onImported?.(final.resumen);
        onClose();
    };

    const ejecutar = async (clave, fn) => {
        setOcupado(clave);
        setError("");
        try {
            await fn();
        } catch (err) {
            const e = estudianteError(err);
            setError(e.message);
            if (e.code === "importacion_cambiada" || e.code === "importacion_conflicto") {
                setPrevia(null);
                setPaso("estructura");
            }
        } finally {
            setOcupado("");
        }
    };

    // Paso 1 -> 2: el servicio detecta formato, hoja, encabezados y sugiere el mapeo.
    const analizar = (nuevasOpciones) =>
        ejecutar("analizando", async () => {
            const solicitud = { plantilla_id: plantillaId || undefined };
            if (nuevasOpciones) solicitud.opciones = nuevasOpciones;
            const data = await analizarDatos(archivo, solicitud);
            setAnalisis(data);
            setOpciones(opcionesDe(data));
            setMapeo(mapeoInicial(data));
            setPrevia(null);
            setPaso("estructura");
        });

    const solicitudConfirmada = () => ({
        opciones,
        mapeo: Object.entries(mapeo)
            .filter(([, campo]) => campo)
            .map(([columna, campo]) => ({ columna: Number(columna), campo })),
        mapeo_confirmado: true,
    });

    // Paso 2 -> 3: revisión de registros con el mapeo confirmado (no escribe).
    const revisar = () =>
        ejecutar("revisando", async () => {
            const data = await analizarDatos(archivo, solicitudConfirmada());
            setPrevia(data);
            setDecisiones({});
            setExcluir(false);
            setFiltro("todas");
            setPaso("registros");
        });

    const porConfirmar = previa ? previa.registros.filter((r) => r.categoria === "POR_CONFIRMAR") : [];
    const criticas = previa ? previa.resumen.invalidos + previa.resumen.conflictos : 0;
    const sinDecidir = porConfirmar.filter((r) => !decisiones[r.fila]).length;
    const aceptadas = porConfirmar.filter((r) => decisiones[r.fila] === "importar").map((r) => r.fila);
    const aRegistrar = previa ? previa.resumen.nuevos + aceptadas.length : 0;
    const puedeConfirmar =
        previa && previa.bloqueantes.length === 0 && (criticas === 0 || excluir) && sinDecidir === 0 && aRegistrar > 0 && !ocupado;

    const confirmar = () =>
        ejecutar("importando", async () => {
            const data = await confirmarDatos(archivo, {
                ...solicitudConfirmada(),
                huella: previa.huella,
                excluir_invalidos: excluir,
                aceptadas,
                omitidas: porConfirmar.filter((r) => decisiones[r.fila] === "omitir").map((r) => r.fila),
            });
            setFinal(data);
            setFiltro("todas");
            setPaso("resultado");
        });

    const guardarPlantilla = () =>
        ejecutar("plantilla", async () => {
            const asociaciones = {};
            for (const c of analisis.columnas) {
                if (mapeo[c.columna]) asociaciones[c.encabezado] = mapeo[c.columna];
            }
            const { plantilla } = await guardarPlantillaDatos({
                tipo: "IMPORTACION",
                nombre: nombrePlantilla,
                configuracion: JSON.stringify({ opciones, asociaciones }),
            });
            setPlantillas((actual) => [...actual.filter((p) => p.id !== plantilla.id), plantilla]);
            setMensajePlantilla(`Plantilla «${plantilla.nombre}» guardada.`);
            setNombrePlantilla("");
        });

    const datos = final ?? previa;
    const filtroActual = FILTROS.find((f) => f.key === filtro) ?? FILTROS[0];
    const registros = datos ? datos.registros.filter(filtroActual.incluye) : [];
    const columnasUsadas = Object.values(mapeo).filter(Boolean);

    return (
        <div className="dp-modal-overlay" onClick={cerrar}>
            <div className="dp-modal est-modal imp-modal" role="dialog" aria-modal="true" aria-label="Importar estudiantes" onClick={(e) => e.stopPropagation()}>
                <div className="dp-modal-header">
                    <h2>Importar estudiantes</h2>
                    <button type="button" className="dp-modal-close" onClick={cerrar} aria-label="Cerrar" disabled={Boolean(ocupado)}>
                        <Icon name="close" size={18} />
                    </button>
                </div>

                <ol className="dat-pasos" aria-label="Pasos">
                    {[
                        ["archivo", "Archivo"],
                        ["estructura", "Columnas"],
                        ["registros", "Revisión"],
                        ["resultado", "Resultado"],
                    ].map(([clave, texto], i) => (
                        <li key={clave} className={paso === clave ? "is-actual" : ""} aria-current={paso === clave ? "step" : undefined}>
                            <span>{i + 1}</span> {texto}
                        </li>
                    ))}
                </ol>

                <div className="dp-form imp-cuerpo">
                    {error && (
                        <p className="dp-form-error" role="alert">
                            {error}
                        </p>
                    )}

                    {paso === "archivo" && (
                        <>
                            <p className="imp-nota-texto">
                                Formatos admitidos: Excel (.xlsx, .xlsm), OpenDocument (.ods), «.xls» exportados por sistemas web (HTML), CSV/TSV/TXT,
                                JSON, JSON Lines y XML. El sistema reconoce las columnas por su nombre y su contenido; podrás revisar todo antes de guardar.{" "}
                                <button
                                    type="button"
                                    className="imp-link"
                                    disabled={Boolean(ocupado)}
                                    onClick={() =>
                                        ejecutar("plantilla-oficial", async () => {
                                            const data = await descargarPlantillaEstudiantes();
                                            descargarBase64(data.contenido_base64, data.nombre, XLSX_MIME);
                                        })
                                    }
                                >
                                    Descargar plantilla oficial
                                </button>
                            </p>
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
                                    if (!ocupado) reiniciar(e.dataTransfer.files?.[0]);
                                }}
                            >
                                <Icon name="file" size={22} />
                                {archivo ? (
                                    <p className="imp-archivo">
                                        <strong>{archivo.name}</strong> <span>({formatearTamano(archivo.size)})</span>
                                    </p>
                                ) : (
                                    <p>Arrastra aquí el archivo o</p>
                                )}
                                <label className="dp-btn-secondary imp-elegir">
                                    {archivo ? "Cambiar archivo" : "Seleccionar archivo"}
                                    <input
                                        type="file"
                                        accept={FORMATOS_IMPORTACION}
                                        aria-label="Archivo de estudiantes"
                                        disabled={Boolean(ocupado)}
                                        onChange={(e) => {
                                            reiniciar(e.target.files?.[0]);
                                            e.target.value = "";
                                        }}
                                    />
                                </label>
                                <p className="imp-nota">Máximo 3 MB.</p>
                            </div>
                            {plantillas.length > 0 && (
                                <label className="dat-campo">
                                    Plantilla de columnas (opcional)
                                    <select value={plantillaId} onChange={(e) => setPlantillaId(e.target.value)} disabled={Boolean(ocupado)}>
                                        <option value="">Reconocer automáticamente</option>
                                        {plantillas.map((p) => (
                                            <option key={p.id} value={p.id}>
                                                {p.nombre}
                                            </option>
                                        ))}
                                    </select>
                                </label>
                            )}
                        </>
                    )}

                    {paso === "estructura" && analisis && (
                        <>
                            <div className="dat-origen">
                                <p>
                                    <strong>{archivo.name}</strong> — formato detectado: <strong>{analisis.formato.toUpperCase()}</strong>
                                    {analisis.fila_encabezado > 0 && <> · encabezados en la fila {analisis.fila_encabezado}</>}
                                    {analisis.plantilla_aplicada && <> · plantilla «{analisis.plantilla_aplicada}»</>}
                                </p>
                                {[...analisis.avisos_archivo, ...analisis.plantilla_avisos].map((a) => (
                                    <p key={a} className="imp-aviso">
                                        {a}
                                    </p>
                                ))}
                            </div>

                            <fieldset className="dat-opciones">
                                <legend>Lectura del archivo</legend>
                                {analisis.hojas.length > 1 && (
                                    <label className="dat-campo">
                                        Hoja / tabla
                                        <select value={opciones.hoja} onChange={(e) => setOpciones({ ...opciones, hoja: e.target.value, fila_encabezado: 0 })}>
                                            {analisis.hojas.map((h) => (
                                                <option key={h}>{h}</option>
                                            ))}
                                        </select>
                                    </label>
                                )}
                                {analisis.rutas.length > 1 && (
                                    <label className="dat-campo">
                                        Registros en
                                        <select value={opciones.ruta} onChange={(e) => setOpciones({ ...opciones, ruta: e.target.value })}>
                                            {analisis.rutas.map((r) => (
                                                <option key={r}>{r}</option>
                                            ))}
                                        </select>
                                    </label>
                                )}
                                {analisis.formato === "csv" && (
                                    <label className="dat-campo">
                                        Separador
                                        <select value={opciones.separador} onChange={(e) => setOpciones({ ...opciones, separador: e.target.value })}>
                                            <option value=",">Coma (,)</option>
                                            <option value=";">Punto y coma (;)</option>
                                            <option value="tab">Tabulador</option>
                                            <option value="|">Barra (|)</option>
                                        </select>
                                    </label>
                                )}
                                {(analisis.formato === "csv" || analisis.formato === "html") && (
                                    <label className="dat-campo">
                                        Codificación
                                        <select value={opciones.codificacion} onChange={(e) => setOpciones({ ...opciones, codificacion: e.target.value })}>
                                            <option value="utf-8">UTF-8</option>
                                            <option value="windows-1252">Windows-1252 (Excel antiguo)</option>
                                            <option value="iso-8859-1">ISO-8859-1</option>
                                            <option value="utf-16">UTF-16</option>
                                        </select>
                                    </label>
                                )}
                                {analisis.fila_encabezado > 0 && (
                                    <label className="dat-campo dat-campo--corto">
                                        Fila de encabezados
                                        <input
                                            type="number"
                                            min="1"
                                            value={opciones.fila_encabezado}
                                            onChange={(e) => setOpciones({ ...opciones, fila_encabezado: Number(e.target.value) || 0 })}
                                        />
                                    </label>
                                )}
                                <button type="button" className="dp-btn-secondary" disabled={Boolean(ocupado)} onClick={() => analizar(opciones)}>
                                    {ocupado === "analizando" ? "Leyendo…" : "Volver a leer"}
                                </button>
                            </fieldset>

                            {analisis.problemas_mapeo.map((p) => (
                                <p key={p} className="imp-aviso">
                                    {p}
                                </p>
                            ))}
                            <div className="dp-table-wrapper imp-tabla">
                                <table className="dp-table">
                                    <thead>
                                        <tr>
                                            <th>Columna del archivo</th>
                                            <th>Ejemplos</th>
                                            <th>Campo del sistema</th>
                                            <th>Reconocimiento</th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        {analisis.columnas.map((c) => (
                                            <tr key={c.columna}>
                                                <td>
                                                    <strong>{c.encabezado}</strong>
                                                </td>
                                                <td className="dat-muestras">{c.muestras.join(" · ") || "—"}</td>
                                                <td>
                                                    {c.importable ? (
                                                        <select
                                                            aria-label={`Campo para «${c.encabezado}»`}
                                                            value={mapeo[c.columna] || ""}
                                                            onChange={(e) => setMapeo({ ...mapeo, [c.columna]: e.target.value })}
                                                        >
                                                            <option value="">— No importar —</option>
                                                            {Object.entries(grupos).map(([grupo, lista]) => (
                                                                <optgroup key={grupo} label={grupo}>
                                                                    {lista.map((campo) => (
                                                                        <option
                                                                            key={campo.clave}
                                                                            value={campo.clave}
                                                                            disabled={mapeo[c.columna] !== campo.clave && columnasUsadas.includes(campo.clave)}
                                                                        >
                                                                            {campo.etiqueta}
                                                                        </option>
                                                                    ))}
                                                                </optgroup>
                                                            ))}
                                                        </select>
                                                    ) : (
                                                        <span className="dat-no-importa">{etiqueta(c.campo)} — no se importa (dato de matrícula)</span>
                                                    )}
                                                </td>
                                                <td className="imp-detalle">
                                                    {c.campo && c.importable && mapeo[c.columna] === c.campo && (
                                                        <span className={`imp-chip imp-chip--${c.confianza === "ALTA" ? "ok" : "aviso"}`}>
                                                            {c.confirmada ? "Plantilla" : CONFIANZA[c.confianza] ?? c.confianza}
                                                        </span>
                                                    )}
                                                    {c.motivo && <p>{c.motivo}</p>}
                                                </td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                            </div>
                            <p className="imp-nota-texto">
                                Revisa en especial las columnas «Por confirmar» o «Sugerencia». Las columnas sin campo no se importan.
                            </p>
                        </>
                    )}

                    {(paso === "registros" || paso === "resultado") && datos && (
                        <>
                            {final && (
                                <p className="imp-final">
                                    Importación terminada: {final.resumen.insertados} estudiante(s) registrado(s), {final.resumen.omitidos} omitido(s).
                                </p>
                            )}
                            {datos.bloqueantes.map((b) => (
                                <p key={b} className="dp-form-error">
                                    {b}
                                </p>
                            ))}
                            {datos.avisos.map((a) => (
                                <p key={a} className="imp-aviso">
                                    {a}
                                </p>
                            ))}
                            <div className="imp-resumen" role="status">
                                {(final
                                    ? [
                                          ["Registrados", datos.resumen.insertados, "ok"],
                                          ["Omitidos", datos.resumen.omitidos, "neutro"],
                                      ]
                                    : [
                                          ["Registros", datos.resumen.total, "neutro"],
                                          ["Nuevos", datos.resumen.nuevos, "ok"],
                                          ["Ya registrados", datos.resumen.existentes, "neutro"],
                                          ["Duplicados", datos.resumen.duplicados, "neutro"],
                                          ["Por confirmar", datos.resumen.por_confirmar, "aviso"],
                                          ["Con errores", datos.resumen.invalidos + datos.resumen.conflictos, "error"],
                                      ]
                                ).map(([label, valor, clase]) => (
                                    <div key={label} className={`imp-resumen-item imp-resumen-item--${clase}`}>
                                        <strong>{valor}</strong>
                                        <span>{label}</span>
                                    </div>
                                ))}
                            </div>
                            <div className="est-segmented imp-filtros" role="tablist" aria-label="Filtrar registros">
                                {FILTROS.map((f) => (
                                    <button key={f.key} type="button" role="tab" aria-selected={filtro === f.key} className={filtro === f.key ? "is-active" : ""} onClick={() => setFiltro(f.key)}>
                                        {f.label}
                                    </button>
                                ))}
                            </div>
                            {registros.length === 0 ? (
                                <p className="dp-table-empty">No hay registros en esta categoría.</p>
                            ) : (
                                <div className="dp-table-wrapper imp-tabla">
                                    <table className="dp-table">
                                        <thead>
                                            <tr>
                                                <th>Fila</th>
                                                <th>Documento</th>
                                                <th>Apellidos y nombres</th>
                                                <th>Datos</th>
                                                <th>Resultado</th>
                                                <th>Detalle</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {registros.map((r) => (
                                                <tr key={r.fila}>
                                                    <td>{r.fila}</td>
                                                    <td className="imp-nowrap">{r.documento || "—"}</td>
                                                    <td>
                                                        {r.apellidos}
                                                        {r.nombres ? `, ${r.nombres}` : ""}
                                                    </td>
                                                    <td className="imp-nowrap">{[r.estado, r.nivel, r.grado, r.seccion].filter(Boolean).join(" · ") || "—"}</td>
                                                    <td>
                                                        {final ? (
                                                            <span className={`imp-chip imp-chip--${r.resultado === "INSERTADO" ? "ok" : "neutro"}`}>
                                                                {r.resultado === "INSERTADO" ? "Registrado" : "Omitido"}
                                                            </span>
                                                        ) : (
                                                            <Chip categoria={r.categoria} />
                                                        )}
                                                        {!final && r.categoria === "POR_CONFIRMAR" && (
                                                            <div className="dat-decision" role="radiogroup" aria-label={`Decisión para la fila ${r.fila}`}>
                                                                {[
                                                                    ["importar", "Es otra persona: importar"],
                                                                    ["omitir", "Omitir"],
                                                                ].map(([valor, texto]) => (
                                                                    <label key={valor}>
                                                                        <input
                                                                            type="radio"
                                                                            name={`decision-${r.fila}`}
                                                                            checked={decisiones[r.fila] === valor}
                                                                            onChange={() => setDecisiones({ ...decisiones, [r.fila]: valor })}
                                                                        />
                                                                        {texto}
                                                                    </label>
                                                                ))}
                                                            </div>
                                                        )}
                                                    </td>
                                                    <td className="imp-detalle">
                                                        {r.errores.map((e) => (
                                                            <p key={e} className="imp-error">
                                                                {e}
                                                            </p>
                                                        ))}
                                                        {r.avisos.map((a) => (
                                                            <p key={a} className="imp-aviso">
                                                                {a}
                                                            </p>
                                                        ))}
                                                        {r.correcciones.map((c) => (
                                                            <p key={c} className="dat-correccion">
                                                                Corrección: {c}
                                                            </p>
                                                        ))}
                                                    </td>
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                </div>
                            )}
                            <div className="dat-acciones-sec">
                                <button
                                    type="button"
                                    className="imp-link"
                                    onClick={() => descargarInforme(datos.registros, final ? "informe_importacion.csv" : "informe_revision.csv")}
                                >
                                    Descargar informe de errores y avisos (CSV)
                                </button>
                            </div>
                        </>
                    )}

                    {paso === "registros" && previa && (
                        <>
                            {criticas > 0 && (
                                <label className="imp-confirmacion">
                                    <input type="checkbox" checked={excluir} disabled={Boolean(ocupado)} onChange={(e) => setExcluir(e.target.checked)} />
                                    <span>Excluir {criticas} fila(s) con errores o conflictos (no se importarán; corrígelas y vuelve a importarlas después).</span>
                                </label>
                            )}
                            {sinDecidir > 0 && <p className="imp-aviso">Falta decidir {sinDecidir} posible(s) duplicado(s) («Por confirmar»).</p>}
                            <div className="dat-plantilla">
                                <input
                                    type="text"
                                    maxLength={80}
                                    placeholder="Nombre para reutilizar este mapeo"
                                    aria-label="Nombre de la plantilla"
                                    value={nombrePlantilla}
                                    onChange={(e) => setNombrePlantilla(e.target.value)}
                                />
                                <button type="button" className="dp-btn-secondary" disabled={!nombrePlantilla.trim() || Boolean(ocupado)} onClick={guardarPlantilla}>
                                    Guardar como plantilla
                                </button>
                                {mensajePlantilla && <span className="dat-ok">{mensajePlantilla}</span>}
                            </div>
                        </>
                    )}

                    {ocupado && ocupado !== "plantilla" && (
                        <p className="imp-progreso" aria-live="polite">
                            {ocupado === "importando" ? "Registrando estudiantes… no cierres esta ventana." : "Procesando…"}
                        </p>
                    )}

                    <div className="dp-form-actions">
                        {paso === "resultado" ? (
                            <button type="button" className="dp-btn-primary" onClick={cerrar}>
                                Cerrar
                            </button>
                        ) : (
                            <>
                                <button
                                    type="button"
                                    className="dp-btn-secondary"
                                    disabled={Boolean(ocupado)}
                                    onClick={() => (paso === "archivo" ? cerrar() : setPaso(paso === "registros" ? "estructura" : "archivo"))}
                                >
                                    {paso === "archivo" ? "Cancelar" : "Atrás"}
                                </button>
                                {paso === "archivo" && (
                                    <button type="button" className="dp-btn-primary" disabled={Boolean(validarArchivo(archivo)) || Boolean(ocupado)} onClick={() => analizar(null)}>
                                        {ocupado === "analizando" ? "Analizando…" : "Analizar archivo"}
                                    </button>
                                )}
                                {paso === "estructura" && (
                                    <button type="button" className="dp-btn-primary" disabled={columnasUsadas.length === 0 || Boolean(ocupado)} onClick={revisar}>
                                        {ocupado === "revisando" ? "Revisando…" : "Revisar registros"}
                                    </button>
                                )}
                                {paso === "registros" && (
                                    <button type="button" className="dp-btn-primary" disabled={!puedeConfirmar} onClick={confirmar}>
                                        {ocupado === "importando" ? "Importando…" : `Registrar ${aRegistrar} estudiante(s)`}
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

export default ImportarDatosModal;

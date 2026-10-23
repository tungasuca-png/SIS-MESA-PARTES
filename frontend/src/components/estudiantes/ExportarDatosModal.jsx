import { useEffect, useMemo, useState } from "react";
import Icon from "../dashboard/Icon";
import {
    eliminarPlantillaDatos,
    exportarDatos,
    getCamposDatos,
    guardarPlantillaDatos,
    listPlantillasDatos,
} from "../../services/datosEstudiantesService";
import { estudianteError } from "../../utils/estudiantesErrors";
import { descargarBase64 } from "../../utils/descargas";
import "../dashboard/forms.css";
import "./importarEstudiantes.css";

// Exportación configurable de estudiantes: campos, orden, encabezados,
// formato y plantillas. Exporta TODOS los resultados que cumplen los filtros
// de la tabla (no solo la página visible); los datos los arma el backend.

const FORMATOS = [
    { valor: "xlsx", label: "Excel (.xlsx)" },
    { valor: "csv", label: "CSV (.csv)" },
    { valor: "tsv", label: "Texto separado por tabulador (.tsv)" },
    { valor: "json", label: "JSON (.json)" },
    { valor: "jsonl", label: "JSON Lines (.jsonl)" },
    { valor: "xml", label: "XML (.xml)" },
];

const NOMBRES_FILTRO = { q: "búsqueda", estado: "estado", nivel: "nivel", grado: "grado", seccion: "sección", turno: "turno" };

function resumenFiltros(filtros) {
    const partes = Object.entries(NOMBRES_FILTRO)
        .filter(([k]) => filtros?.[k])
        .map(([k, nombre]) => `${nombre}: «${filtros[k]}»`);
    return partes.length ? partes.join(", ") : "sin filtros (todos los estudiantes)";
}

function ExportarDatosModal({ filtros, onClose, onExported }) {
    const [campos, setCampos] = useState([]);
    const [columnas, setColumnas] = useState([]);
    const [formato, setFormato] = useState("xlsx");
    const [separador, setSeparador] = useState(",");
    const [codificacion, setCodificacion] = useState("utf-8-bom");
    const [proteger, setProteger] = useState(true);
    const [plantillas, setPlantillas] = useState([]);
    const [plantillaId, setPlantillaId] = useState("");
    const [nombrePlantilla, setNombrePlantilla] = useState("");
    const [agregar, setAgregar] = useState("");
    const [ocupado, setOcupado] = useState("");
    const [error, setError] = useState("");
    const [mensaje, setMensaje] = useState("");

    useEffect(() => {
        let activo = true;
        getCamposDatos()
            .then((data) => {
                if (!activo) return;
                setCampos(data.campos.filter((c) => c.exportable));
                setColumnas(data.columnas_tabla);
            })
            .catch((err) => activo && setError(estudianteError(err).message));
        listPlantillasDatos("EXPORTACION")
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
    const usadas = columnas.map((c) => c.clave);
    const esTexto = formato === "csv" || formato === "tsv";

    const mover = (i, d) => {
        const j = i + d;
        if (j < 0 || j >= columnas.length) return;
        const nuevas = [...columnas];
        [nuevas[i], nuevas[j]] = [nuevas[j], nuevas[i]];
        setColumnas(nuevas);
    };

    const aplicarPlantilla = (id) => {
        setPlantillaId(id);
        setMensaje("");
        const p = plantillas.find((x) => x.id === id);
        if (!p) return;
        const cfg = JSON.parse(p.configuracion);
        setColumnas(cfg.columnas);
        setFormato(cfg.formato || "xlsx");
        setSeparador(cfg.separador || ",");
        setCodificacion(cfg.codificacion || "utf-8-bom");
        setProteger(cfg.proteger_formulas ?? true);
    };

    const ejecutar = async (clave, fn) => {
        setOcupado(clave);
        setError("");
        setMensaje("");
        try {
            await fn();
        } catch (err) {
            setError(estudianteError(err).message);
        } finally {
            setOcupado("");
        }
    };

    const configuracion = () => ({
        columnas,
        formato,
        ...(esTexto ? { separador: formato === "csv" ? separador : undefined, codificacion, proteger_formulas: proteger } : {}),
    });

    const exportar = () =>
        ejecutar("exportando", async () => {
            const data = await exportarDatos({ ...filtros, ...configuracion() });
            descargarBase64(data.contenido_base64, data.nombre, data.mime);
            const protegidas = data.protegidas > 0 ? ` ${data.protegidas} celda(s) que empezaban con =, +, - o @ se marcaron como texto.` : "";
            setMensaje(`Se exportaron ${data.filas} estudiante(s).${protegidas}`);
            onExported?.(data);
        });

    const guardar = () =>
        ejecutar("guardando", async () => {
            const actual = plantillas.find((p) => p.id === plantillaId);
            const sobrescribir = actual && actual.nombre === nombrePlantilla.trim();
            const { plantilla } = await guardarPlantillaDatos({
                id: sobrescribir ? actual.id : undefined,
                expected_version: sobrescribir ? actual.version : undefined,
                tipo: "EXPORTACION",
                nombre: nombrePlantilla,
                formato,
                configuracion: JSON.stringify(configuracion()),
            });
            setPlantillas((ps) => [...ps.filter((p) => p.id !== plantilla.id), plantilla]);
            setPlantillaId(plantilla.id);
            setNombrePlantilla("");
            setMensaje(`Plantilla «${plantilla.nombre}» guardada.`);
        });

    const eliminar = () =>
        ejecutar("eliminando", async () => {
            await eliminarPlantillaDatos(plantillaId, "EXPORTACION");
            setPlantillas((ps) => ps.filter((p) => p.id !== plantillaId));
            setPlantillaId("");
            setMensaje("Plantilla eliminada.");
        });

    return (
        <div className="dp-modal-overlay" onClick={() => !ocupado && onClose()}>
            <div className="dp-modal est-modal imp-modal" role="dialog" aria-modal="true" aria-label="Exportar estudiantes" onClick={(e) => e.stopPropagation()}>
                <div className="dp-modal-header">
                    <h2>Exportar estudiantes</h2>
                    <button type="button" className="dp-modal-close" onClick={onClose} aria-label="Cerrar" disabled={Boolean(ocupado)}>
                        <Icon name="close" size={18} />
                    </button>
                </div>
                <div className="dp-form imp-cuerpo">
                    <p className="imp-nota-texto">
                        Se exportarán <strong>todos</strong> los estudiantes que cumplen los filtros de la tabla ({resumenFiltros(filtros)}), no solo la página visible.
                    </p>

                    {plantillas.length > 0 && (
                        <div className="dat-plantilla">
                            <label className="dat-campo">
                                Plantilla
                                <select value={plantillaId} onChange={(e) => aplicarPlantilla(e.target.value)}>
                                    <option value="">— Configuración actual —</option>
                                    {plantillas.map((p) => (
                                        <option key={p.id} value={p.id}>
                                            {p.nombre} ({p.formato})
                                        </option>
                                    ))}
                                </select>
                            </label>
                            {plantillaId && (
                                <button type="button" className="dp-btn-secondary" disabled={Boolean(ocupado)} onClick={eliminar}>
                                    Eliminar plantilla
                                </button>
                            )}
                        </div>
                    )}

                    <div className="dp-table-wrapper imp-tabla">
                        <table className="dp-table">
                            <thead>
                                <tr>
                                    <th>Orden</th>
                                    <th>Campo</th>
                                    <th>Encabezado en el archivo</th>
                                    <th />
                                </tr>
                            </thead>
                            <tbody>
                                {columnas.map((c, i) => (
                                    <tr key={c.clave}>
                                        <td className="imp-nowrap">
                                            <button type="button" className="dat-mini" aria-label={`Subir ${etiqueta(c.clave)}`} disabled={i === 0} onClick={() => mover(i, -1)}>
                                                ▲
                                            </button>
                                            <button
                                                type="button"
                                                className="dat-mini"
                                                aria-label={`Bajar ${etiqueta(c.clave)}`}
                                                disabled={i === columnas.length - 1}
                                                onClick={() => mover(i, 1)}
                                            >
                                                ▼
                                            </button>
                                        </td>
                                        <td>{etiqueta(c.clave)}</td>
                                        <td>
                                            <input
                                                type="text"
                                                maxLength={80}
                                                aria-label={`Encabezado de ${etiqueta(c.clave)}`}
                                                value={c.encabezado}
                                                onChange={(e) => setColumnas(columnas.map((x, j) => (j === i ? { ...x, encabezado: e.target.value } : x)))}
                                            />
                                        </td>
                                        <td>
                                            <button type="button" className="imp-link" onClick={() => setColumnas(columnas.filter((_, j) => j !== i))}>
                                                Quitar
                                            </button>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                    <div className="dat-plantilla">
                        <label className="dat-campo">
                            Agregar campo
                            <select value={agregar} onChange={(e) => setAgregar(e.target.value)}>
                                <option value="">Elegir…</option>
                                {Object.entries(grupos).map(([grupo, lista]) => (
                                    <optgroup key={grupo} label={grupo}>
                                        {lista.map((c) => (
                                            <option key={c.clave} value={c.clave} disabled={usadas.includes(c.clave)}>
                                                {c.etiqueta}
                                            </option>
                                        ))}
                                    </optgroup>
                                ))}
                            </select>
                        </label>
                        <button
                            type="button"
                            className="dp-btn-secondary"
                            disabled={!agregar}
                            onClick={() => {
                                const campo = campos.find((c) => c.clave === agregar);
                                setColumnas([...columnas, { clave: agregar, encabezado: agregar === "acciones" ? "Acciones" : campo?.etiqueta ?? agregar }]);
                                setAgregar("");
                            }}
                        >
                            Agregar
                        </button>
                    </div>

                    <fieldset className="dat-opciones">
                        <legend>Formato</legend>
                        <label className="dat-campo">
                            Formato de salida
                            <select value={formato} onChange={(e) => setFormato(e.target.value)}>
                                {FORMATOS.map((f) => (
                                    <option key={f.valor} value={f.valor}>
                                        {f.label}
                                    </option>
                                ))}
                            </select>
                        </label>
                        {formato === "csv" && (
                            <label className="dat-campo">
                                Separador
                                <select value={separador} onChange={(e) => setSeparador(e.target.value)}>
                                    <option value=",">Coma (,)</option>
                                    <option value=";">Punto y coma (;)</option>
                                    <option value="|">Barra (|)</option>
                                </select>
                            </label>
                        )}
                        {esTexto && (
                            <>
                                <label className="dat-campo">
                                    Codificación
                                    <select value={codificacion} onChange={(e) => setCodificacion(e.target.value)}>
                                        <option value="utf-8-bom">UTF-8 para Excel (con BOM)</option>
                                        <option value="utf-8">UTF-8</option>
                                        <option value="windows-1252">Windows-1252</option>
                                        <option value="iso-8859-1">ISO-8859-1</option>
                                    </select>
                                </label>
                                <label className="imp-confirmacion">
                                    <input type="checkbox" checked={proteger} onChange={(e) => setProteger(e.target.checked)} />
                                    <span>Proteger contra fórmulas: los valores que empiezan con =, +, - o @ llevan un apóstrofo para que Excel no los ejecute.</span>
                                </label>
                            </>
                        )}
                        {formato === "xlsx" && <p className="imp-nota-texto">En Excel todas las celdas son de texto: se conservan los ceros iniciales y nada se ejecuta como fórmula.</p>}
                    </fieldset>

                    <div className="dat-plantilla">
                        <input
                            type="text"
                            maxLength={80}
                            placeholder="Nombre para guardar esta configuración"
                            aria-label="Nombre de la plantilla de exportación"
                            value={nombrePlantilla}
                            onChange={(e) => setNombrePlantilla(e.target.value)}
                        />
                        <button type="button" className="dp-btn-secondary" disabled={!nombrePlantilla.trim() || columnas.length === 0 || Boolean(ocupado)} onClick={guardar}>
                            Guardar como plantilla
                        </button>
                    </div>

                    {error && (
                        <p className="dp-form-error" role="alert">
                            {error}
                        </p>
                    )}
                    {mensaje && (
                        <p className="dat-ok" role="status">
                            {mensaje}
                        </p>
                    )}

                    <div className="dp-form-actions">
                        <button type="button" className="dp-btn-secondary" onClick={onClose} disabled={Boolean(ocupado)}>
                            Cerrar
                        </button>
                        <button type="button" className="dp-btn-primary" onClick={exportar} disabled={columnas.length === 0 || Boolean(ocupado)}>
                            {ocupado === "exportando" ? "Exportando…" : "Exportar"}
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
}

export default ExportarDatosModal;

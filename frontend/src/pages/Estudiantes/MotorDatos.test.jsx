import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { httpError, renderEstudiantes } from "../../test/estudiantesTestUtils";
import ImportarDatosModal from "../../components/estudiantes/ImportarDatosModal";
import ExportarDatosModal from "../../components/estudiantes/ExportarDatosModal";
import * as datos from "../../services/datosEstudiantesService";
import * as estudiantes from "../../services/estudiantesService";
import * as descargas from "../../utils/descargas";
import { informeCSV } from "../../utils/informeDatos";

// Motor universal de datos en la interfaz. HTTP simulado; datos ficticios.
vi.mock("../../services/datosEstudiantesService", () => ({
    FORMATOS_IMPORTACION: ".xlsx,.csv",
    MAX_BYTES_IMPORTACION: 3 * 1024 * 1024,
    getCamposDatos: vi.fn(),
    analizarDatos: vi.fn(),
    confirmarDatos: vi.fn(),
    exportarDatos: vi.fn(),
    listPlantillasDatos: vi.fn(),
    guardarPlantillaDatos: vi.fn(),
    eliminarPlantillaDatos: vi.fn(),
}));
vi.mock("../../services/estudiantesService", () => ({
    XLSX_MIME: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    searchEstudiantes: vi.fn(),
    descargarPlantillaEstudiantes: vi.fn(),
}));
vi.mock("../../utils/descargas", async (original) => ({ ...(await original()), descargarBase64: vi.fn() }));
vi.mock("../../components/portal/RoleLayout", () => ({ default: ({ children }) => <div>{children}</div> }));

const CAMPOS = {
    campos: [
        { clave: "codigo", etiqueta: "Código", grupo: "Estudiante", importable: true, exportable: true },
        { clave: "numero_documento", etiqueta: "Número de documento", grupo: "Estudiante", importable: true, exportable: true },
        { clave: "documento", etiqueta: "Documento (tipo y número)", grupo: "Estudiante", importable: true, exportable: true },
        { clave: "apellidos", etiqueta: "Apellidos", grupo: "Estudiante", importable: true, exportable: true },
        { clave: "nombres", etiqueta: "Nombres", grupo: "Estudiante", importable: true, exportable: true },
        { clave: "nombre_completo", etiqueta: "Apellidos y nombres", grupo: "Estudiante", importable: true, exportable: true },
        { clave: "grado", etiqueta: "Grado", grupo: "Registro académico (estudiante)", importable: true, exportable: true },
        { clave: "estado_matricula", etiqueta: "Estado de matrícula", grupo: "Matrícula vigente", importable: false, exportable: true },
        { clave: "acciones", etiqueta: "Acciones (columna vacía de compatibilidad)", grupo: "Compatibilidad", importable: false, exportable: true },
    ],
    formatos_entrada: ["csv"],
    formatos_salida: ["xlsx", "csv", "json"],
    columnas_tabla: [
        { clave: "codigo", encabezado: "Código" },
        { clave: "nombre_completo", encabezado: "Estudiante" },
    ],
};

const columna = (i, encabezado, campo, confianza = "ALTA", extra = {}) => ({
    columna: i, encabezado, campo, confianza, motivo: "", alternativas: [], muestras: ["x"], confirmada: false, importable: true, ...extra,
});

const ANALISIS = {
    formato: "csv", extension: "csv", hojas: [], hoja: "", rutas: [], ruta: "", separador: ";", codificacion: "utf-8", fila_encabezado: 1,
    avisos_archivo: [], plantilla_avisos: [], problemas_mapeo: [], requiere_confirmar_mapeo: true, bloqueantes: [], avisos: [],
    columnas: [
        columna(0, "Nro. Doc", "numero_documento"),
        columna(1, "Ap.", "apellidos", "MEDIA", { motivo: "encabezado parecido a «Apellidos»: confirme" }),
        columna(2, "Nombre(s)", "nombres"),
        columna(3, "Teléfono", ""),
        columna(4, "Estado de matrícula", "estado_matricula", "ALTA", { importable: false }),
    ],
    resumen: { total: 0 }, registros: [], huella: "h0",
};

const registro = (fila, categoria, extra = {}) => ({
    fila, categoria, codigo: "", documento: "DNI 0123456" + fila, apellidos: "Núñez", nombres: `Alumno ${fila}`, estado: "ACTIVO",
    nivel: "", grado: "", seccion: "", turno: "", errores: [], avisos: [], correcciones: [], coincidencia: "", resultado: "", ...extra,
});

const PREVIA = {
    ...ANALISIS,
    requiere_confirmar_mapeo: false,
    huella: "h-previa",
    resumen: { total: 4, nuevos: 2, existentes: 0, duplicados: 0, por_confirmar: 1, conflictos: 0, invalidos: 1, con_avisos: 1 },
    registros: [
        registro(2, "NUEVO"),
        registro(3, "NUEVO", { correcciones: ["tipo de documento DNI deducido (8 dígitos)"] }),
        registro(4, "POR_CONFIRMAR", { avisos: ["hay 1 estudiante(s) registrado(s) con el mismo nombre"] }),
        registro(5, "INVALIDO", { errores: ["condición «DOCENTE»: solo se importan estudiantes"] }),
    ],
};

const archivo = () => new File(["dni;nombre"], "alumnos.csv", { type: "text/csv" });

beforeEach(() => {
    vi.clearAllMocks();
    datos.getCamposDatos.mockResolvedValue(CAMPOS);
    datos.listPlantillasDatos.mockResolvedValue({ plantillas: [] });
    estudiantes.searchEstudiantes.mockResolvedValue({ estudiantes: [], total: 0 });
});
afterEach(cleanup);

describe("Asistente de importación", () => {
    it("analiza, permite corregir el mapeo, revisa sin escribir y exige resolver antes de confirmar", async () => {
        datos.analizarDatos.mockResolvedValueOnce(ANALISIS).mockResolvedValueOnce(PREVIA);
        let resolver;
        datos.confirmarDatos.mockReturnValue(new Promise((r) => (resolver = r)));
        const onImported = vi.fn();
        render(<ImportarDatosModal onClose={vi.fn()} onImported={onImported} />);

        const f = archivo();
        fireEvent.change(screen.getByLabelText("Archivo de estudiantes"), { target: { files: [f] } });
        fireEvent.click(screen.getByRole("button", { name: "Analizar archivo" }));
        expect(await screen.findByText("CSV")).toBeTruthy();
        expect(datos.analizarDatos).toHaveBeenLastCalledWith(f, { plantilla_id: undefined });

        // La sugerencia dudosa se muestra para confirmar; el usuario la cambia.
        expect(screen.getByText("encabezado parecido a «Apellidos»: confirme")).toBeTruthy();
        expect(screen.getByText(/no se importa \(dato de matrícula\)/)).toBeTruthy();
        fireEvent.change(screen.getByLabelText("Campo para «Ap.»"), { target: { value: "nombre_completo" } });
        fireEvent.change(screen.getByLabelText("Campo para «Teléfono»"), { target: { value: "" } });
        fireEvent.click(screen.getByRole("button", { name: "Revisar registros" }));
        await screen.findByRole("radiogroup", { name: "Decisión para la fila 4" });
        const solicitud = datos.analizarDatos.mock.calls[1][1];
        expect(solicitud.mapeo_confirmado).toBe(true);
        expect(solicitud.mapeo).toEqual([
            { columna: 0, campo: "numero_documento" },
            { columna: 1, campo: "nombre_completo" },
            { columna: 2, campo: "nombres" },
        ]);
        expect(solicitud.opciones).toMatchObject({ separador: ";", fila_encabezado: 1 });

        // Bloqueado hasta excluir errores y decidir el posible duplicado.
        const registrar = () => screen.getByRole("button", { name: /Registrar \d+ estudiante/ });
        expect(registrar().disabled).toBe(true);
        fireEvent.click(screen.getByRole("checkbox"));
        expect(registrar().disabled).toBe(true);
        expect(screen.getByText("Falta decidir 1 posible(s) duplicado(s) («Por confirmar»).")).toBeTruthy();
        fireEvent.click(within(screen.getByRole("radiogroup", { name: "Decisión para la fila 4" })).getByLabelText("Es otra persona: importar"));
        expect(registrar().textContent).toBe("Registrar 3 estudiante(s)");
        expect(registrar().disabled).toBe(false);

        fireEvent.click(registrar());
        fireEvent.click(screen.getByRole("button", { name: "Importando…" }));
        expect(datos.confirmarDatos).toHaveBeenCalledTimes(1);
        expect(datos.confirmarDatos.mock.calls[0][1]).toMatchObject({ huella: "h-previa", excluir_invalidos: true, aceptadas: [4], omitidas: [] });
        expect(screen.queryByText(/Importación terminada/)).toBeNull();

        resolver({
            ...PREVIA,
            confirmada: true,
            resumen: { ...PREVIA.resumen, insertados: 3, omitidos: 1 },
            registros: PREVIA.registros.map((r) => ({ ...r, resultado: r.fila === 5 ? "OMITIDO" : "INSERTADO" })),
        });
        expect(await screen.findByText("Importación terminada: 3 estudiante(s) registrado(s), 1 omitido(s).")).toBeTruthy();
        fireEvent.click(screen.getByText("Cerrar", { selector: "button" }));
        expect(onImported).toHaveBeenCalledWith(expect.objectContaining({ insertados: 3 }));
    });

    it("informa errores de formato del backend y permite reintentar con otro archivo", async () => {
        datos.analizarDatos.mockRejectedValue(httpError(400, "formato_no_soportado: es un Excel 97-2003 binario (.xls); guárdelo como .xlsx"));
        render(<ImportarDatosModal onClose={vi.fn()} />);
        fireEvent.change(screen.getByLabelText("Archivo de estudiantes"), { target: { files: [archivo()] } });
        fireEvent.click(screen.getByRole("button", { name: "Analizar archivo" }));
        expect(await screen.findByText(/Formato no admitido: es un Excel 97-2003 binario/)).toBeTruthy();
        expect(screen.getByRole("button", { name: "Analizar archivo" }).disabled).toBe(false);
    });

    it("rechaza en el navegador un archivo vacío o de más de 3 MB", () => {
        render(<ImportarDatosModal onClose={vi.fn()} />);
        const grande = archivo();
        Object.defineProperty(grande, "size", { value: 4 * 1024 * 1024 });
        fireEvent.change(screen.getByLabelText("Archivo de estudiantes"), { target: { files: [grande] } });
        expect(screen.getByRole("alert").textContent).toContain("3 MB");
        expect(screen.getByRole("button", { name: "Analizar archivo" }).disabled).toBe(true);
    });

    it("un archivo o mapeo que cambió obliga a volver a revisar", async () => {
        datos.analizarDatos.mockResolvedValueOnce(ANALISIS).mockResolvedValueOnce({ ...PREVIA, resumen: { ...PREVIA.resumen, por_confirmar: 0, invalidos: 0 }, registros: [registro(2, "NUEVO")] });
        datos.confirmarDatos.mockRejectedValue(httpError(400, "importacion_cambiada: vuelva a analizarlo"));
        render(<ImportarDatosModal onClose={vi.fn()} />);
        fireEvent.change(screen.getByLabelText("Archivo de estudiantes"), { target: { files: [archivo()] } });
        fireEvent.click(screen.getByRole("button", { name: "Analizar archivo" }));
        fireEvent.click(await screen.findByRole("button", { name: "Revisar registros" }));
        fireEvent.click(await screen.findByRole("button", { name: "Registrar 2 estudiante(s)" }));
        expect(await screen.findByText(/cambió desde la vista previa/)).toBeTruthy();
        expect(screen.getByRole("button", { name: "Revisar registros" })).toBeTruthy();
    });
});

describe("Informe de errores", () => {
    it("CSV con BOM, separador ; y protección de fórmulas", () => {
        const csv = informeCSV([registro(2, "INVALIDO", { nombres: "=HYPERLINK(1)", errores: ["x; y"] })]);
        expect(csv.charCodeAt(0)).toBe(0xfeff);
        expect(csv).toContain("'=HYPERLINK(1)");
        expect(csv).toContain('"x; y"');
    });
});

describe("Exportación", () => {
    it("parte de las columnas de la tabla, permite agregar, reordenar, renombrar y elegir formato; usa los filtros de la tabla", async () => {
        datos.exportarDatos.mockResolvedValue({ nombre: "estudiantes_tungasuca_2026-10-23.csv", mime: "text/csv", contenido_base64: "YQ==", filas: 12, protegidas: 1 });
        render(<ExportarDatosModal filtros={{ q: "ana", estado: "ACTIVO", nivel: "", orden: "" }} onClose={vi.fn()} />);
        expect(await screen.findByDisplayValue("Estudiante")).toBeTruthy();
        expect(screen.getByText(/búsqueda: «ana», estado: «ACTIVO»/)).toBeTruthy();

        fireEvent.change(screen.getByLabelText("Agregar campo"), { target: { value: "acciones" } });
        fireEvent.click(screen.getByRole("button", { name: "Agregar" }));
        fireEvent.click(screen.getByRole("button", { name: "Subir Apellidos y nombres" }));
        fireEvent.change(screen.getByLabelText("Encabezado de Código"), { target: { value: "Cód." } });
        fireEvent.change(screen.getByLabelText("Formato de salida"), { target: { value: "csv" } });
        fireEvent.change(screen.getByLabelText("Separador"), { target: { value: ";" } });
        fireEvent.click(screen.getByRole("button", { name: "Exportar" }));

        await waitFor(() => expect(datos.exportarDatos).toHaveBeenCalled());
        expect(datos.exportarDatos.mock.calls[0][0]).toEqual({
            q: "ana", estado: "ACTIVO", nivel: "", orden: "",
            columnas: [
                { clave: "nombre_completo", encabezado: "Estudiante" },
                { clave: "codigo", encabezado: "Cód." },
                { clave: "acciones", encabezado: "Acciones" },
            ],
            formato: "csv", separador: ";", codificacion: "utf-8-bom", proteger_formulas: true,
        });
        expect(await screen.findByText(/Se exportaron 12 estudiante\(s\)\. 1 celda/)).toBeTruthy();
        expect(descargas.descargarBase64).toHaveBeenCalledWith("YQ==", "estudiantes_tungasuca_2026-10-23.csv", "text/csv");
    });

    it("sin resultados informa y no descarga nada", async () => {
        datos.exportarDatos.mockRejectedValue(httpError(400, "export_sin_resultados: no hay estudiantes"));
        render(<ExportarDatosModal filtros={{}} onClose={vi.fn()} />);
        fireEvent.click(await screen.findByRole("button", { name: "Exportar" }));
        expect(await screen.findByText(/No hay estudiantes con los filtros aplicados/)).toBeTruthy();
        expect(descargas.descargarBase64).not.toHaveBeenCalled();
    });
});

describe("Acciones en el directorio", () => {
    it("con solo estudiantes.view: exporta, no importa", async () => {
        renderEstudiantes("/estudiantes", { permissions: ["estudiantes.view"] });
        expect(screen.queryByRole("button", { name: "Importar" })).toBeNull();
        fireEvent.click(await screen.findByRole("button", { name: "Exportar" }));
        expect(screen.getByRole("dialog", { name: "Exportar estudiantes" })).toBeTruthy();
    });

    it("con estudiantes.create ofrece importar", async () => {
        renderEstudiantes("/estudiantes", { permissions: ["estudiantes.view", "estudiantes.create"] });
        fireEvent.click(await screen.findByRole("button", { name: "Importar" }));
        expect(screen.getByRole("dialog", { name: "Importar estudiantes" })).toBeTruthy();
    });
});

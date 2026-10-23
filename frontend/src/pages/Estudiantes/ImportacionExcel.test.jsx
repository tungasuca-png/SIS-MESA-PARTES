import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { httpError, renderEstudiantes } from "../../test/estudiantesTestUtils";
import ImportarEstudiantesModal from "../../components/estudiantes/ImportarEstudiantesModal";
import * as service from "../../services/estudiantesService";
import * as descargas from "../../utils/descargas";

// Importación/exportación Excel en la interfaz. HTTP simulado; datos ficticios.
vi.mock("../../services/estudiantesService", () => ({
    XLSX_MIME: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    searchEstudiantes: vi.fn(),
    exportarEstudiantesExcel: vi.fn(),
    descargarPlantillaEstudiantes: vi.fn(),
    previsualizarImportacionEstudiantes: vi.fn(),
    confirmarImportacionEstudiantes: vi.fn(),
}));
vi.mock("../../utils/descargas", async (original) => ({ ...(await original()), descargarBase64: vi.fn() }));
vi.mock("../../components/portal/RoleLayout", () => ({ default: ({ children }) => <div>{children}</div> }));

const xlsx = (nombre = "estudiantes.xlsx", size = 2048) => {
    const f = new File(["x"], nombre, { type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" });
    Object.defineProperty(f, "size", { value: size });
    return f;
};

const fila = (n, resultado, extra = {}) => ({
    fila: n,
    tipo_documento: "DNI",
    numero_documento: `0123456${n}`,
    apellido_paterno: "Núñez",
    apellido_materno: "",
    nombres: `Alumno ${n}`,
    estado: "ACTIVO",
    nivel: "PRIMARIA",
    resultado,
    errores: [],
    avisos: [],
    ...extra,
});

const PREVIA = {
    huella: "h-123",
    confirmada: false,
    resumen: { total: 3, validas: 2, con_errores: 1, duplicadas: 0, ya_existen: 0, insertadas: 0, omitidas: 0, rechazadas: 0 },
    filas: [fila(2, "VALIDA"), fila(3, "VALIDA"), fila(4, "ERROR", { errores: ["CONDICION debe ser ESTUDIANTE"] })],
};
const FINAL = {
    huella: "h-123",
    confirmada: true,
    resumen: { total: 3, validas: 0, con_errores: 0, duplicadas: 0, ya_existen: 0, insertadas: 2, omitidas: 0, rechazadas: 1 },
    filas: [fila(2, "INSERTADA"), fila(3, "INSERTADA"), fila(4, "RECHAZADA", { errores: ["CONDICION debe ser ESTUDIANTE"] })],
};

const elegirArchivo = (archivo) => fireEvent.change(screen.getByLabelText("Archivo Excel de estudiantes"), { target: { files: [archivo] } });

beforeEach(() => {
    vi.clearAllMocks();
    service.searchEstudiantes.mockResolvedValue({ estudiantes: [], total: 0 });
});
afterEach(cleanup);

describe("Modal de importación", () => {
    it("rechaza en el navegador un archivo que no es .xlsx o pesa más de 1 MB, sin llamar al backend", () => {
        render(<ImportarEstudiantesModal onClose={vi.fn()} />);
        elegirArchivo(xlsx("alumnos.xls"));
        expect(screen.getByRole("alert").textContent).toContain(".xlsx");
        expect(screen.getByRole("button", { name: "Validar archivo" }).disabled).toBe(true);
        elegirArchivo(xlsx("grande.xlsx", 2 * 1024 * 1024));
        expect(screen.getByRole("alert").textContent).toContain("1 MB");
        expect(service.previsualizarImportacionEstudiantes).not.toHaveBeenCalled();
    });

    it("previsualiza, exige confirmación explícita, confirma una sola vez y muestra el resultado del backend", async () => {
        const onImported = vi.fn();
        const onClose = vi.fn();
        service.previsualizarImportacionEstudiantes.mockResolvedValue(PREVIA);
        let resolver;
        service.confirmarImportacionEstudiantes.mockReturnValue(new Promise((r) => (resolver = r)));
        render(<ImportarEstudiantesModal onClose={onClose} onImported={onImported} />);

        const archivo = xlsx();
        elegirArchivo(archivo);
        expect(screen.getByText("estudiantes.xlsx")).toBeTruthy();
        fireEvent.click(screen.getByRole("button", { name: "Validar archivo" }));
        expect(await screen.findByText("CONDICION debe ser ESTUDIANTE")).toBeTruthy();
        expect(service.previsualizarImportacionEstudiantes).toHaveBeenCalledWith(archivo);

        const importar = screen.getByRole("button", { name: "Importar 2 estudiante(s)" });
        expect(importar.disabled).toBe(true); // falta la confirmación explícita
        fireEvent.click(screen.getByRole("checkbox"));
        expect(importar.disabled).toBe(false);

        fireEvent.click(importar);
        fireEvent.click(screen.getByRole("button", { name: "Importando…" })); // doble clic
        expect(service.confirmarImportacionEstudiantes).toHaveBeenCalledTimes(1);
        expect(service.confirmarImportacionEstudiantes).toHaveBeenCalledWith(archivo, "h-123");
        // Nada se da por importado antes de la respuesta.
        expect(screen.queryByText(/Importación terminada/)).toBeNull();

        resolver(FINAL);
        expect(await screen.findByText("Importación terminada: 2 estudiante(s) registrado(s).")).toBeTruthy();
        fireEvent.click(screen.getByText("Cerrar", { selector: "button" }));
        expect(onImported).toHaveBeenCalledWith(FINAL.resumen);
        expect(onClose).toHaveBeenCalled();
    });

    it("sin filas válidas no permite confirmar", async () => {
        service.previsualizarImportacionEstudiantes.mockResolvedValue({
            ...PREVIA,
            resumen: { ...PREVIA.resumen, validas: 0, con_errores: 3 },
            filas: [fila(2, "ERROR", { errores: ["NIVEL inválido"] })],
        });
        render(<ImportarEstudiantesModal onClose={vi.fn()} />);
        elegirArchivo(xlsx());
        fireEvent.click(screen.getByRole("button", { name: "Validar archivo" }));
        expect(await screen.findByText(/No hay filas válidas/)).toBeTruthy();
        expect(screen.getByRole("checkbox").disabled).toBe(true);
        expect(screen.getByRole("button", { name: "Importar 0 estudiante(s)" }).disabled).toBe(true);
    });

    it("muestra el error del backend (encabezados) y permite reintentar", async () => {
        service.previsualizarImportacionEstudiantes.mockRejectedValueOnce(
            httpError(400, "encabezados_invalidos: la columna 3 debe llamarse \"DNI\" (se encontró \"DOC\")")
        );
        render(<ImportarEstudiantesModal onClose={vi.fn()} />);
        elegirArchivo(xlsx());
        fireEvent.click(screen.getByRole("button", { name: "Validar archivo" }));
        expect(await screen.findByText(/Encabezados incorrectos: la columna 3 debe llamarse/)).toBeTruthy();
        expect(screen.getByRole("button", { name: "Validar archivo" }).disabled).toBe(false);
    });

    it("si el archivo cambió desde la vista previa, obliga a validar de nuevo", async () => {
        service.previsualizarImportacionEstudiantes.mockResolvedValue(PREVIA);
        service.confirmarImportacionEstudiantes.mockRejectedValue(httpError(400, "importacion_cambiada: el archivo no es el mismo"));
        render(<ImportarEstudiantesModal onClose={vi.fn()} />);
        elegirArchivo(xlsx());
        fireEvent.click(screen.getByRole("button", { name: "Validar archivo" }));
        fireEvent.click(await screen.findByRole("checkbox"));
        fireEvent.click(screen.getByRole("button", { name: "Importar 2 estudiante(s)" }));
        expect(await screen.findByText(/cambió desde la vista previa/)).toBeTruthy();
        expect(screen.getByRole("button", { name: "Validar archivo" })).toBeTruthy();
    });

    it("descarga la plantilla generada por el backend", async () => {
        service.descargarPlantillaEstudiantes.mockResolvedValue({ nombre: "plantilla_estudiantes.xlsx", contenido_base64: "UEs=" });
        render(<ImportarEstudiantesModal onClose={vi.fn()} />);
        fireEvent.click(screen.getByRole("button", { name: "Descargar plantilla" }));
        await waitFor(() =>
            expect(descargas.descargarBase64).toHaveBeenCalledWith("UEs=", "plantilla_estudiantes.xlsx", service.XLSX_MIME)
        );
    });
});

describe("Acciones Excel en el directorio", () => {
    it("con solo estudiantes.view: exporta, pero no ofrece importar", async () => {
        service.exportarEstudiantesExcel.mockResolvedValue({ nombre: "estudiantes_2026-10-23.xlsx", contenido_base64: "UEs=", filas: 5 });
        renderEstudiantes("/estudiantes", { permissions: ["estudiantes.view"] });
        expect(screen.queryByRole("button", { name: "Importar Excel" })).toBeNull();
        fireEvent.click(await screen.findByRole("button", { name: "Exportar Excel" }));
        expect(await screen.findByText("Se exportaron 5 estudiante(s).")).toBeTruthy();
        expect(descargas.descargarBase64).toHaveBeenCalledWith("UEs=", "estudiantes_2026-10-23.xlsx", service.XLSX_MIME);
    });

    it("con estudiantes.create ofrece importar", async () => {
        renderEstudiantes("/estudiantes", { permissions: ["estudiantes.view", "estudiantes.create"] });
        fireEvent.click(await screen.findByRole("button", { name: "Importar Excel" }));
        expect(screen.getByRole("dialog", { name: "Importar estudiantes desde Excel" })).toBeTruthy();
    });

    it("informa el error de exportación del backend", async () => {
        service.exportarEstudiantesExcel.mockRejectedValue(httpError(400, "export_demasiados: hay más de 10000 estudiantes; filtre por estado o nivel"));
        renderEstudiantes("/estudiantes", { permissions: ["estudiantes.view"] });
        fireEvent.click(await screen.findByRole("button", { name: "Exportar Excel" }));
        expect(await screen.findByText(/Demasiados registros: hay más de 10000 estudiantes/)).toBeTruthy();
    });
});

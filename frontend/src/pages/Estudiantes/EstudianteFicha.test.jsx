import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import { estudianteBasico, httpError, renderEstudiantes } from "../../test/estudiantesTestUtils";
import * as service from "../../services/estudiantesService";
import * as psicologia from "../../services/psicologiaService";
import * as usuarios from "../../services/usuariosService";

// Ficha integral (etapa 6): pestañas con su estado real e integración con
// los endpoints existentes. Datos ficticios; los servicios HTTP se simulan.
vi.mock("../../services/estudiantesService", () => ({
    getEstudiante: vi.fn(),
    getEstudiantesBasic: vi.fn(),
    getEstudiantePersonal: vi.fn(),
    getMatriculasEstudiante: vi.fn(),
    getFamiliaresEstudiante: vi.fn(),
    getHistorialEstudiante: vi.fn(),
    vincularFamiliar: vi.fn(),
    desvincularFamiliar: vi.fn(),
    actualizarFamiliar: vi.fn(),
    cambiarParentescoFamiliar: vi.fn(),
    searchPersonas: vi.fn(),
    updatePersona: vi.fn(),
    listPeriodos: vi.fn(),
    listSecciones: vi.fn(),
    matricularEstudiante: vi.fn(),
    cambiarSeccionMatricula: vi.fn(),
    retirarMatricula: vi.fn(),
    reingresarMatricula: vi.fn(),
    concluirMatricula: vi.fn(),
}));
vi.mock("../../services/psicologiaService", () => ({ getExpedientePsicologicoByEstudiante: vi.fn(), listAtenciones: vi.fn() }));
vi.mock("../../services/usuariosService", () => ({ getUsuariosBasic: vi.fn() }));
vi.mock("../../components/portal/RoleLayout", () => ({
    default: ({ title, children }) => (
        <div>
            <h1>{title}</h1>
            {children}
        </div>
    ),
}));

const basico = estudianteBasico();
const ruta = `/estudiantes/${basico.id}`;
const P2026 = { id: "p-2026", anio: 2026, nombre: "Año escolar 2026", estado: "EN_CURSO" };
const P2025 = { id: "p-2025", anio: 2025, nombre: "Año escolar 2025", estado: "CERRADO" };
const SEC_A = { id: "s-a", periodo_id: "p-2026", nivel: "PRIMARIA", grado: "3", seccion: "A", turno: "MAÑANA", matriculados: 10, vacantes: 30 };
const SEC_B = { id: "s-b", periodo_id: "p-2026", nivel: "PRIMARIA", grado: "3", seccion: "B", turno: "MAÑANA", matriculados: 5 };

const matricula = (overrides = {}) => ({
    id: "m-2026",
    estudiante_id: basico.id,
    periodo: P2026,
    seccion: SEC_A,
    estado_matricula: "MATRICULADO",
    fecha_matricula: "2026-03-01",
    fecha_retiro: "",
    origen: "REGISTRO",
    version: 3,
    movimientos: [],
    ...overrides,
});

const pestana = async (nombre) => fireEvent.click(await screen.findByRole("tab", { name: new RegExp(nombre) }));

beforeEach(() => {
    vi.clearAllMocks();
    service.getEstudiante.mockResolvedValue({ estudiante: basico });
    service.getEstudiantesBasic.mockResolvedValue({ estudiantes: [basico] });
    service.getMatriculasEstudiante.mockResolvedValue({ matriculas: [] });
    service.getFamiliaresEstudiante.mockResolvedValue({ familiares: [] });
    service.getHistorialEstudiante.mockResolvedValue({ eventos: [], total: 0 });
    service.listPeriodos.mockResolvedValue({ periodos: [P2026, P2025] });
    service.listSecciones.mockResolvedValue({ secciones: [SEC_A, SEC_B] });
    usuarios.getUsuariosBasic.mockResolvedValue({ usuarios: [{ id: "u-1", nombre_completo: "Secretaría Uno" }] });
});
afterEach(cleanup);

describe("Ficha integral — pestañas", () => {
    it("muestra las 9 pestañas con su estado real", async () => {
        renderEstudiantes(ruta);
        const tabs = await screen.findAllByRole("tab");
        expect(tabs.map((t) => t.textContent)).toEqual([
            "Información general",
            "Padres y apoderadosParcial",
            "Historial académico",
            "NotasPróximamente",
            "Atenciones psicológicas",
            "Permisos y justificacionesPróximamente",
            "Documentos adjuntosPróximamente",
            "AsistenciaPróximamente",
            "Historial del estudiante",
        ]);
        expect(screen.getByRole("tab", { name: "Información general" }).getAttribute("aria-selected")).toBe("true");
    });

    it.each([
        ["Notas", /servicio de calificaciones/],
        ["Permisos y justificaciones", /módulo de permisos y justificaciones/],
        ["Documentos adjuntos", /no los asocia a un estudiante/],
        ["Asistencia", /servicio de asistencia/],
    ])("«%s» explica que no está disponible, sin datos ni botones", async (nombre, motivo) => {
        renderEstudiantes(ruta);
        await pestana(nombre);
        const panel = screen.getByRole("tabpanel");
        expect(within(panel).getByText(motivo)).toBeTruthy();
        expect(within(panel).getByText("Próximamente")).toBeTruthy();
        expect(within(panel).queryAllByRole("button")).toHaveLength(0);
        expect(within(panel).queryByRole("table")).toBeNull();
    });

    it("abre la pestaña indicada en la URL (?tab=) sin consultar la auditoría", async () => {
        renderEstudiantes(`${ruta}?tab=historial`);
        expect(await screen.findByRole("heading", { name: "Trayectoria académica" })).toBeTruthy();
        expect(service.getHistorialEstudiante).not.toHaveBeenCalled();
        expect(usuarios.getUsuariosBasic).not.toHaveBeenCalled();
    });

    it("«Ver historial» de la cabecera abre directamente la trayectoria académica", async () => {
        renderEstudiantes(ruta);
        fireEvent.click(await screen.findByRole("button", { name: "Ver historial" }));
        expect(await screen.findByRole("heading", { name: "Trayectoria académica" })).toBeTruthy();
        expect(screen.getByRole("tab", { name: "Historial del estudiante" }).getAttribute("aria-selected")).toBe("true");
    });
});

describe("Ficha integral — historial del estudiante (trayectoria académica)", () => {
    const trayectoria = () => [
        matricula({
            id: "m-2026",
            fecha_matricula: "2026-03-01",
            estado_matricula: "TRASLADADO",
            fecha_retiro: "2026-07-01",
            movimientos: [
                { tipo: "CAMBIO_TURNO", fecha: "2026-04-01", seccion_anterior_id: "s-b", seccion_nueva_id: "s-a" },
                { tipo: "RETIRO", fecha: "2026-05-10", seccion_anterior_id: "s-a" },
                { tipo: "REINGRESO", fecha: "2026-06-01", seccion_nueva_id: "s-a" },
                { tipo: "TRASLADO_SALIDA", fecha: "2026-07-01", seccion_anterior_id: "s-a" },
            ],
        }),
        matricula({
            id: "m-2025",
            periodo: P2025,
            seccion: SEC_B,
            fecha_matricula: "2025-03-03",
            estado_matricula: "CONCLUIDO",
            movimientos: [{ tipo: "CONCLUSION", fecha: "2025-12-19", seccion_anterior_id: "s-b" }],
        }),
    ];

    it("muestra matrículas y movimientos en orden cronológico con fecha, tipo, periodo y detalle", async () => {
        service.getMatriculasEstudiante.mockResolvedValue({ matriculas: trayectoria() });
        renderEstudiantes(`${ruta}?tab=historial`);
        const lista = await screen.findByRole("list", { name: "Trayectoria académica" });
        // Las etiquetas de sección (anterior/nueva) llegan de las secciones del periodo.
        await waitFor(() => expect(within(lista).getByText("De PRIMARIA · 3° · «B» · MAÑANA a PRIMARIA · 3° · «A» · MAÑANA")).toBeTruthy());

        const items = within(lista).getAllByRole("listitem");
        expect(items.map((li) => li.querySelector(".est-timeline-date").textContent)).toEqual([
            "2025-03-03",
            "2025-12-19",
            "2026-03-01",
            "2026-04-01",
            "2026-05-10",
            "2026-06-01",
            "2026-07-01",
        ]);
        expect(items.map((li) => li.querySelector(".est-timeline-title").textContent)).toEqual([
            "Matrícula · Año escolar 2025",
            "Conclusión del periodo · Año escolar 2025",
            "Matrícula · Año escolar 2026",
            "Cambio de turno · Año escolar 2026",
            "Retiro · Año escolar 2026",
            "Reingreso · Año escolar 2026",
            "Traslado (salida) · Año escolar 2026",
        ]);
        expect(within(items[0]).getByText("PRIMARIA · 3° · «B» · MAÑANA")).toBeTruthy();
        expect(within(items[0]).getByText("Situación actual de esta matrícula: Concluido")).toBeTruthy();
        expect(within(items[1]).getByText("Concluyó el periodo en PRIMARIA · 3° · «B» · MAÑANA")).toBeTruthy();
        expect(within(items[4]).getByText("Se retiró de PRIMARIA · 3° · «A» · MAÑANA")).toBeTruthy();
        expect(within(items[5]).getByText("Reingresó a PRIMARIA · 3° · «A» · MAÑANA")).toBeTruthy();
        expect(within(items[6]).getByText("Trasladado(a) a otra institución desde PRIMARIA · 3° · «A» · MAÑANA")).toBeTruthy();
    });

    it("no presenta auditoría técnica: ni campos modificados ni quién editó", async () => {
        service.getMatriculasEstudiante.mockResolvedValue({ matriculas: trayectoria() });
        service.getHistorialEstudiante.mockResolvedValue({
            total: 1,
            eventos: [{ id: "e1", accion: "UPDATE", fecha_hora: "2026-05-01T15:00:00Z", usuario_id: "u-1", campos: ["nombres"] }],
        });
        renderEstudiantes(`${ruta}?tab=historial`);
        await screen.findByRole("list", { name: "Trayectoria académica" });
        expect(screen.queryByText(/Cambios en el registro/)).toBeNull();
        expect(screen.queryByText(/Campos:/)).toBeNull();
        expect(screen.queryByText(/Secretaría Uno/)).toBeNull();
        expect(screen.queryByText(/^Por /)).toBeNull();
        expect(screen.queryByRole("button", { name: /cambios anteriores/ })).toBeNull();
        expect(service.getHistorialEstudiante).not.toHaveBeenCalled();
    });

    it("una matrícula sin fecha se ubica al inicio de su año y lo indica", async () => {
        service.getMatriculasEstudiante.mockResolvedValue({
            matriculas: [matricula({ id: "m-sin", fecha_matricula: "", seccion: null, movimientos: [{ tipo: "CAMBIO_SECCION", fecha: "2026-02-10", seccion_nueva_id: "s-a" }] })],
        });
        renderEstudiantes(`${ruta}?tab=historial`);
        const lista = await screen.findByRole("list", { name: "Trayectoria académica" });
        const items = within(lista).getAllByRole("listitem");
        expect(within(items[0]).getByText("Sin fecha registrada")).toBeTruthy();
        expect(within(items[0]).getByText("Sin sección asignada")).toBeTruthy();
        await waitFor(() => expect(within(items[1]).getByText("Sección asignada: PRIMARIA · 3° · «A» · MAÑANA")).toBeTruthy());
    });

    it("sin matrículas lo indica, y un error se muestra sin inventar eventos", async () => {
        renderEstudiantes(`${ruta}?tab=historial`);
        expect(await screen.findByText("Sin matrículas ni movimientos registrados.")).toBeTruthy();
        cleanup();

        service.getMatriculasEstudiante.mockRejectedValue(httpError(500, "x"));
        renderEstudiantes(`${ruta}?tab=historial`);
        expect(await screen.findByText(/Ocurrió un error en el servidor/)).toBeTruthy();
        expect(screen.queryByRole("list", { name: "Trayectoria académica" })).toBeNull();
    });
});

describe("Ficha integral — padres y apoderados", () => {
    const FAMILIA = ["estudiantes.view", "estudiantes.familia.view", "estudiantes.familia.update"];
    const padre = {
        relacion_id: "r-1",
        parentesco: "TIO_A",
        es_apoderado: false,
        vigente: true,
        desde: "2024-01-01",
        persona: { id: "per-1", version: 2, tipo_documento: "DNI", numero_documento: "40000001", nombres: "Pedro", apellido_paterno: "Ríos", apellido_materno: "" },
    };

    it("vincula una persona nueva con parentesco y rol de apoderado", async () => {
        service.vincularFamiliar.mockResolvedValue({ familiar: { ...padre, parentesco: "MADRE", persona: { ...padre.persona, nombres: "Rosa" } } });
        renderEstudiantes(ruta, { permissions: FAMILIA });
        await pestana("Padres y apoderados");
        fireEvent.click(await screen.findByRole("button", { name: "Vincular responsable" }));
        const dialog = screen.getByRole("dialog", { name: "Vincular responsable" });
        fireEvent.click(within(dialog).getByRole("radio", { name: "Registrar nueva persona" }));

        fireEvent.click(within(dialog).getByRole("button", { name: "Vincular" }));
        expect(within(dialog).getByText("Elige el parentesco.")).toBeTruthy();
        fireEvent.change(within(dialog).getByLabelText("Parentesco *"), { target: { value: "MADRE" } });
        fireEvent.change(within(dialog).getByLabelText("Número de documento"), { target: { value: "123" } });
        fireEvent.change(within(dialog).getByLabelText("Nombres *"), { target: { value: " Rosa " } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Vincular" }));
        expect(within(dialog).getByText("El DNI debe tener 8 dígitos.")).toBeTruthy();
        expect(service.vincularFamiliar).not.toHaveBeenCalled();

        fireEvent.change(within(dialog).getByLabelText("Número de documento"), { target: { value: "40000002" } });
        fireEvent.click(within(dialog).getByLabelText(/Es el apoderado/));
        fireEvent.click(within(dialog).getByRole("button", { name: "Vincular" }));
        await waitFor(() =>
            expect(service.vincularFamiliar).toHaveBeenCalledWith(basico.id, {
                personaId: undefined,
                persona: { tipo_documento: "DNI", numero_documento: "40000002", nombres: "Rosa", apellido_paterno: "", apellido_materno: "" },
                parentesco: "MADRE",
                esApoderado: true,
                desde: "",
            })
        );
        expect(await screen.findByText(/quedó vinculado\(a\) como madre/)).toBeTruthy();
        expect(service.getFamiliaresEstudiante).toHaveBeenCalledTimes(2);
    });

    it("busca y vincula una persona ya registrada (documento enmascarado)", async () => {
        service.searchPersonas.mockResolvedValue({ personas: [padre.persona], total: 1 });
        service.vincularFamiliar.mockResolvedValue({ familiar: padre });
        renderEstudiantes(ruta, { permissions: FAMILIA });
        await pestana("Padres y apoderados");
        fireEvent.click(await screen.findByRole("button", { name: "Vincular responsable" }));
        const dialog = screen.getByRole("dialog", { name: "Vincular responsable" });
        fireEvent.change(within(dialog).getByLabelText(/Buscar persona/), { target: { value: "ríos" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Buscar" }));
        expect(await within(dialog).findByText("Pedro Ríos")).toBeTruthy();
        expect(within(dialog).getByText(/••••0001/)).toBeTruthy();
        expect(service.searchPersonas).toHaveBeenCalledWith({ q: "ríos", pageSize: 10 });
        fireEvent.click(within(dialog).getByRole("radio", { name: /Pedro Ríos/ }));
        fireEvent.change(within(dialog).getByLabelText("Parentesco *"), { target: { value: "TIO_A" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Vincular" }));
        await waitFor(() => expect(service.vincularFamiliar).toHaveBeenCalledWith(basico.id, expect.objectContaining({ personaId: "per-1", persona: undefined })));
    });

    it("cambia el parentesco con el endpoint que conserva el historial", async () => {
        service.getFamiliaresEstudiante.mockResolvedValue({ familiares: [padre] });
        service.cambiarParentescoFamiliar.mockResolvedValue({ anterior: { ...padre, vigente: false }, familiar: { ...padre, relacion_id: "r-2", parentesco: "ABUELO_A" } });
        renderEstudiantes(ruta, { permissions: FAMILIA });
        await pestana("Padres y apoderados");
        fireEvent.click(await screen.findByRole("button", { name: "Cambiar parentesco" }));
        const dialog = screen.getByRole("dialog", { name: "Cambiar parentesco" });
        expect(within(dialog).queryByRole("option", { name: "Tío(a)" })).toBeNull();
        fireEvent.change(within(dialog).getByLabelText("Nuevo parentesco *"), { target: { value: "ABUELO_A" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Cambiar parentesco" }));
        await waitFor(() => expect(service.cambiarParentescoFamiliar).toHaveBeenCalledWith(basico.id, "r-1", { parentesco: "ABUELO_A", desde: "" }));
        expect(service.desvincularFamiliar).not.toHaveBeenCalled();
        expect(service.vincularFamiliar).not.toHaveBeenCalled();
        expect(await screen.findByText(/La relación anterior quedó en el historial/)).toBeTruthy();
    });

    it("designar apoderado: muestra la regla del backend si ya hay otro", async () => {
        service.getFamiliaresEstudiante.mockResolvedValue({ familiares: [padre] });
        service.actualizarFamiliar.mockRejectedValue(httpError(409, "duplicate_apoderado: ya tiene"));
        renderEstudiantes(ruta, { permissions: FAMILIA });
        await pestana("Padres y apoderados");
        fireEvent.click(await screen.findByRole("button", { name: "Designar apoderado" }));
        const dialog = screen.getByRole("dialog", { name: "Designar apoderado" });
        fireEvent.click(within(dialog).getByRole("button", { name: "Confirmar" }));
        expect(await within(dialog).findByText(/ya tiene un apoderado vigente/)).toBeTruthy();
        expect(service.actualizarFamiliar).toHaveBeenCalledWith(basico.id, "r-1", { esApoderado: true });
    });

    it("corrige los datos de la persona enviando solo lo cambiado con su versión", async () => {
        service.getFamiliaresEstudiante.mockResolvedValue({ familiares: [padre] });
        service.updatePersona.mockResolvedValue({ persona: { ...padre.persona, apellido_materno: "Soto", version: 3 } });
        renderEstudiantes(ruta, { permissions: FAMILIA });
        await pestana("Padres y apoderados");
        fireEvent.click(await screen.findByRole("button", { name: "Corregir datos" }));
        const dialog = screen.getByRole("dialog", { name: "Corregir datos de la persona" });
        fireEvent.change(within(dialog).getByLabelText("Apellido materno"), { target: { value: "Soto" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Guardar cambios" }));
        await waitFor(() => expect(service.updatePersona).toHaveBeenCalledWith("per-1", { apellido_materno: "Soto", expected_version: 2 }));
    });

    it("desvincula sin borrar (cierra la relación)", async () => {
        service.getFamiliaresEstudiante.mockResolvedValue({ familiares: [padre] });
        service.desvincularFamiliar.mockResolvedValue({ familiar: { ...padre, vigente: false, hasta: "2026-10-19" } });
        renderEstudiantes(ruta, { permissions: FAMILIA });
        await pestana("Padres y apoderados");
        fireEvent.click(await screen.findByRole("button", { name: "Desvincular" }));
        const dialog = screen.getByRole("dialog", { name: "Desvincular responsable" });
        expect(within(dialog).getByText(/No se borra/)).toBeTruthy();
        fireEvent.click(within(dialog).getByRole("button", { name: "Desvincular" }));
        await waitFor(() => expect(service.desvincularFamiliar).toHaveBeenCalledWith(basico.id, "r-1", { hasta: "" }));
    });
});

describe("Ficha integral — matrícula", () => {
    const MATRICULA = ["estudiantes.view", "estudiantes.matricula.update"];

    it("matricula en un periodo abierto en el que no está (secciones con ocupación)", async () => {
        service.getMatriculasEstudiante.mockResolvedValue({ matriculas: [matricula({ id: "m-25", periodo: P2025, estado_matricula: "CONCLUIDO" })] });
        const P2027 = { id: "p-2027", anio: 2027, nombre: "Año escolar 2027", estado: "PLANIFICADO" };
        service.listPeriodos.mockResolvedValue({ periodos: [P2027, P2026, P2025] });
        service.matricularEstudiante.mockResolvedValue({ matricula: matricula() });
        renderEstudiantes(ruta, { permissions: MATRICULA });
        await pestana("Historial académico");
        fireEvent.click(await screen.findByRole("button", { name: "Matricular" }));
        const dialog = screen.getByRole("dialog", { name: "Matricular en un periodo" });
        const periodo = within(dialog).getByLabelText("Periodo académico *");
        await waitFor(() => expect(periodo.options.length).toBe(3));
        // Sin el periodo cerrado.
        expect([...periodo.options].map((o) => o.value)).toEqual(["", "p-2027", "p-2026"]);
        fireEvent.change(periodo, { target: { value: "p-2026" } });
        const seccion = await within(dialog).findByLabelText("Sección");
        await waitFor(() => expect(seccion.options.length).toBe(3));
        expect(within(dialog).getByText(/10\/30 vacantes ocupadas/)).toBeTruthy();
        fireEvent.change(seccion, { target: { value: "s-a" } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Matricular" }));
        await waitFor(() =>
            expect(service.matricularEstudiante).toHaveBeenCalledWith(basico.id, { periodoId: "p-2026", seccionId: "s-a", fechaMatricula: "" })
        );
        expect(await screen.findByText("Matrícula registrada en Año escolar 2026.")).toBeTruthy();
        expect(service.getMatriculasEstudiante).toHaveBeenCalledTimes(2);
    });

    it("retiro exige motivo y muestra el error de regla del backend", async () => {
        service.getMatriculasEstudiante.mockResolvedValue({ matriculas: [matricula()] });
        service.retirarMatricula.mockRejectedValue(httpError(400, "periodo_cerrado: cerrado"));
        renderEstudiantes(ruta, { permissions: MATRICULA });
        await pestana("Historial académico");
        fireEvent.click(await screen.findByRole("button", { name: "Retiro o traslado" }));
        const dialog = screen.getByRole("dialog", { name: "Registrar retiro o traslado" });
        fireEvent.click(within(dialog).getByRole("button", { name: "Registrar" }));
        expect(within(dialog).getByText("Indica el motivo.")).toBeTruthy();
        fireEvent.click(within(dialog).getByRole("radio", { name: "Traslado a otra institución" }));
        fireEvent.change(within(dialog).getByLabelText("Motivo *"), { target: { value: " Cambio de domicilio " } });
        fireEvent.click(within(dialog).getByRole("button", { name: "Registrar" }));
        await waitFor(() =>
            expect(service.retirarMatricula).toHaveBeenCalledWith("m-2026", { expectedVersion: 3, fecha: "", tipo: "TRASLADO", motivo: "Cambio de domicilio" })
        );
        expect(await within(dialog).findByText("El periodo está cerrado y ya no admite cambios.")).toBeTruthy();
    });

    it("ofrece solo las operaciones válidas según estado y periodo", async () => {
        service.getMatriculasEstudiante.mockResolvedValue({
            matriculas: [
                matricula({ id: "m-ret", estado_matricula: "RETIRADO", fecha_retiro: "2026-06-01" }),
                matricula({ id: "m-cer", periodo: P2025, estado_matricula: "MATRICULADO" }),
            ],
        });
        renderEstudiantes(ruta, { permissions: MATRICULA });
        await pestana("Historial académico");
        expect(await screen.findByRole("button", { name: "Reingreso" })).toBeTruthy();
        // Periodo cerrado: solo concluir.
        expect(screen.getAllByRole("button", { name: "Concluir" })).toHaveLength(1);
        expect(screen.queryByRole("button", { name: "Cambiar sección" })).toBeNull();
        expect(screen.queryByRole("button", { name: "Retiro o traslado" })).toBeNull();
    });

    it("sin estudiantes.matricula.update es solo lectura", async () => {
        service.getMatriculasEstudiante.mockResolvedValue({ matriculas: [matricula()] });
        renderEstudiantes(ruta);
        await pestana("Historial académico");
        expect(await screen.findByText("Fecha de matrícula")).toBeTruthy();
        expect(screen.queryByRole("button", { name: "Matricular" })).toBeNull();
        expect(screen.queryByRole("button", { name: "Retiro o traslado" })).toBeNull();
    });
});

describe("Ficha integral — atenciones psicológicas", () => {
    const EXP = {
        id: "x-1",
        estudiante_id: basico.id,
        codigo: "PSI-2026-001",
        estado: "EN_SEGUIMIENTO",
        responsable_id: "u-1",
        fecha_apertura: "2026-04-01T10:00:00Z",
        resumen_caso: "TEXTO CLINICO RESERVADO",
        motivo_cierre: "MOTIVO RESERVADO",
    };
    const VER = ["estudiantes.view", "psicologia.expedientes.view"];
    const abrir = async (permissions) => {
        renderEstudiantes(ruta, { permissions });
        await pestana("Atenciones psicológicas");
    };

    it("1. autorizado con expediente: estado, apertura, responsable y acceso al módulo; nunca el resumen del caso", async () => {
        psicologia.getExpedientePsicologicoByEstudiante.mockResolvedValue({ expediente: EXP });
        await abrir(VER);
        expect(await screen.findByText("PSI-2026-001")).toBeTruthy();
        expect(screen.getByText("En seguimiento")).toBeTruthy();
        expect(screen.getByText("2026-04-01")).toBeTruthy();
        expect(await screen.findByText("Secretaría Uno")).toBeTruthy();
        expect(screen.getByRole("link", { name: /módulo de Psicología/ }).getAttribute("href")).toBe("/psicologia/expedientes/x-1");
        expect(screen.queryByText(/TEXTO CLINICO/)).toBeNull();
        expect(screen.queryByText(/MOTIVO RESERVADO/)).toBeNull();
        expect(psicologia.getExpedientePsicologicoByEstudiante).toHaveBeenCalledWith(basico.id);
        expect(psicologia.getExpedientePsicologicoByEstudiante).toHaveBeenCalledTimes(1);
    });

    it("2. autorizado (alcance general) sin expedientes: lo confirma solo tras un 404 autorizado", async () => {
        psicologia.getExpedientePsicologicoByEstudiante.mockRejectedValue(httpError(404, "expediente psicológico no encontrado"));
        await abrir(VER);
        expect(await screen.findByText("Este estudiante no tiene expedientes psicológicos registrados.")).toBeTruthy();
    });

    it("2b. con solo view_own un 404 no afirma que el estudiante no tenga expedientes", async () => {
        psicologia.getExpedientePsicologicoByEstudiante.mockRejectedValue(httpError(404, "no encontrado"));
        await abrir(["estudiantes.view", "psicologia.expedientes.view_own"]);
        expect(await screen.findByText("No hay expedientes psicológicos de este estudiante bajo tu responsabilidad.")).toBeTruthy();
        expect(screen.queryByText(/no tiene expedientes psicológicos registrados/)).toBeNull();
    });

    it("3. sin permiso de Psicología: mensaje fijo, sin consultas ni datos", async () => {
        await abrir(["estudiantes.view", "psicologia.atenciones.view"]);
        expect(screen.getByText("No tienes permisos para consultar esta información.")).toBeTruthy();
        expect(psicologia.getExpedientePsicologicoByEstudiante).not.toHaveBeenCalled();
        expect(psicologia.listAtenciones).not.toHaveBeenCalled();
        expect(screen.queryByText(/PSI-/)).toBeNull();
    });

    it("4. con permiso de expediente pero sin el de atenciones: no consulta el contenido clínico", async () => {
        psicologia.getExpedientePsicologicoByEstudiante.mockResolvedValue({ expediente: EXP });
        await abrir(VER);
        expect(await screen.findByText("PSI-2026-001")).toBeTruthy();
        expect(screen.getByText(/requiere la autorización específica de Psicología/)).toBeTruthy();
        expect(psicologia.listAtenciones).not.toHaveBeenCalled();
    });

    it("4b. con psicologia.atenciones.view lista las atenciones (número, fecha, estado) con enlace al módulo", async () => {
        psicologia.getExpedientePsicologicoByEstudiante.mockResolvedValue({ expediente: EXP });
        psicologia.listAtenciones.mockResolvedValue({
            total: 1,
            atenciones: [{ id: "a-1", numero: 3, estado: "REGISTRADA", fecha_atencion: "2026-05-02", motivos: [{ codigo: "X", nombre: "MOTIVO CLINICO" }] }],
        });
        await abrir([...VER, "psicologia.atenciones.view"]);
        const lista = await screen.findByRole("list", { name: "Atenciones registradas" });
        expect(psicologia.listAtenciones).toHaveBeenCalledWith("x-1", { pageSize: 10 });
        expect(within(lista).getByRole("link", { name: "Atención N.° 3" }).getAttribute("href")).toBe("/psicologia/atenciones/a-1");
        expect(within(lista).getByText(/2026-05-02 · Registrada/)).toBeTruthy();
        expect(screen.queryByText(/MOTIVO CLINICO/)).toBeNull();
    });

    it("4c. si Psicología deniega las atenciones (403) lo dice sin mostrar contenido", async () => {
        psicologia.getExpedientePsicologicoByEstudiante.mockResolvedValue({ expediente: EXP });
        psicologia.listAtenciones.mockRejectedValue(httpError(403, "el usuario no tiene el permiso requerido"));
        await abrir([...VER, "psicologia.atenciones.view"]);
        const bloque = (await screen.findByText("Atenciones registradas")).parentElement;
        expect(await within(bloque).findByText("No tienes permisos para consultar esta información.")).toBeTruthy();
    });

    it("5. sesión no autenticada (401): no se presenta como «sin expedientes»", async () => {
        psicologia.getExpedientePsicologicoByEstudiante.mockRejectedValue(httpError(401, "se requiere autenticación"));
        await abrir(VER);
        expect(await screen.findByText("Tu sesión no es válida. Inicia sesión nuevamente.")).toBeTruthy();
        expect(screen.queryByText(/no tiene expedientes/)).toBeNull();
    });

    it("6. servicio no disponible: mensaje de conexión y reintento sin duplicar solicitudes", async () => {
        const sinRed = new Error("Network Error");
        psicologia.getExpedientePsicologicoByEstudiante.mockRejectedValueOnce(sinRed).mockResolvedValueOnce({ expediente: EXP });
        await abrir(VER);
        expect(await screen.findByText("No se pudo conectar con el servicio de Psicología.")).toBeTruthy();
        expect(screen.queryByText(/no tiene expedientes/)).toBeNull();
        fireEvent.click(screen.getByRole("button", { name: "Reintentar" }));
        expect(await screen.findByText("PSI-2026-001")).toBeTruthy();
        expect(psicologia.getExpedientePsicologicoByEstudiante).toHaveBeenCalledTimes(2);
    });

    it.each([
        [403, "No tienes permisos para consultar esta información."],
        [500, "No se pudo conectar con el servicio de Psicología."],
        [503, "No se pudo conectar con el servicio de Psicología."],
    ])("7. respuesta %i del backend", async (status, mensaje) => {
        psicologia.getExpedientePsicologicoByEstudiante.mockRejectedValue(httpError(status, "x"));
        await abrir(VER);
        expect(await screen.findByText(mensaje)).toBeTruthy();
        expect(screen.queryByText(/no tiene expedientes/)).toBeNull();
        expect(screen.queryByText(/PSI-/)).toBeNull();
    });

    it("9. las demás pestañas siguen funcionando después de consultar Psicología", async () => {
        psicologia.getExpedientePsicologicoByEstudiante.mockResolvedValue({ expediente: EXP });
        await abrir(VER);
        await screen.findByText("PSI-2026-001");
        await pestana("Historial del estudiante");
        expect(await screen.findByRole("heading", { name: "Trayectoria académica" })).toBeTruthy();
        await pestana("Información general");
        expect(await screen.findByText("Identidad básica")).toBeTruthy();
    });
});

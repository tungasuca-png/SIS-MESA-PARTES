import { getExpedientes } from "../../services/expedientesService";
import { ESTADOS, TIPOS } from "../../constants/expedientes";

// Métricas del Dashboard calculadas SOLO con el endpoint real de listado de
// expedientes (GET /api/expedientes). No existe un endpoint de estadísticas:
// - Los conteos usan el campo "total" (COUNT del backend) con page_size=1.
// - La evolución diaria pagina el listado (ordenado por fecha_registro DESC)
//   hasta salir del periodo.
// El backend aplica el alcance de cada rol (ADMIN/SECRETARIA ven todo; el
// resto del personal, lo derivado a su área), así que todas las cifras ya
// vienen acotadas a lo que el usuario puede ver.

const contar = async (filtros) => (await getExpedientes({ ...filtros, page: 1, pageSize: 1 })).total ?? 0;

// Indicadores + expedientes recientes (se refrescan en vivo).
export async function cargarResumen({ recientes = 6 } = {}) {
    const [listado, porEstado, urgentes] = await Promise.all([
        getExpedientes({ page: 1, pageSize: recientes }),
        Promise.all(ESTADOS.map((estado) => contar({ estado: estado.value }))),
        // "Urgentes sin atender": PENDIENTE + EN_PROCESO con prioridad URGENTE.
        Promise.all(["PENDIENTE", "EN_PROCESO"].map((estado) => contar({ estado, prioridad: "URGENTE" }))),
    ]);
    return {
        total: listado.total ?? 0,
        porEstado: Object.fromEntries(ESTADOS.map((estado, i) => [estado.value, porEstado[i]])),
        urgentesAbiertos: urgentes[0] + urgentes[1],
        recientes: listado.expedientes || [],
    };
}

// Expedientes por tipo (todos los tipos válidos del backend), de mayor a
// menor, sin los tipos en cero.
export async function cargarPorTipo() {
    const totales = await Promise.all(TIPOS.map((tipo) => contar({ tipo: tipo.value })));
    return TIPOS.map((tipo, i) => ({ key: tipo.value, label: tipo.label, value: totales[i] }))
        .filter((item) => item.value > 0)
        .sort((a, b) => b.value - a.value);
}

export const PERIODOS = [
    { dias: 7, label: "7 días" },
    { dias: 30, label: "30 días" },
    { dias: 90, label: "90 días" },
];

const POR_PAGINA = 100; // máximo que acepta el backend
const MAX_PAGINAS = 10; // tope de seguridad: 1000 registros por consulta

function claveDia(fecha) {
    const y = fecha.getFullYear();
    const m = String(fecha.getMonth() + 1).padStart(2, "0");
    const d = String(fecha.getDate()).padStart(2, "0");
    return `${y}-${m}-${d}`;
}

function inicioDelPeriodo(dias, hoy) {
    const inicio = new Date(hoy.getFullYear(), hoy.getMonth(), hoy.getDate());
    inicio.setDate(inicio.getDate() - (dias - 1));
    return inicio;
}

// Cuenta registros por día (fecha local) en los últimos `dias` días,
// incluido hoy. Los días sin registros valen 0; las fechas inválidas o
// fuera del periodo se ignoran.
export function agruparPorDia(fechas, dias, hoy = new Date()) {
    const inicio = inicioDelPeriodo(dias, hoy);
    const puntos = [];
    const indice = new Map();
    for (let i = 0; i < dias; i += 1) {
        const dia = new Date(inicio);
        dia.setDate(inicio.getDate() + i);
        indice.set(claveDia(dia), puntos.length);
        puntos.push({ fecha: claveDia(dia), dia, total: 0 });
    }
    for (const valor of fechas) {
        const fecha = new Date(valor);
        if (Number.isNaN(fecha.getTime())) continue;
        const pos = indice.get(claveDia(fecha));
        if (pos !== undefined) puntos[pos].total += 1;
    }
    return puntos;
}

// Registros por día del periodo. `completo` es false si se alcanzó el tope
// de páginas antes de cubrir todo el periodo (la serie sería parcial y la
// interfaz lo avisa en vez de mostrarla como si fuera total).
export async function cargarSerieDiaria(dias, hoy = new Date()) {
    const inicio = inicioDelPeriodo(dias, hoy);
    const fechas = [];
    let completo = true;
    for (let pagina = 1; ; pagina += 1) {
        const { expedientes = [] } = await getExpedientes({ page: pagina, pageSize: POR_PAGINA });
        let salioDelPeriodo = false;
        for (const exp of expedientes) {
            const fecha = new Date(exp.fecha_registro);
            if (!Number.isNaN(fecha.getTime()) && fecha < inicio) {
                salioDelPeriodo = true;
                break;
            }
            fechas.push(exp.fecha_registro);
        }
        if (salioDelPeriodo || expedientes.length < POR_PAGINA) break;
        if (pagina >= MAX_PAGINAS) {
            completo = false;
            break;
        }
    }
    return { puntos: agruparPorDia(fechas, dias, hoy), completo };
}

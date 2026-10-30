// Protección de datos en pantalla. Solo presentación: el backend ya decide
// qué datos entrega a cada permiso; esto evita mostrar completo un documento
// o un contacto donde no hace falta (listados, cabeceras, capturas).

// enmascararDocumento("12345678") -> "••••5678". Muestra los últimos 4 (3 si
// el número es corto) para que quien atiende confirme que es la persona
// correcta sin exponer el número entero.
export function enmascararDocumento(numero) {
    const valor = (numero || "").trim();
    if (!valor) return "";
    const visibles = valor.length > 6 ? 4 : Math.min(3, Math.max(valor.length - 2, 1));
    return "•".repeat(Math.max(valor.length - visibles, 2)) + valor.slice(-visibles);
}

// enmascararCorreo("ana.perez@colegio.pe") -> "a•••@colegio.pe".
export function enmascararCorreo(correo) {
    const valor = (correo || "").trim();
    const at = valor.indexOf("@");
    if (at <= 0) return valor ? "•••" : "";
    return `${valor[0]}•••${valor.slice(at)}`;
}

// enmascararTelefono("987 654 321") -> "•••••321".
export function enmascararTelefono(telefono) {
    const digitos = (telefono || "").replace(/\D/g, "");
    if (!digitos) return "";
    return "•".repeat(Math.max(digitos.length - 3, 2)) + digitos.slice(-3);
}

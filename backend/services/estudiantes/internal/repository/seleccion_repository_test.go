package repository

import (
	"fmt"
	"strings"
	"testing"
)

// Las condiciones de la búsqueda de selección: solo ACTIVO, DNI exacto de
// tipo DNI, nombres/apellidos normalizados; nunca texto_busqueda (código y
// SIAGIE) ni valores concatenados en el SQL.
func TestCondicionesSeleccion(t *testing.T) {
	var args []any
	param := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	conds := condicionesSeleccion(SeleccionFilter{Nombres: []string{"quis%pe"}, DNIs: []string{"45127823"}}, param)
	sql := strings.Join(conds, " AND ")

	for _, debe := range []string{`estado = 'ACTIVO'`, `tipo_documento = 'DNI' AND numero_documento = $1`, `estudiantes_normalizar($2)`, `COALESCE(apellido_paterno, '')`, `COALESCE(apellido_materno, '')`} {
		if !strings.Contains(sql, debe) {
			t.Fatalf("falta %q en: %s", debe, sql)
		}
	}
	for _, prohibido := range []string{"texto_busqueda", "codigo", "siagie", "45127823", "quis"} {
		if strings.Contains(strings.ToLower(sql), prohibido) {
			t.Fatalf("el SQL no debe contener %q: %s", prohibido, sql)
		}
	}
	if len(args) != 2 || args[0] != "45127823" || args[1] != `quis\%pe` {
		t.Fatalf("parámetros inesperados (el comodín debe escaparse): %v", args)
	}
}

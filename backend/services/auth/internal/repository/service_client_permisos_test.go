package repository

import (
	"strings"
	"testing"
)

// Aislamiento: otorgar/retirar permisos de un sistema solo escribe en
// service_client_permissions; nunca en role_permissions, roles ni usuarios.
func TestSQLPermisosSistema_SoloTocaServiceClientPermissions(t *testing.T) {
	escrituras := map[string]string{"otorgar": sqlOtorgarPermiso, "retirar": sqlRetirarPermiso}
	for nombre, sql := range escrituras {
		s := strings.ToLower(sql)
		if !strings.Contains(s, "service_client_permissions") {
			t.Fatalf("%s debe operar sobre service_client_permissions", nombre)
		}
		for _, prohibido := range []string{"role_permissions", "usuario_roles", "update roles", "into roles", "from roles"} {
			if strings.Contains(s, prohibido) {
				t.Fatalf("%s no debe tocar %s", nombre, prohibido)
			}
		}
	}
	if !strings.HasPrefix(strings.TrimSpace(strings.ToLower(sqlRetirarPermiso)), "delete from service_client_permissions") {
		t.Fatal("retirar solo puede borrar de service_client_permissions")
	}
	if !strings.Contains(strings.ToLower(sqlOtorgarPermiso), "on conflict (service_client_id, permission_id) do nothing") {
		t.Fatal("otorgar debe ser idempotente (ON CONFLICT DO NOTHING)")
	}
	if !strings.HasPrefix(strings.ToLower(sqlPermisoCatalogado), "select") || !strings.Contains(sqlPermisoCatalogado, "estado = TRUE") {
		t.Fatal("solo se otorgan permisos activos del catálogo")
	}
}

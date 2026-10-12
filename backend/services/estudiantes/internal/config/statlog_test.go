package config

import (
	"os"
	"strings"
	"testing"

	"estudiantes/estudiantes"
)

// FASE E-05: todo RPC de Estudiantes lleva datos personales de menores, así
// que ninguno puede registrar su contenido en el stat log de go-zero. Si se
// agrega un RPC al proto sin excluirlo en etc/estudiantes.yaml, esta prueba
// falla.
func TestStatLog_TodosLosRPCExcluidos(t *testing.T) {
	raw, err := os.ReadFile("../../etc/estudiantes.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(raw)
	svc := estudiantes.File_estudiantes_service_proto.Services().Get(0)
	metodos := svc.Methods()
	for i := 0; i < metodos.Len(); i++ {
		full := "/" + string(svc.FullName()) + "/" + string(metodos.Get(i).Name())
		if !strings.Contains(cfg, "- "+full+"\n") && !strings.Contains(cfg, "- "+full+"\r\n") {
			t.Errorf("%s no está en Middlewares.StatConf.IgnoreContentMethods", full)
		}
	}
}

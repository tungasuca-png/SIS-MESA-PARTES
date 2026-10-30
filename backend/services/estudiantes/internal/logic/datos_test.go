package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"estudiantes/internal/repository"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"tabular"
)

func TestNormalizarConfiguracion_Importacion(t *testing.T) {
	cfg, _, err := normalizarConfiguracion(tipoImportacion, "", `{"opciones":{"separador":";"},"asociaciones":{"Nro. Doc":"numero_documento","Nombre(s)":"nombres"}}`)
	if err != nil {
		t.Fatal(err)
	}
	var c configImportacion
	_ = json.Unmarshal(cfg, &c)
	if c.Asociaciones["NRO DOC"] != "numero_documento" || c.Asociaciones["NOMBRE"] != "nombres" || c.Opciones.Separador != ";" {
		t.Fatalf("encabezados normalizados al guardar: %s", cfg)
	}
	for nombre, raw := range map[string]string{
		"campo inexistente":     `{"asociaciones":{"X":"password_hash"}}`,
		"campo no importable":   `{"asociaciones":{"Periodo":"matricula_periodo"}}`,
		"campo en dos columnas": `{"asociaciones":{"DNI":"numero_documento","Documento":"numero_documento"}}`,
		"sin asociaciones":      `{"asociaciones":{}}`,
		"clave desconocida":     `{"asociaciones":{"DNI":"numero_documento"},"script":"x"}`,
		"no es JSON":            `<xml/>`,
		"encabezado vacío":      `{"asociaciones":{"  ":"nombres"}}`,
	} {
		if _, _, err := normalizarConfiguracion(tipoImportacion, "", raw); status.Code(err) != codes.InvalidArgument {
			t.Errorf("%s: se esperaba InvalidArgument, got %v", nombre, err)
		}
	}
}

func TestNormalizarConfiguracion_Exportacion(t *testing.T) {
	cfg, formato, err := normalizarConfiguracion(tipoExportacion, "", `{"columnas":[{"clave":"codigo","encabezado":""},{"clave":"acciones","encabezado":"Acciones"}],"formato":"csv","separador":";","proteger_formulas":true}`)
	if err != nil || formato != "csv" {
		t.Fatalf("%v %s", err, formato)
	}
	var c configExportacion
	_ = json.Unmarshal(cfg, &c)
	if c.Columnas[0].Encabezado != "Código" || !c.ProtegerFormulas {
		t.Fatalf("encabezado por defecto y opciones: %s", cfg)
	}
	for nombre, raw := range map[string]string{
		"formato no admitido": `{"columnas":[{"clave":"codigo"}],"formato":"pdf"}`,
		"campo inventado":     `{"columnas":[{"clave":"clave_secreta"}],"formato":"xlsx"}`,
		"sin columnas":        `{"columnas":[],"formato":"xlsx"}`,
		"separador inválido":  `{"columnas":[{"clave":"codigo"}],"formato":"csv","separador":"#"}`,
	} {
		if _, _, err := normalizarConfiguracion(tipoExportacion, "", raw); status.Code(err) != codes.InvalidArgument {
			t.Errorf("%s: se esperaba InvalidArgument, got %v", nombre, err)
		}
	}
}

func TestHuellaImportacion_CubreArchivoOpcionesYMapeo(t *testing.T) {
	o := tabular.Origen{Formato: "csv", Separador: ";"}
	m := tabular.Mapeo{Asignaciones: []tabular.Asignacion{{Columna: 0, Campo: "nombres"}, {Columna: 1, Campo: "apellidos"}}}
	base := huellaImportacion("a", o, m)
	if base != huellaImportacion("a", o, m) {
		t.Fatal("determinista")
	}
	otroMapeo := tabular.Mapeo{Asignaciones: []tabular.Asignacion{{Columna: 0, Campo: "apellidos"}, {Columna: 1, Campo: "nombres"}}}
	otroOrigen := o
	otroOrigen.Separador = ","
	for nombre, h := range map[string]string{
		"otro archivo":   huellaImportacion("b", o, m),
		"otras opciones": huellaImportacion("a", otroOrigen, m),
		"otro mapeo":     huellaImportacion("a", o, otroMapeo),
	} {
		if h == base {
			t.Errorf("%s debe cambiar la huella", nombre)
		}
	}
}

func TestErrorDatos(t *testing.T) {
	log := logx.WithContext(t.Context())
	casos := []struct {
		err  error
		code codes.Code
	}{
		{&tabular.Error{Code: tabular.ErrNoSoportado, Msg: "xlsb"}, codes.InvalidArgument},
		{repository.ErrPlantillasSinTabla, codes.FailedPrecondition},
		{repository.ErrPlantillaNotFound, codes.NotFound},
		{repository.ErrPlantillaDuplicada, codes.AlreadyExists},
		{repository.ErrPlantillaVersion, codes.Aborted},
		{status.Error(codes.PermissionDenied, "x"), codes.PermissionDenied},
		{fmt.Errorf("pq: conexión rechazada en 127.0.0.1:5433: %w", errors.New("x")), codes.Internal},
	}
	for _, c := range casos {
		err := errorDatos(log, "prueba", c.err)
		if status.Code(err) != c.code {
			t.Errorf("%v -> %v, se esperaba %v", c.err, err, c.code)
		}
		if c.code == codes.Internal && status.Convert(err).Message() != "no se pudo procesar los datos" {
			t.Errorf("error interno sin detalles: %v", err)
		}
	}
}

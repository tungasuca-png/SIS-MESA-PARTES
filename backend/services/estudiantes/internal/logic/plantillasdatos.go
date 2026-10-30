package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"estudiantes/estudiantes"
	"estudiantes/internal/datos"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"tabular"
)

// Plantillas del motor de datos. Permisos por tipo: las de IMPORTACION
// exigen poder importar (estudiantes.create + view); las de EXPORTACION,
// estudiantes.view (son solo una configuración de salida).

func autorizarPlantilla(ctx context.Context, sc *svc.ServiceContext, tipo string) (string, error) {
	switch tipo {
	case tipoImportacion:
		return autorizarImportacion(ctx, sc)
	case tipoExportacion:
		return autorizar(ctx, sc, "estudiantes.view")
	}
	return "", status.Error(codes.InvalidArgument, "validation_error: tipo de plantilla inválido: use IMPORTACION o EXPORTACION")
}

func plantillaProto(p *repository.Plantilla) *estudiantes.PlantillaDatos {
	return &estudiantes.PlantillaDatos{
		Id: p.ID, Tipo: p.Tipo, Nombre: p.Nombre, Formato: p.Formato, Configuracion: string(p.Configuracion),
		Version: int32(p.Version), CreadoPor: p.CreadoPor, ActualizadoPor: p.ActualizadoPor, Creado: p.CreatedAt, Actualizado: p.UpdatedAt,
	}
}

// normalizarConfiguracion valida el JSON según el tipo y lo reescribe en
// forma canónica: solo campos conocidos, asociaciones a campos importables
// (con el encabezado normalizado) y columnas exportables.
func normalizarConfiguracion(tipo, formato, raw string) ([]byte, string, error) {
	invalida := func(msg string) error {
		return status.Error(codes.InvalidArgument, "plantilla_invalida: "+msg)
	}
	if len(raw) > 60000 {
		return nil, "", invalida("la configuración es demasiado grande")
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	switch tipo {
	case tipoImportacion:
		var cfg configImportacion
		if err := dec.Decode(&cfg); err != nil {
			return nil, "", invalida("configuración de importación con formato inesperado")
		}
		if len(cfg.Asociaciones) == 0 || len(cfg.Asociaciones) > 200 {
			return nil, "", invalida("debe tener entre 1 y 200 asociaciones confirmadas")
		}
		limpias := map[string]string{}
		usados := map[string]bool{}
		for enc, campo := range cfg.Asociaciones {
			n := tabular.NormalizarEncabezado(enc)
			c, ok := datos.Esquema.Campo(campo)
			if n == "" || !ok || !c.Importable {
				return nil, "", invalida("asociación inválida: «" + enc + "» → «" + campo + "»")
			}
			if usados[campo] {
				return nil, "", invalida("el campo «" + c.Etiqueta + "» está asociado a dos encabezados")
			}
			usados[campo], limpias[n] = true, campo
		}
		cfg.Asociaciones = limpias
		b, err := json.Marshal(cfg)
		return b, "", err
	case tipoExportacion:
		var cfg configExportacion
		if err := dec.Decode(&cfg); err != nil {
			return nil, "", invalida("configuración de exportación con formato inesperado")
		}
		pedidas := make([]tabular.ColumnaExport, 0, len(cfg.Columnas))
		for _, c := range cfg.Columnas {
			pedidas = append(pedidas, tabular.ColumnaExport{Clave: c.Clave, Encabezado: c.Encabezado})
		}
		cols, err := datos.ColumnasExport(pedidas)
		if err != nil {
			return nil, "", invalida(err.Error())
		}
		cfg.Columnas = cfg.Columnas[:0]
		for _, c := range cols {
			cfg.Columnas = append(cfg.Columnas, columnaJSON{Clave: c.Clave, Encabezado: c.Encabezado})
		}
		if cfg.Formato == "" {
			cfg.Formato = tabular.SalidaXLSX
		}
		if _, err := tabular.NuevoExportador(cols, tabular.OpcionesExport{Formato: cfg.Formato, Separador: cfg.Separador, Codificacion: cfg.Codificacion}); err != nil {
			return nil, "", invalida(err.Error())
		}
		b, err := json.Marshal(cfg)
		return b, cfg.Formato, err
	}
	return nil, "", invalida("tipo desconocido")
}

func (l *ListPlantillasDatosLogic) ListPlantillasDatos(in *estudiantes.ListPlantillasDatosRequest) (*estudiantes.ListPlantillasDatosResponse, error) {
	tipo := ""
	if in != nil {
		tipo = strings.ToUpper(strings.TrimSpace(in.Tipo))
	}
	if tipo == "" {
		tipo = tipoExportacion
	}
	if _, err := autorizarPlantilla(l.ctx, l.svcCtx, tipo); err != nil {
		return nil, err
	}
	ps, err := l.svcCtx.EstudianteRepository.ListPlantillas(l.ctx, datos.Modulo, tipo)
	if err != nil {
		return nil, errorDatos(l.Logger, "listar plantillas", err)
	}
	resp := &estudiantes.ListPlantillasDatosResponse{}
	for _, p := range ps {
		resp.Plantillas = append(resp.Plantillas, plantillaProto(p))
	}
	return resp, nil
}

func (l *GuardarPlantillaDatosLogic) GuardarPlantillaDatos(in *estudiantes.GuardarPlantillaDatosRequest) (*estudiantes.PlantillaDatosResponse, error) {
	if in == nil {
		in = &estudiantes.GuardarPlantillaDatosRequest{}
	}
	tipo := strings.ToUpper(strings.TrimSpace(in.Tipo))
	userID, err := autorizarPlantilla(l.ctx, l.svcCtx, tipo)
	if err != nil {
		return nil, err
	}
	nombre := strings.Join(strings.Fields(in.Nombre), " ")
	if nombre == "" || utf8.RuneCountInString(nombre) > 80 {
		return nil, status.Error(codes.InvalidArgument, "validation_error: el nombre de la plantilla es obligatorio (hasta 80 caracteres)")
	}
	cfg, formato, err := normalizarConfiguracion(tipo, in.Formato, in.Configuracion)
	if err != nil {
		return nil, err
	}
	p := repository.Plantilla{Modulo: datos.Modulo, Tipo: tipo, Nombre: nombre, Formato: formato, Configuracion: bytes.TrimSpace(cfg),
		CreadoPor: userID, ActualizadoPor: userID}
	var guardada *repository.Plantilla
	if id := strings.TrimSpace(in.Id); id != "" {
		if p.ID, err = validarID(id); err != nil {
			return nil, err
		}
		if in.ExpectedVersion < 1 {
			return nil, status.Error(codes.InvalidArgument, "validation_error: expected_version es obligatorio")
		}
		guardada, err = l.svcCtx.EstudianteRepository.ActualizarPlantilla(l.ctx, p, int(in.ExpectedVersion))
	} else {
		guardada, err = l.svcCtx.EstudianteRepository.CrearPlantilla(l.ctx, p)
	}
	if err != nil {
		return nil, errorDatos(l.Logger, "guardar plantilla", err)
	}
	return &estudiantes.PlantillaDatosResponse{Plantilla: plantillaProto(guardada)}, nil
}

func (l *EliminarPlantillaDatosLogic) EliminarPlantillaDatos(in *estudiantes.EliminarPlantillaDatosRequest) (*estudiantes.EliminarPlantillaDatosResponse, error) {
	if in == nil {
		in = &estudiantes.EliminarPlantillaDatosRequest{}
	}
	tipo := strings.ToUpper(strings.TrimSpace(in.Tipo))
	if _, err := autorizarPlantilla(l.ctx, l.svcCtx, tipo); err != nil {
		return nil, err
	}
	id, err := validarID(in.Id)
	if err != nil {
		return nil, err
	}
	if err := l.svcCtx.EstudianteRepository.EliminarPlantilla(l.ctx, id, datos.Modulo, tipo); err != nil {
		return nil, errorDatos(l.Logger, "eliminar plantilla", err)
	}
	return &estudiantes.EliminarPlantillaDatosResponse{Eliminada: true}, nil
}

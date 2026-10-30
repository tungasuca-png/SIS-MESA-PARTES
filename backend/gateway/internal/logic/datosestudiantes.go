package logic

import (
	"context"
	"encoding/base64"
	"time"

	"gateway/internal/svc"
	"gateway/internal/types"

	"estudiantes/estudiantesclient"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

// Motor universal de datos de Estudiantes: el Gateway reenvía con el JWT del
// usuario y traduce. Detección de formato, validación, permisos y escritura
// ocurren en Estudiantes Service.

const (
	tiempoDatosAnalizar  = 60 * time.Second
	tiempoDatosConfirmar = 180 * time.Second
	tiempoDatosExportar  = 60 * time.Second
	// maxRespuestaExport: una exportación grande (JSON/XML de miles de
	// registros) puede superar los 4 MB por defecto de gRPC.
	maxRespuestaExport = 32 << 20
)

// SolicitudDatos: lo que llega por multipart (archivo + campo "solicitud").
type SolicitudDatos struct {
	NombreArchivo string `json:"-"`
	Contenido     []byte `json:"-"`
	Opciones      struct {
		Hoja           string `json:"hoja"`
		Ruta           string `json:"ruta"`
		Separador      string `json:"separador"`
		Codificacion   string `json:"codificacion"`
		FilaEncabezado int32  `json:"fila_encabezado"`
	} `json:"opciones"`
	Mapeo []struct {
		Columna int32  `json:"columna"`
		Campo   string `json:"campo"`
	} `json:"mapeo"`
	MapeoConfirmado  bool    `json:"mapeo_confirmado"`
	PlantillaId      string  `json:"plantilla_id"`
	Huella           string  `json:"huella"`
	ExcluirInvalidos bool    `json:"excluir_invalidos"`
	Aceptadas        []int32 `json:"aceptadas"`
	Omitidas         []int32 `json:"omitidas"`
}

func (s SolicitudDatos) Proto() *estudiantesclient.DatosImportacionRequest {
	out := &estudiantesclient.DatosImportacionRequest{
		NombreArchivo: s.NombreArchivo, Contenido: s.Contenido,
		Opciones: &estudiantesclient.OpcionesLectura{Hoja: s.Opciones.Hoja, Ruta: s.Opciones.Ruta, Separador: s.Opciones.Separador,
			Codificacion: s.Opciones.Codificacion, FilaEncabezado: s.Opciones.FilaEncabezado},
		MapeoConfirmado: s.MapeoConfirmado, PlantillaId: s.PlantillaId, Huella: s.Huella,
		ExcluirInvalidos: s.ExcluirInvalidos, Aceptadas: s.Aceptadas, Omitidas: s.Omitidas,
	}
	if s.Opciones == (SolicitudDatos{}).Opciones {
		out.Opciones = nil // sin opciones: el servicio detecta todo (o usa las de la plantilla)
	}
	for _, m := range s.Mapeo {
		out.Mapeo = append(out.Mapeo, &estudiantesclient.AsignacionColumna{Columna: m.Columna, Campo: m.Campo})
	}
	return out
}

func noNulo(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}

func toDatosImportacionDTO(in *estudiantesclient.DatosImportacionResponse) *types.DatosImportacionResponse {
	out := &types.DatosImportacionResponse{
		Formato: in.Formato, Extension: in.Extension, Hojas: noNulo(in.Hojas), Hoja: in.Hoja, Rutas: noNulo(in.Rutas), Ruta: in.Ruta,
		Separador: in.Separador, Codificacion: in.Codificacion, FilaEncabezado: in.FilaEncabezado, AvisosArchivo: noNulo(in.AvisosArchivo),
		ProblemasMapeo: noNulo(in.ProblemasMapeo), RequiereConfirmarMapeo: in.RequiereConfirmarMapeo, Bloqueantes: noNulo(in.Bloqueantes),
		Avisos: noNulo(in.Avisos), Huella: in.Huella, Confirmada: in.Confirmada, PlantillaAplicada: in.PlantillaAplicada,
		PlantillaAvisos: noNulo(in.PlantillaAvisos), Columnas: []types.ColumnaAnalizadaDTO{}, Registros: []types.RegistroAnalizadoDTO{},
	}
	if r := in.Resumen; r != nil {
		out.Resumen = types.ResumenDatosDTO{Total: r.Total, Nuevos: r.Nuevos, Existentes: r.Existentes, Duplicados: r.Duplicados,
			PorConfirmar: r.PorConfirmar, Conflictos: r.Conflictos, Invalidos: r.Invalidos, ConAvisos: r.ConAvisos,
			Insertados: r.Insertados, Omitidos: r.Omitidos}
	}
	for _, c := range in.Columnas {
		out.Columnas = append(out.Columnas, types.ColumnaAnalizadaDTO{Columna: c.Columna, Encabezado: c.Encabezado, Campo: c.Campo,
			Confianza: c.Confianza, Motivo: c.Motivo, Alternativas: noNulo(c.Alternativas), Muestras: noNulo(c.Muestras),
			Confirmada: c.Confirmada, Importable: c.Importable})
	}
	for _, r := range in.Registros {
		out.Registros = append(out.Registros, types.RegistroAnalizadoDTO{Fila: r.Fila, Categoria: r.Categoria, Codigo: r.Codigo,
			Documento: r.Documento, Apellidos: r.Apellidos, Nombres: r.Nombres, Estado: r.Estado, Nivel: r.Nivel, Grado: r.Grado,
			Seccion: r.Seccion, Turno: r.Turno, Errores: noNulo(r.Errores), Avisos: noNulo(r.Avisos), Correcciones: noNulo(r.Correcciones),
			Coincidencia: r.Coincidencia, Resultado: r.Resultado})
	}
	return out
}

func toPlantillaDTO(p *estudiantesclient.PlantillaDatos) types.PlantillaDatosDTO {
	if p == nil {
		return types.PlantillaDatosDTO{}
	}
	return types.PlantillaDatosDTO{Id: p.Id, Tipo: p.Tipo, Nombre: p.Nombre, Formato: p.Formato, Configuracion: p.Configuracion,
		Version: p.Version, CreadoPor: p.CreadoPor, ActualizadoPor: p.ActualizadoPor, Creado: p.Creado, Actualizado: p.Actualizado}
}

type datosLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func nuevoDatosLogic(ctx context.Context, svcCtx *svc.ServiceContext) datosLogic {
	return datosLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type AnalizarDatosEstudiantesLogic struct{ datosLogic }

func NewAnalizarDatosEstudiantesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalizarDatosEstudiantesLogic {
	return &AnalizarDatosEstudiantesLogic{nuevoDatosLogic(ctx, svcCtx)}
}

func (l *AnalizarDatosEstudiantesLogic) AnalizarDatosEstudiantes(req *types.DatosArchivoRequest, s SolicitudDatos) (*types.DatosImportacionResponse, error) {
	resp, err := l.svcCtx.EstudiantesClient.AnalizarDatosEstudiantes(withAuthorization(l.ctx, req.Authorization), s.Proto(),
		zrpc.WithCallTimeout(tiempoDatosAnalizar))
	if err != nil {
		return nil, err
	}
	return toDatosImportacionDTO(resp), nil
}

type ConfirmarDatosEstudiantesLogic struct{ datosLogic }

func NewConfirmarDatosEstudiantesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfirmarDatosEstudiantesLogic {
	return &ConfirmarDatosEstudiantesLogic{nuevoDatosLogic(ctx, svcCtx)}
}

func (l *ConfirmarDatosEstudiantesLogic) ConfirmarDatosEstudiantes(req *types.DatosArchivoRequest, s SolicitudDatos) (*types.DatosImportacionResponse, error) {
	resp, err := l.svcCtx.EstudiantesClient.ConfirmarDatosEstudiantes(withAuthorization(l.ctx, req.Authorization), s.Proto(),
		zrpc.WithCallTimeout(tiempoDatosConfirmar))
	if err != nil {
		return nil, err
	}
	return toDatosImportacionDTO(resp), nil
}

type CamposDatosEstudiantesLogic struct{ datosLogic }

func NewCamposDatosEstudiantesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CamposDatosEstudiantesLogic {
	return &CamposDatosEstudiantesLogic{nuevoDatosLogic(ctx, svcCtx)}
}

func (l *CamposDatosEstudiantesLogic) CamposDatosEstudiantes(req *types.DatosCamposRequest) (*types.DatosCamposResponse, error) {
	resp, err := l.svcCtx.EstudiantesClient.CamposDatosEstudiantes(withAuthorization(l.ctx, req.Authorization), &estudiantesclient.CamposDatosRequest{})
	if err != nil {
		return nil, err
	}
	out := &types.DatosCamposResponse{Campos: []types.CampoDatosDTO{}, FormatosEntrada: noNulo(resp.FormatosEntrada),
		FormatosSalida: noNulo(resp.FormatosSalida), ColumnasTabla: []types.ColumnaExportDatosDTO{}}
	for _, c := range resp.Campos {
		out.Campos = append(out.Campos, types.CampoDatosDTO{Clave: c.Clave, Etiqueta: c.Etiqueta, Grupo: c.Grupo, Importable: c.Importable, Exportable: c.Exportable})
	}
	for _, c := range resp.ColumnasTabla {
		out.ColumnasTabla = append(out.ColumnasTabla, types.ColumnaExportDatosDTO{Clave: c.Clave, Encabezado: c.Encabezado})
	}
	return out, nil
}

type ExportarDatosEstudiantesLogic struct{ datosLogic }

func NewExportarDatosEstudiantesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExportarDatosEstudiantesLogic {
	return &ExportarDatosEstudiantesLogic{nuevoDatosLogic(ctx, svcCtx)}
}

func (l *ExportarDatosEstudiantesLogic) ExportarDatosEstudiantes(req *types.DatosExportarRequest) (*types.ArchivoDatosResponse, error) {
	in := &estudiantesclient.DatosExportacionRequest{Q: req.Q, Estado: req.Estado, Nivel: req.Nivel, Grado: req.Grado, Seccion: req.Seccion,
		Turno: req.Turno, Orden: req.Orden, Formato: req.Formato, Separador: req.Separador, Codificacion: req.Codificacion,
		ProtegerFormulas: req.ProtegerFormulas}
	for _, c := range req.Columnas {
		in.Columnas = append(in.Columnas, &estudiantesclient.ColumnaExportDatos{Clave: c.Clave, Encabezado: c.Encabezado})
	}
	resp, err := l.svcCtx.EstudiantesClient.ExportarDatosEstudiantes(withAuthorization(l.ctx, req.Authorization), in,
		zrpc.WithCallTimeout(tiempoDatosExportar), grpc.MaxCallRecvMsgSize(maxRespuestaExport))
	if err != nil {
		return nil, err
	}
	return &types.ArchivoDatosResponse{Nombre: resp.Nombre, Mime: resp.Mime, ContenidoBase64: base64.StdEncoding.EncodeToString(resp.Contenido),
		Filas: resp.Filas, Protegidas: resp.Protegidas}, nil
}

type ListPlantillasDatosLogic struct{ datosLogic }

func NewListPlantillasDatosLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlantillasDatosLogic {
	return &ListPlantillasDatosLogic{nuevoDatosLogic(ctx, svcCtx)}
}

func (l *ListPlantillasDatosLogic) ListPlantillasDatos(req *types.ListPlantillasDatosRequest) (*types.ListPlantillasDatosResponse, error) {
	resp, err := l.svcCtx.EstudiantesClient.ListPlantillasDatos(withAuthorization(l.ctx, req.Authorization), &estudiantesclient.ListPlantillasDatosRequest{Tipo: req.Tipo})
	if err != nil {
		return nil, err
	}
	out := &types.ListPlantillasDatosResponse{Plantillas: []types.PlantillaDatosDTO{}}
	for _, p := range resp.Plantillas {
		out.Plantillas = append(out.Plantillas, toPlantillaDTO(p))
	}
	return out, nil
}

type GuardarPlantillaDatosLogic struct{ datosLogic }

func NewGuardarPlantillaDatosLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GuardarPlantillaDatosLogic {
	return &GuardarPlantillaDatosLogic{nuevoDatosLogic(ctx, svcCtx)}
}

func (l *GuardarPlantillaDatosLogic) GuardarPlantillaDatos(req *types.GuardarPlantillaDatosRequest) (*types.PlantillaDatosResponse, error) {
	resp, err := l.svcCtx.EstudiantesClient.GuardarPlantillaDatos(withAuthorization(l.ctx, req.Authorization), &estudiantesclient.GuardarPlantillaDatosRequest{
		Id: req.Id, Tipo: req.Tipo, Nombre: req.Nombre, Formato: req.Formato, Configuracion: req.Configuracion, ExpectedVersion: req.ExpectedVersion,
	})
	if err != nil {
		return nil, err
	}
	return &types.PlantillaDatosResponse{Plantilla: toPlantillaDTO(resp.Plantilla)}, nil
}

type EliminarPlantillaDatosLogic struct{ datosLogic }

func NewEliminarPlantillaDatosLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EliminarPlantillaDatosLogic {
	return &EliminarPlantillaDatosLogic{nuevoDatosLogic(ctx, svcCtx)}
}

func (l *EliminarPlantillaDatosLogic) EliminarPlantillaDatos(req *types.EliminarPlantillaDatosRequest) (*types.EliminarPlantillaDatosResponse, error) {
	resp, err := l.svcCtx.EstudiantesClient.EliminarPlantillaDatos(withAuthorization(l.ctx, req.Authorization),
		&estudiantesclient.EliminarPlantillaDatosRequest{Id: req.PlantillaId, Tipo: req.Tipo})
	if err != nil {
		return nil, err
	}
	return &types.EliminarPlantillaDatosResponse{Eliminada: resp.Eliminada}, nil
}

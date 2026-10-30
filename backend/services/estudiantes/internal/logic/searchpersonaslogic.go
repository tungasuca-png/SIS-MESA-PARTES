package logic

import (
	"context"
	"unicode/utf8"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchPersonasLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchPersonasLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchPersonasLogic {
	return &SearchPersonasLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SearchPersonas (etapa 5): busca personas registradas por documento, nombres
// o apellidos (mismos límites que SearchEstudiantes) para reutilizarlas al
// vincular — "estudiantes.familia.view". Devuelve solo la identidad, sin
// vínculos.
func (l *SearchPersonasLogic) SearchPersonas(in *estudiantes.SearchPersonasRequest) (*estudiantes.SearchPersonasResponse, error) {
	if _, err := autorizar(l.ctx, l.svcCtx, permisoFamiliaView); err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.SearchPersonasRequest{}
	}
	q := validation.CleanText(in.Q)
	if !validation.QueryLenOK(q) || utf8.RuneCountInString(q) > validation.MaxQueryLength {
		return nil, errorSeguro(l.Logger, "buscar personas", "", validation.Invalid("q debe tener entre 2 y 100 caracteres"))
	}
	terms := validation.QueryTerms(q)
	if len(terms) > validation.MaxQueryTerms {
		return nil, errorSeguro(l.Logger, "buscar personas", "", validation.Invalid("demasiadas palabras en la búsqueda"))
	}
	page, size := paginar(in.Page, in.PageSize)
	personas, total, err := l.svcCtx.EstudianteRepository.SearchPersonas(l.ctx, repository.PersonaSearch{Terms: terms, Page: page, PageSize: size})
	if err != nil {
		return nil, errorSeguro(l.Logger, "buscar personas", "no se pudo buscar personas", err)
	}
	out := make([]*estudiantes.Persona, 0, len(personas))
	for i := range personas {
		out = append(out, personaToProto(&personas[i]))
	}
	return &estudiantes.SearchPersonasResponse{Personas: out, Total: int32(total)}, nil
}

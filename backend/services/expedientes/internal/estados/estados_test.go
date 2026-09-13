package estados

import "testing"

func TestIsValidTipo(t *testing.T) {
	cases := map[string]bool{
		"SOLICITUD": true, "TRAMITE": true, "OFICIO": true, "OTRO": true,
		"CERTIFICADO": true, "CONSTANCIA": true, "PERMISO": true,
		"JUSTIFICACION_FALTA": true, "JUSTIFICACION_TARDANZA": true,
		"INVALIDO": false, "ABC": false, "": false,
	}
	for tipo, want := range cases {
		if got := IsValidTipo(tipo); got != want {
			t.Errorf("IsValidTipo(%q) = %v, want %v", tipo, got, want)
		}
	}
}

func TestIsValidPrioridad(t *testing.T) {
	cases := map[string]bool{"NORMAL": true, "URGENTE": true, "ALTA": false, "": false}
	for p, want := range cases {
		if got := IsValidPrioridad(p); got != want {
			t.Errorf("IsValidPrioridad(%q) = %v, want %v", p, got, want)
		}
	}
}

func TestCanTransition(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{
		{EstadoPendiente, EstadoEnProceso, true},
		{EstadoPendiente, EstadoObservado, true},
		{EstadoEnProceso, EstadoAtendido, true},
		{EstadoAtendido, EstadoPendiente, false},
		{EstadoPendiente, EstadoAtendido, false},
		{EstadoObservado, EstadoEnProceso, false},
		{EstadoAtendido, EstadoObservado, false},
	}
	for _, c := range cases {
		if got := CanTransition(c.from, c.to); got != c.want {
			t.Errorf("CanTransition(%q, %q) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

func TestCanDerivar(t *testing.T) {
	cases := []struct {
		tipo, areaActual, areaDestino string
		want                          bool
	}{
		// F2: SECRETARIA->DIRECTOR (Etapa 3) y DIRECTOR->SECRETARIA (Etapa 4, cierre).
		{TipoCertificado, AreaSecretaria, AreaDirector, true},
		{TipoConstancia, AreaSecretaria, AreaDirector, true},
		{TipoCertificado, AreaDirector, AreaSecretaria, true},
		{TipoConstancia, AreaDirector, AreaSecretaria, true},
		// F2: nada más está confirmado (Dirección no deriva a Subdirección ni a Docente).
		{TipoCertificado, AreaDirector, AreaSubdirector, false},
		{TipoCertificado, AreaSecretaria, AreaDocente, false},
		{TipoConstancia, AreaSecretaria, AreaSubdirector, false},
		// F4: tres transiciones confirmadas en cadena (Etapa 3 + Etapa 4).
		{TipoPermiso, AreaSecretaria, AreaDirector, true},
		{TipoPermiso, AreaDirector, AreaSubdirector, true},
		{TipoPermiso, AreaSubdirector, AreaDocente, true},
		{TipoJustificacionFalta, AreaSecretaria, AreaDirector, true},
		{TipoJustificacionFalta, AreaDirector, AreaSubdirector, true},
		{TipoJustificacionFalta, AreaSubdirector, AreaDocente, true},
		{TipoJustificacionTardanza, AreaSecretaria, AreaDirector, true},
		{TipoJustificacionTardanza, AreaDirector, AreaSubdirector, true},
		{TipoJustificacionTardanza, AreaSubdirector, AreaDocente, true},
		// F4: lo que no está confirmado se rechaza (ej. saltar directo a Subdirección,
		// o que Subdirección devuelva a Dirección en vez de mandar al Docente).
		{TipoPermiso, AreaSecretaria, AreaSubdirector, false},
		{TipoPermiso, AreaSubdirector, AreaDirector, false},
		// Genéricos: sin regla institucional, se mantienen permisivos (comportamiento previo a Etapa 3).
		{TipoSolicitud, AreaSecretaria, AreaDocente, true},
		{TipoOtro, AreaAuxiliar, AreaAdmin, true},
		// Área destino inválida siempre se rechaza, sin importar el tipo.
		{TipoCertificado, AreaSecretaria, "INVALIDA", false},
		{TipoSolicitud, AreaSecretaria, "INVALIDA", false},
	}
	for _, c := range cases {
		if got := CanDerivar(c.tipo, c.areaActual, c.areaDestino); got != c.want {
			t.Errorf("CanDerivar(%q, %q, %q) = %v, want %v", c.tipo, c.areaActual, c.areaDestino, got, c.want)
		}
	}
}

func TestEsF2(t *testing.T) {
	for _, tipo := range []string{TipoCertificado, TipoConstancia} {
		if !EsF2(tipo) {
			t.Errorf("EsF2(%q) = false, want true", tipo)
		}
		if EsF4(tipo) {
			t.Errorf("EsF4(%q) = true, want false", tipo)
		}
	}
}

func TestEsF4(t *testing.T) {
	for _, tipo := range []string{TipoPermiso, TipoJustificacionFalta, TipoJustificacionTardanza} {
		if !EsF4(tipo) {
			t.Errorf("EsF4(%q) = false, want true", tipo)
		}
		if EsF2(tipo) {
			t.Errorf("EsF2(%q) = true, want false", tipo)
		}
	}
	for _, tipo := range []string{TipoSolicitud, TipoTramite, TipoOficio, TipoOtro} {
		if EsF2(tipo) || EsF4(tipo) {
			t.Errorf("tipo genérico %q no debería ser ni F2 ni F4", tipo)
		}
	}
}

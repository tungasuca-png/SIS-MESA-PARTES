package estados

import "testing"

func TestIsValidTipo(t *testing.T) {
	cases := map[string]bool{
		"SOLICITUD": true, "TRAMITE": true, "OFICIO": true, "OTRO": true,
		"INVALIDO": false, "": false,
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

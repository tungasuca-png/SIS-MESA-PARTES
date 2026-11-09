package reconciliacion

import (
	"errors"
	"net/smtp"
	"strings"
	"testing"
	"time"
)

func TestCamposDistintos(t *testing.T) {
	ts := time.Date(2026, 11, 8, 10, 0, 0, 123456000, time.UTC)
	m := movimiento{ID: "a", Exp: "e", Seq: 1, Tipo: "DERIVACION", Actor: "u", Rol: "SECRETARIA", AO: "SECRETARIA", AD: "DIRECTOR",
		EA: "PENDIENTE", EN: "EN_PROCESO", Motivo: "m", Condicion: "", Ocurrido: ts}
	p := m
	p.Ocurrido = ts.In(time.FixedZone("PE", -5*3600)) // mismo instante, otra zona
	if c := camposDistintos(m, p); len(c) != 0 {
		t.Fatalf("idénticos: %v", c)
	}
	p.AD, p.Motivo, p.Ocurrido = "SUBDIRECTOR", "otro", ts.Add(time.Microsecond)
	c := camposDistintos(m, p)
	if strings.Join(c, ",") != "area_destino,motivo,ocurrido_en" {
		t.Fatalf("campos: %v", c)
	}
}

func TestOrdenarDeterminista(t *testing.T) {
	ds := []Discrepancia{
		{Tipo: "B", ExpedienteID: "2", Secuencia: ip(1)},
		{Tipo: "Z", ExpedienteID: "1"},
		{Tipo: "A", ExpedienteID: "1", Secuencia: ip(2)},
		{Tipo: "A", ExpedienteID: "2", Secuencia: ip(1)},
	}
	ordenar(ds)
	var got []string
	for _, d := range ds {
		got = append(got, d.ExpedienteID+d.Tipo)
	}
	if strings.Join(got, " ") != "1Z 1A 2A 2B" {
		t.Fatalf("orden: %v", got)
	}
}

func TestTruncarRunas(t *testing.T) {
	if got := truncar("áéíóú", 3); got != "áéí" {
		t.Fatalf("truncar: %q", got)
	}
}

func TestAlertaSMTP(t *testing.T) {
	orig := EnviarCorreo
	defer func() { EnviarCorreo = orig }()
	var servidor, de string
	var para []string
	var msg []byte
	var conAuth bool
	EnviarCorreo = func(addr string, a smtp.Auth, from string, to []string, m []byte) error {
		servidor, de, para, msg, conAuth = addr, from, to, m, a != nil
		return nil
	}
	inf := &Informe{Ejecucion: "x", Modo: ModoDeteccion, Estado: "CON_DISCREPANCIAS",
		Discrepancias: []Discrepancia{{Tipo: "EVENTO_FALLIDO"}, {Tipo: "EVENTO_FALLIDO"}, {Tipo: "MOVIMIENTO_SIN_PROYECCION"}}}

	// Deshabilitada: no envía nada.
	if err := Alertar(ConfigSMTP{}, Asunto(inf, nil), Cuerpo(inf, nil)); err != nil || msg != nil {
		t.Fatalf("deshabilitada: %v %v", err, msg)
	}

	c := ConfigSMTP{Servidor: "smtp.ejemplo.test:587", Usuario: "u", Clave: "c", Remitente: "rec@ejemplo.test",
		Destinatarios: []string{"a@ejemplo.test", "b@ejemplo.test"}}
	if err := Alertar(c, Asunto(inf, nil), Cuerpo(inf, nil)); err != nil {
		t.Fatal(err)
	}
	s := string(msg)
	if servidor != c.Servidor || de != c.Remitente || len(para) != 2 || !conAuth {
		t.Fatalf("envío: %s %s %v %v", servidor, de, para, conAuth)
	}
	for _, want := range []string{"Subject: [SIS-MESA-PARTES] Reconciliación D8: 3 discrepancia(s)", "EVENTO_FALLIDO: 2", "MOVIMIENTO_SIN_PROYECCION: 1"} {
		if !strings.Contains(s, want) {
			t.Fatalf("falta %q en\n%s", want, s)
		}
	}
	if strings.Contains(s, c.Clave) && strings.Contains(s, "Clave") {
		t.Fatal("el mensaje no debe incluir credenciales")
	}

	// Inyección de cabeceras.
	c.Destinatarios = []string{"a@ejemplo.test\r\nBcc: x@malo.test"}
	if err := Alertar(c, "s", "b"); err == nil {
		t.Fatal("debía rechazar saltos de línea en cabeceras")
	}

	// Error de ejecución.
	if a := Asunto(nil, errors.New("x")); !strings.HasSuffix(a, "ERROR") {
		t.Fatalf("asunto: %s", a)
	}
}

func TestConfigSMTPDesdeEntorno(t *testing.T) {
	t.Setenv("RECONCILIADOR_SMTP_SERVIDOR", "h:25")
	t.Setenv("RECONCILIADOR_SMTP_REMITENTE", "r@x.test")
	t.Setenv("RECONCILIADOR_SMTP_DESTINATARIOS", " a@x.test , ,b@x.test")
	c := ConfigSMTPDesdeEntorno()
	if !c.Habilitada() || len(c.Destinatarios) != 2 || c.Destinatarios[1] != "b@x.test" {
		t.Fatalf("config: %+v", c)
	}
	t.Setenv("RECONCILIADOR_SMTP_DESTINATARIOS", "")
	if ConfigSMTPDesdeEntorno().Habilitada() {
		t.Fatal("sin destinatarios no debe habilitarse")
	}
}

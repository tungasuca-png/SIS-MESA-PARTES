package correo

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/quotedprintable"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"auth/internal/config"
)

// servidorSMTP: servidor mínimo en proceso (sin TLS) que guarda el DATA.
type servidorSMTP struct {
	ln       net.Listener
	starttls bool
	rechazar string // comando que responde 550
	recibido chan string
}

func nuevoServidor(t *testing.T, starttls bool, rechazar string) *servidorSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &servidorSMTP{ln: ln, starttls: starttls, rechazar: rechazar, recibido: make(chan string, 1)}
	t.Cleanup(func() { ln.Close() })
	go s.atender()
	return s
}

func (s *servidorSMTP) atender() {
	c, err := s.ln.Accept()
	if err != nil {
		return
	}
	defer c.Close()
	r := bufio.NewReader(c)
	escribir := func(l string) { fmt.Fprintf(c, "%s\r\n", l) }
	escribir("220 prueba ESMTP")
	for {
		linea, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(linea))
		verbo := strings.SplitN(cmd, " ", 2)[0]
		if s.rechazar != "" && strings.HasPrefix(cmd, s.rechazar) {
			escribir("550 rechazado")
			continue
		}
		switch verbo {
		case "EHLO", "HELO":
			if s.starttls {
				escribir("250-prueba")
				escribir("250 STARTTLS")
			} else {
				escribir("250 prueba")
			}
		case "MAIL", "RCPT", "RSET", "NOOP":
			escribir("250 ok")
		case "DATA":
			escribir("354 adelante")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				b.WriteString(l)
			}
			s.recibido <- b.String()
			escribir("250 recibido")
		case "QUIT":
			escribir("221 adiós")
			return
		default:
			escribir("502 no implementado")
		}
	}
}

func cfgLocal(s *servidorSMTP, seguridad string) config.SMTPConfig {
	return config.SMTPConfig{Servidor: s.ln.Addr().String(), Remitente: "no-responder@colegio.example",
		NombreRemitente: "SIS-MESA-PARTES - I.E. Tungasuca", Seguridad: seguridad}
}

func TestSMTP_EnviaMensajeCompleto(t *testing.T) {
	srv := nuevoServidor(t, false, "")
	e, err := NewSMTP(cfgLocal(srv, "ninguna"))
	if err != nil || !e.Configurado() {
		t.Fatal(err)
	}
	m := CodigoVerificacion("persona@correo.example", "048213", 10*time.Minute)
	if err := e.Enviar(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	crudo := <-srv.recibido
	cabeceras, cuerpo, _ := strings.Cut(crudo, "\r\n\r\n")
	for _, debe := range []string{"To: persona@correo.example", "Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: quoted-printable", "Subject: =?utf-8?q?", "Message-ID: <"} {
		if !strings.Contains(cabeceras, debe) {
			t.Errorf("falta la cabecera %q", debe)
		}
	}
	texto, _ := io.ReadAll(quotedprintable.NewReader(strings.NewReader(cuerpo)))
	for _, debe := range []string{"SIS-MESA-PARTES", "Colegio Tungasuca", "048213", "vence en 10 minutos", "No compartas", "Si no solicitaste"} {
		if !strings.Contains(string(texto), debe) {
			t.Errorf("el cuerpo no contiene %q", debe)
		}
	}
}

func TestSMTP_ExigeSTARTTLSYNoFiltraDirecciones(t *testing.T) {
	// starttls con un servidor que no lo ofrece: nunca se envía en claro.
	srv := nuevoServidor(t, false, "")
	e, _ := NewSMTP(cfgLocal(srv, "starttls"))
	err := e.Enviar(context.Background(), CodigoVerificacion("a@correo.example", "123456", time.Minute))
	var envio *ErrEnvio
	if !errors.As(err, &envio) || !strings.Contains(envio.Etapa, "STARTTLS") {
		t.Fatalf("sin STARTTLS debía fallar: %v", err)
	}
	select {
	case <-srv.recibido:
		t.Fatal("no debía enviarse nada")
	default:
	}
	// Destinatario rechazado: el error no contiene la dirección ni el código.
	srv2 := nuevoServidor(t, false, "RCPT")
	e2, _ := NewSMTP(cfgLocal(srv2, "ninguna"))
	err = e2.Enviar(context.Background(), CodigoVerificacion("secreta@correo.example", "777111", time.Minute))
	if !errors.As(err, &envio) || envio.Codigo != 550 || strings.Contains(err.Error(), "secreta") || strings.Contains(err.Error(), "777111") {
		t.Fatalf("error de destinatario: %v", err)
	}
	// Servidor caído.
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	dir := ln.Addr().String()
	ln.Close()
	e3, _ := NewSMTP(config.SMTPConfig{Servidor: dir, Remitente: "a@colegio.example", Seguridad: "ninguna"})
	if err := e3.Enviar(context.Background(), CodigoVerificacion("a@correo.example", "1", time.Minute)); !errors.As(err, &envio) {
		t.Fatalf("servidor caído: %v", err)
	}
}

func TestSMTP_RechazaInyeccionYConfiguracionInsegura(t *testing.T) {
	srv := nuevoServidor(t, false, "")
	e, _ := NewSMTP(cfgLocal(srv, "ninguna"))
	for _, para := range []string{"a@b.example\r\nBcc: otro@x.example", "Nombre <a@b.example>", "no-es-correo", ""} {
		if err := e.Enviar(context.Background(), Mensaje{Para: para, Asunto: "x", Texto: "y"}); !errors.Is(err, ErrDireccion) {
			t.Errorf("destinatario %q: %v", para, err)
		}
	}
	if err := e.Enviar(context.Background(), Mensaje{Para: "a@b.example", Asunto: "x\r\nBcc: o@x.example", Texto: "y"}); !errors.Is(err, ErrDireccion) {
		t.Errorf("asunto con salto de línea: %v", err)
	}

	sinConfig, err := NewSMTP(config.SMTPConfig{})
	if err != nil || sinConfig.Configurado() {
		t.Fatal("sin servidor: no configurado, sin error")
	}
	if err := sinConfig.Enviar(context.Background(), Mensaje{Para: "a@b.example"}); !errors.Is(err, ErrNoConfigurado) {
		t.Fatalf("enviar sin configuración: %v", err)
	}
	for nombre, c := range map[string]config.SMTPConfig{
		"sin claro hacia un host remoto": {Servidor: "smtp.proveedor.example:25", Remitente: "a@b.example", Seguridad: "ninguna"},
		"seguridad desconocida":          {Servidor: "smtp.proveedor.example:587", Remitente: "a@b.example", Seguridad: "ssl3"},
		"servidor sin puerto":            {Servidor: "smtp.proveedor.example", Remitente: "a@b.example"},
		"remitente inválido":             {Servidor: "smtp.proveedor.example:587", Remitente: "no-es-correo"},
		"nombre con salto":               {Servidor: "smtp.proveedor.example:587", Remitente: "a@b.example", NombreRemitente: "x\r\nBcc: y"},
	} {
		if _, err := NewSMTP(c); err == nil {
			t.Errorf("%s: debía rechazarse", nombre)
		}
	}
	if c, err := NewSMTP(config.SMTPConfig{Servidor: "smtp.proveedor.example:587", Remitente: "a@b.example"}); err != nil || c.cfg.Seguridad != "starttls" {
		t.Fatalf("STARTTLS es el valor por defecto: %v", err)
	}
}

func TestPlantillas_CuentaExistenteSinCodigo(t *testing.T) {
	m := CuentaExistente("a@b.example")
	if strings.ContainsAny(m.Texto, "0123456789") {
		t.Fatal("el aviso no debe contener dígitos (ni un código)")
	}
	if !strings.Contains(m.Texto, "ya existe una cuenta") || !strings.Contains(m.Texto, "SIS-MESA-PARTES") {
		t.Fatal("contenido del aviso")
	}
}

// Envío real a Mailpit (servidor SMTP de pruebas, no entrega a internet).
// Solo si MAILPIT_SMTP (host:puerto) y MAILPIT_API (http://host:puerto)
// están definidas.
func TestSMTP_MailpitReal(t *testing.T) {
	smtpDir, api := os.Getenv("MAILPIT_SMTP"), os.Getenv("MAILPIT_API")
	if smtpDir == "" || api == "" {
		t.Skip("MAILPIT_SMTP / MAILPIT_API no definidas")
	}
	e, err := NewSMTP(config.SMTPConfig{Servidor: smtpDir, Remitente: "no-responder@colegio.example",
		NombreRemitente: "SIS-MESA-PARTES - I.E. Tungasuca", Seguridad: "ninguna"})
	if err != nil {
		t.Fatal(err)
	}
	para := fmt.Sprintf("prueba%d@correo.example", time.Now().UnixNano())
	if err := e.Enviar(context.Background(), CodigoVerificacion(para, "531907", 10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	var lista struct {
		Messages []struct {
			ID      string `json:"ID"`
			Subject string `json:"Subject"`
			To      []struct {
				Address string `json:"Address"`
			} `json:"To"`
		} `json:"messages"`
	}
	resp, err := http.Get(api + "/api/v1/messages")
	if err != nil {
		t.Fatal(err)
	}
	_ = json.NewDecoder(resp.Body).Decode(&lista)
	resp.Body.Close()
	for _, m := range lista.Messages {
		if len(m.To) == 1 && m.To[0].Address == para {
			r, err := http.Get(api + "/api/v1/message/" + m.ID)
			if err != nil {
				t.Fatal(err)
			}
			var det struct {
				Text string `json:"Text"`
			}
			_ = json.NewDecoder(r.Body).Decode(&det)
			r.Body.Close()
			if m.Subject != "SIS-MESA-PARTES: tu código de verificación" || !strings.Contains(det.Text, "531907") ||
				!strings.Contains(det.Text, "Colegio Tungasuca") {
				t.Fatalf("mensaje recibido: %q", m.Subject)
			}
			return
		}
	}
	t.Fatal("Mailpit no recibió el mensaje")
}

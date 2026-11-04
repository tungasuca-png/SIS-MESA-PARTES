// Package correo envía los correos del registro manual (código de
// verificación y aviso de cuenta existente) mediante SMTP.
//
// Seguridad:
//   - Las credenciales vienen de la configuración (backend/.env, SMTP_*);
//     nunca del código.
//   - Exige TLS (STARTTLS o TLS implícito). Sin cifrado solo se admite un
//     servidor local de pruebas (localhost / 127.0.0.1 / ::1).
//   - Rechaza saltos de línea en direcciones y asunto (inyección de
//     cabeceras).
//   - Ni el cuerpo (que lleva el código) ni las direcciones llegan a los
//     logs: los errores solo indican la etapa y el código SMTP.
package correo

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"auth/internal/config"
)

// Mensaje de texto plano (UTF-8).
type Mensaje struct {
	Para   string
	Asunto string
	Texto  string
}

// Enviador: la lógica depende de esta interfaz (las pruebas usan un falso).
type Enviador interface {
	Configurado() bool
	Enviar(ctx context.Context, m Mensaje) error
}

var (
	// ErrNoConfigurado: falta SMTP_SERVIDOR o SMTP_REMITENTE.
	ErrNoConfigurado = errors.New("correo: proveedor SMTP no configurado")
	// ErrDireccion: destinatario o remitente inválidos.
	ErrDireccion = errors.New("correo: dirección inválida")
)

// ErrEnvio: el proveedor rechazó o no respondió. Etapa y código SMTP (si
// hay) sin direcciones ni contenido.
type ErrEnvio struct {
	Etapa  string
	Codigo int
}

func (e *ErrEnvio) Error() string {
	if e.Codigo != 0 {
		return fmt.Sprintf("correo: falló %s (SMTP %d)", e.Etapa, e.Codigo)
	}
	return "correo: falló " + e.Etapa
}

const tiempoMaximo = 20 * time.Second

// SMTP implementa Enviador.
type SMTP struct {
	cfg  config.SMTPConfig
	host string
}

// NewSMTP valida la configuración. Sin servidor o remitente devuelve un
// enviador "no configurado" (no un error): el registro manual queda
// deshabilitado y el resto del servicio arranca igual.
func NewSMTP(cfg config.SMTPConfig) (*SMTP, error) {
	cfg.Servidor = strings.TrimSpace(cfg.Servidor)
	cfg.Remitente = strings.TrimSpace(cfg.Remitente)
	cfg.Seguridad = strings.ToLower(strings.TrimSpace(cfg.Seguridad))
	if cfg.Seguridad == "" {
		cfg.Seguridad = "starttls"
	}
	s := &SMTP{cfg: cfg}
	if cfg.Servidor == "" || cfg.Remitente == "" {
		return s, nil
	}
	host, _, err := net.SplitHostPort(cfg.Servidor)
	if err != nil || host == "" {
		return nil, errors.New("SMTP_SERVIDOR debe tener la forma host:puerto")
	}
	if _, err := direccion(cfg.Remitente); err != nil {
		return nil, errors.New("SMTP_REMITENTE no es una dirección de correo válida")
	}
	if strings.ContainsAny(cfg.NombreRemitente, "\r\n") {
		return nil, errors.New("SMTP_NOMBRE_REMITENTE no puede tener saltos de línea")
	}
	switch cfg.Seguridad {
	case "starttls", "tls":
	case "ninguna":
		if !esLocal(host) {
			return nil, errors.New("SMTP_SEGURIDAD=ninguna solo se permite con un servidor local de pruebas")
		}
	default:
		return nil, errors.New(`SMTP_SEGURIDAD debe ser "starttls", "tls" o "ninguna"`)
	}
	s.host = host
	return s, nil
}

func (s *SMTP) Configurado() bool { return s != nil && s.host != "" }

func esLocal(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// direccion: una sola dirección "simple", sin nombre ni saltos de línea.
func direccion(v string) (string, error) {
	if strings.ContainsAny(v, "\r\n") {
		return "", ErrDireccion
	}
	a, err := mail.ParseAddress(v)
	if err != nil || a.Address != v {
		return "", ErrDireccion
	}
	return a.Address, nil
}

// Enviar entrega un mensaje. No reintenta: quien llama decide.
func (s *SMTP) Enviar(ctx context.Context, m Mensaje) error {
	if !s.Configurado() {
		return ErrNoConfigurado
	}
	para, err := direccion(m.Para)
	if err != nil {
		return err
	}
	if strings.ContainsAny(m.Asunto, "\r\n") {
		return ErrDireccion
	}
	cuerpo, err := s.componer(para, m)
	if err != nil {
		return &ErrEnvio{Etapa: "composición"}
	}

	ctx, cancel := context.WithTimeout(ctx, tiempoMaximo)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", s.cfg.Servidor)
	if err != nil {
		return &ErrEnvio{Etapa: "conexión"}
	}
	defer conn.Close()
	if limite, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(limite)
	}
	tlsCfg := &tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12}
	if s.cfg.Seguridad == "tls" {
		tc := tls.Client(conn, tlsCfg)
		if err := tc.HandshakeContext(ctx); err != nil {
			return &ErrEnvio{Etapa: "TLS"}
		}
		conn = tc
	}
	c, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return etapa("saludo", err)
	}
	defer c.Close()
	if s.cfg.Seguridad == "starttls" {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			// Nunca se envían credenciales ni códigos sin cifrar.
			return &ErrEnvio{Etapa: "STARTTLS (no ofrecido por el servidor)"}
		}
		if err := c.StartTLS(tlsCfg); err != nil {
			return etapa("STARTTLS", err)
		}
	}
	if s.cfg.Usuario != "" {
		if err := c.Auth(smtp.PlainAuth("", s.cfg.Usuario, s.cfg.Clave, s.host)); err != nil {
			return etapa("autenticación", err)
		}
	}
	if err := c.Mail(s.cfg.Remitente); err != nil {
		return etapa("remitente", err)
	}
	if err := c.Rcpt(para); err != nil {
		return etapa("destinatario", err)
	}
	w, err := c.Data()
	if err != nil {
		return etapa("datos", err)
	}
	if _, err := w.Write(cuerpo); err != nil {
		return etapa("datos", err)
	}
	if err := w.Close(); err != nil {
		return etapa("datos", err)
	}
	_ = c.Quit()
	return nil
}

func etapa(nombre string, err error) error {
	var tp *textproto.Error
	if errors.As(err, &tp) {
		return &ErrEnvio{Etapa: nombre, Codigo: tp.Code}
	}
	return &ErrEnvio{Etapa: nombre}
}

// componer arma cabeceras MIME y el cuerpo en quoted-printable.
func (s *SMTP) componer(para string, m Mensaje) ([]byte, error) {
	id := make([]byte, 12)
	if _, err := rand.Read(id); err != nil {
		return nil, err
	}
	dominio := s.cfg.Remitente[strings.LastIndex(s.cfg.Remitente, "@")+1:]
	de := s.cfg.Remitente
	if n := strings.TrimSpace(s.cfg.NombreRemitente); n != "" {
		de = (&mail.Address{Name: n, Address: s.cfg.Remitente}).String()
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\n", de)
	fmt.Fprintf(&b, "To: %s\r\n", para)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", m.Asunto))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: <%s@%s>\r\n", hex.EncodeToString(id), dominio)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	b.WriteString("Auto-Submitted: auto-generated\r\n\r\n")
	qp := quotedprintable.NewWriter(&b)
	texto := strings.ReplaceAll(strings.ReplaceAll(m.Texto, "\r\n", "\n"), "\n", "\r\n")
	if _, err := qp.Write([]byte(texto)); err != nil {
		return nil, err
	}
	if err := qp.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

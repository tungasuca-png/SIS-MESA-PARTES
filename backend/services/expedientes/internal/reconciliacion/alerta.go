package reconciliacion

import (
	"fmt"
	"net"
	"net/smtp"
	"os"
	"sort"
	"strings"
	"time"
)

// ConfigSMTP: alerta por correo opcional. Todo se lee del entorno
// (RECONCILIADOR_SMTP_*); nada de esto se guarda en el repositorio.
type ConfigSMTP struct {
	Servidor      string // host:puerto (STARTTLS si el servidor lo ofrece)
	Usuario       string
	Clave         string
	Remitente     string
	Destinatarios []string
}

func ConfigSMTPDesdeEntorno() ConfigSMTP {
	var dest []string
	for _, d := range strings.Split(os.Getenv("RECONCILIADOR_SMTP_DESTINATARIOS"), ",") {
		if d = strings.TrimSpace(d); d != "" {
			dest = append(dest, d)
		}
	}
	return ConfigSMTP{
		Servidor:      strings.TrimSpace(os.Getenv("RECONCILIADOR_SMTP_SERVIDOR")),
		Usuario:       os.Getenv("RECONCILIADOR_SMTP_USUARIO"),
		Clave:         os.Getenv("RECONCILIADOR_SMTP_CLAVE"),
		Remitente:     strings.TrimSpace(os.Getenv("RECONCILIADOR_SMTP_REMITENTE")),
		Destinatarios: dest,
	}
}

func (c ConfigSMTP) Habilitada() bool {
	return c.Servidor != "" && c.Remitente != "" && len(c.Destinatarios) > 0
}

// EnviarCorreo se sustituye en las pruebas.
var EnviarCorreo = smtp.SendMail

// Asunto y Cuerpo del aviso: solo conteos, tipos e identificadores; nunca
// contenido de expedientes ni credenciales.
func Asunto(inf *Informe, errEjec error) string {
	switch {
	case errEjec != nil:
		return "[SIS-MESA-PARTES] Reconciliación D8: ERROR"
	case len(inf.Discrepancias) > 0:
		return fmt.Sprintf("[SIS-MESA-PARTES] Reconciliación D8: %d discrepancia(s)", len(inf.Discrepancias))
	default:
		return "[SIS-MESA-PARTES] Reconciliación D8: sin discrepancias"
	}
}

func Cuerpo(inf *Informe, errEjec error) string {
	var b strings.Builder
	if inf != nil {
		fmt.Fprintf(&b, "Ejecución: %s\nModo: %s\nEstado: %s\nCorte: %s\n", inf.Ejecucion, inf.Modo, inf.Estado, inf.CorteHasta.Format(time.RFC3339))
		fmt.Fprintf(&b, "Expedientes revisados: %d\nMovimientos origen: %d\nMovimientos proyectados: %d\n",
			inf.ExpedientesRevisados, inf.MovimientosOrigen, inf.MovimientosProyectados)
		if inf.Huella != "" {
			fmt.Fprintf(&b, "Huella SHA-256 del informe: %s\n", inf.Huella)
		}
		if inf.Archivo != "" {
			fmt.Fprintf(&b, "Informe: %s\n", inf.Archivo)
		}
		conteo := map[string]int{}
		for _, d := range inf.Discrepancias {
			conteo[d.Tipo]++
		}
		tipos := make([]string, 0, len(conteo))
		for t := range conteo {
			tipos = append(tipos, t)
		}
		sort.Strings(tipos)
		for _, t := range tipos {
			fmt.Fprintf(&b, "  %s: %d\n", t, conteo[t])
		}
		for m, a := range inf.Acciones {
			fmt.Fprintf(&b, "Republicación %s: %s\n", m, a)
		}
	}
	if errEjec != nil {
		fmt.Fprintf(&b, "Error: %v\n", errEjec)
	}
	b.WriteString("\nProcedimiento: docs/operacion/D8_reconciliacion.md\n")
	return b.String()
}

// Alertar envía el aviso. Las líneas de cabecera se construyen solo con
// valores de configuración sin saltos de línea (evita inyección de cabeceras).
func Alertar(c ConfigSMTP, asunto, cuerpo string) error {
	if !c.Habilitada() {
		return nil
	}
	for _, v := range append([]string{c.Remitente, asunto}, c.Destinatarios...) {
		if strings.ContainsAny(v, "\r\n") {
			return fmt.Errorf("valor de cabecera inválido")
		}
	}
	var auth smtp.Auth
	if c.Usuario != "" {
		host, _, err := net.SplitHostPort(c.Servidor)
		if err != nil {
			return fmt.Errorf("RECONCILIADOR_SMTP_SERVIDOR debe ser host:puerto: %w", err)
		}
		auth = smtp.PlainAuth("", c.Usuario, c.Clave, host)
	}
	msg := "From: " + c.Remitente + "\r\n" +
		"To: " + strings.Join(c.Destinatarios, ", ") + "\r\n" +
		"Subject: " + asunto + "\r\n" +
		"MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" +
		strings.ReplaceAll(cuerpo, "\n", "\r\n")
	return EnviarCorreo(c.Servidor, auth, c.Remitente, c.Destinatarios, []byte(msg))
}

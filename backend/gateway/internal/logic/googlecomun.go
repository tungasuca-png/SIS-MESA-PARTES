package logic

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Acceso con Google en el Gateway: solo transporta redirecciones y cookies.
// Auth canjea el código, valida el ID token y decide; el Gateway nunca
// confía en lo que envíe el navegador más allá de reenviarlo a Auth.

const (
	// CookieFlujoGoogle: state, nonce, verificador PKCE y (si es una
	// vinculación) el código de vinculación. HttpOnly, SameSite=Lax (el
	// regreso desde Google es una navegación entre sitios), 10 minutos.
	CookieFlujoGoogle = "tg_google_flujo"
	// CookieVinculoGoogle: código de vinculación emitido tras reautenticar
	// con la clave. HttpOnly, SameSite=Strict: solo viaja en navegaciones
	// iniciadas desde el propio sistema (un enlace de terceros no la lleva).
	CookieVinculoGoogle = "tg_google_vinculo"
	RutaCookiesGoogle   = "/api/auth/google"
	DuracionFlujoGoogle = 600

	MensajeDNIRegistradoGoogle = "Este DNI ya tiene una cuenta registrada. Inicia sesión con tus credenciales institucionales. " +
		"Si deseas utilizar Google, podrás vincularlo desde tu perfil después de iniciar sesión."
)

// FlujoGoogle: contenido de la cookie del flujo.
type FlujoGoogle struct {
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	Vinculo  string `json:"l,omitempty"`
}

func CodificarFlujo(f FlujoGoogle) string {
	b, _ := json.Marshal(f)
	return base64.RawURLEncoding.EncodeToString(b)
}

func DecodificarFlujo(valor string) (FlujoGoogle, bool) {
	var f FlujoGoogle
	b, err := base64.RawURLEncoding.DecodeString(valor)
	if err != nil || len(b) > 2048 || json.Unmarshal(b, &f) != nil || f.State == "" || f.Nonce == "" || f.Verifier == "" {
		return FlujoGoogle{}, false
	}
	return f, true
}

var reClaveGoogle = regexp.MustCompile(`google_([a-z_]+):`)

// ClaveErrorGoogle: clave estable de un error de Auth ("google_<clave>: ...")
// para redirigir al frontend sin exponer el mensaje en la URL.
func ClaveErrorGoogle(err error) string {
	if m := reClaveGoogle.FindStringSubmatch(status.Convert(err).Message()); m != nil {
		return m[1]
	}
	switch status.Code(err) {
	case codes.ResourceExhausted:
		return "cuenta_bloqueada"
	case codes.Unavailable, codes.DeadlineExceeded:
		return "no_disponible"
	}
	return "error"
}

// destinoGoogle arma la URL del frontend. Los códigos de un solo uso van en
// el fragmento (#): el navegador no lo envía al servidor ni en el Referer.
func destinoGoogle(frontend, ruta string, fragmento, consulta url.Values) string {
	d := strings.TrimRight(strings.TrimSpace(frontend), "/") + ruta
	if len(consulta) > 0 {
		d += "?" + consulta.Encode()
	}
	if len(fragmento) > 0 {
		d += "#" + fragmento.Encode()
	}
	return d
}

// DestinoLoginGoogle: pantalla /login/google con el resultado o el error.
func DestinoLoginGoogle(frontend string, valores url.Values) string {
	return destinoGoogle(frontend, "/login/google", valores, nil)
}

// DestinoPerfilGoogle: perfil del portal tras una vinculación.
func DestinoPerfilGoogle(frontend string, valores url.Values) string {
	return destinoGoogle(frontend, "/mesa-de-partes/perfil", nil, valores)
}

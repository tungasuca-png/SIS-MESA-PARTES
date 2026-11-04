package correo

import (
	"fmt"
	"time"
)

const (
	sistema     = "SIS-MESA-PARTES"
	institucion = "I.E. Tungasuca (Colegio Tungasuca)"
)

// CodigoVerificacion: correo con el código de 6 dígitos.
func CodigoVerificacion(para, codigo string, vigencia time.Duration) Mensaje {
	minutos := int(vigencia.Round(time.Minute) / time.Minute)
	if minutos < 1 {
		minutos = 1
	}
	return Mensaje{
		Para:   para,
		Asunto: fmt.Sprintf("%s: tu código de verificación", sistema),
		Texto: fmt.Sprintf(`%s - %s
Mesa de Partes Virtual

Hola:

Para activar tu cuenta de solicitante, ingresa este código de verificación:

    %s

El código vence en %d minutos y solo puede usarse una vez.

No compartas este código con nadie. El personal de la institución nunca te lo pedirá.

Si no solicitaste crear una cuenta, ignora este mensaje: sin el código la cuenta no se activa y tus datos no se registran.

Este es un mensaje automático; no lo respondas.
`, sistema, institucion, codigo, minutos),
	}
}

// CuentaExistente: el correo ya tiene una cuenta. Se envía en lugar del
// código para no revelar en la pantalla si el correo está registrado.
func CuentaExistente(para string) Mensaje {
	return Mensaje{
		Para:   para,
		Asunto: fmt.Sprintf("%s: intento de registro con tu correo", sistema),
		Texto: fmt.Sprintf(`%s - %s
Mesa de Partes Virtual

Hola:

Alguien intentó crear una cuenta nueva con este correo, pero ya existe una cuenta asociada a él. No se creó ninguna cuenta nueva.

Si fuiste tú, inicia sesión con tu DNI y tu clave, o con «Continuar con Google» si te registraste así.

Si no fuiste tú, no necesitas hacer nada: tu cuenta no cambió.

Este es un mensaje automático; no lo respondas.
`, sistema, institucion),
	}
}

package config

import (
	"testing"

	"github.com/zeromicro/go-zero/core/conf"
)

// AllowLegacyAuth se lee como TEXTO desde el YAML real: sin comillas, "true"
// llegaría como booleano YAML y la configuración no cargaría (Psicología no
// iniciaría en desarrollo con el fallback explícito).
func TestAllowLegacyAuthSeLeeComoTexto(t *testing.T) {
	for _, valor := range []string{"", "false", "true"} {
		t.Run("valor="+valor, func(t *testing.T) {
			t.Setenv("PSICOLOGIA_ALLOW_LEGACY_AUTH", valor)
			var c Config
			if err := conf.Load("../../etc/psicologia.yaml", &c, conf.UseEnv()); err != nil {
				t.Fatalf("la configuración no cargó con PSICOLOGIA_ALLOW_LEGACY_AUTH=%q: %v", valor, err)
			}
			if c.AllowLegacyAuth != valor {
				t.Fatalf("se esperaba %q, se obtuvo %q", valor, c.AllowLegacyAuth)
			}
		})
	}
}

package outbox

import (
	"testing"
	"time"
)

func TestConfigValidar(t *testing.T) {
	base := Config{Lote: 10, Lease: 60 * time.Second, Intervalo: 2 * time.Second, MaxIntentos: 20, TimeoutRPC: 1800 * time.Millisecond}

	casos := []struct {
		nombre   string
		ajustar  func(*Config)
		error    bool
		conAviso bool
	}{
		{"configuración del repositorio", func(*Config) {}, false, false},
		{"lease en el máximo de la base", func(c *Config) { c.Lease = LeaseMaximo }, false, false},
		{"lease mayor que el máximo de la base", func(c *Config) { c.Lease = LeaseMaximo + time.Second }, true, false},
		{"lease igual a timeout + margen", func(c *Config) { c.Lease = 15 * time.Second; c.TimeoutRPC = 13 * time.Second }, true, false},
		{"lease menor que el timeout", func(c *Config) { c.Lease = 15 * time.Second; c.TimeoutRPC = 20 * time.Second }, true, false},
		{"lote que no cabe en el lease", func(c *Config) { c.Lote = 100 }, false, true},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			c := base
			tc.ajustar(&c)
			aviso, err := c.Validar()
			if (err != nil) != tc.error {
				t.Fatalf("error = %v, se esperaba error=%v", err, tc.error)
			}
			if (aviso != "") != tc.conAviso {
				t.Fatalf("aviso = %q, se esperaba aviso=%v", aviso, tc.conAviso)
			}
		})
	}
}

// Con una configuración válida, Ciclo llega a procesar el primer evento: la
// condición de corte de Ciclo no se cumple al inicio.
func TestConfigValidaPermiteProcesarElPrimerEvento(t *testing.T) {
	c := Config{Lote: 1, Lease: 15 * time.Second, TimeoutRPC: 12 * time.Second}
	if _, err := c.Validar(); err != nil {
		t.Fatalf("Validar: %v", err)
	}
	if 0 > c.Lease-c.TimeoutRPC-margenCiclo {
		t.Fatalf("Ciclo cortaría antes del primer evento")
	}
}

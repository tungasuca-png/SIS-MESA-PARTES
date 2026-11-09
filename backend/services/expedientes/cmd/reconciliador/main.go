// Reconciliador D8 (expedientes_db ↔ derivaciones_db).
//
// Uso diario automatizado (Programador de tareas, 02:00):
//
//	reconciliador -modo deteccion
//
// Republicación manual y controlada (solo tras revisar el informe):
//
//	reconciliador -modo republicacion -max 50
//
// Conexión (variables de entorno; la contraseña NUNCA va en el DSN ni en el repositorio):
//
//	RECONCILIADOR_EXPEDIENTES_DSN   p. ej. postgres://reconciliador_movimientos@127.0.0.1:5432/expedientes_db?sslmode=disable
//	RECONCILIADOR_DERIVACIONES_DSN  p. ej. postgres://reconciliador_movimientos@127.0.0.1:5432/derivaciones_db?sslmode=disable
//	RECONCILIADOR_DB_PASSWORD
//	RECONCILIADOR_SMTP_* (opcional, ver internal/reconciliacion/alerta.go)
//
// Códigos de salida: 0 sin discrepancias; 2 con discrepancias; 1 error.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"expedientes/internal/reconciliacion"
)

func main() {
	os.Exit(ejecutar())
}

func ejecutar() int {
	modo := flag.String("modo", "deteccion", "deteccion | republicacion")
	gracia := flag.Duration("gracia", 15*time.Minute, "ventana de gracia: movimientos más recientes no se evalúan como faltantes")
	max := flag.Int("max", 50, "máximo de movimientos a republicar (solo modo republicacion)")
	dir := flag.String("informes", "reconciliacion-informes", "directorio donde se guardan los informes JSON")
	limite := flag.Duration("limite", 10*time.Minute, "tiempo máximo de la ejecución")
	alertarSiempre := flag.Bool("alertar-siempre", false, "enviar correo aunque no haya discrepancias")
	flag.Parse()

	m := strings.ToUpper(strings.TrimSpace(*modo))
	if m != reconciliacion.ModoDeteccion && m != reconciliacion.ModoRepublicacion {
		log.Printf("reconciliador: modo inválido %q", *modo)
		return 1
	}
	if *gracia < time.Minute {
		log.Printf("reconciliador: la gracia mínima es 1m")
		return 1
	}
	if *max < 1 || *max > 1000 {
		log.Printf("reconciliador: -max debe estar entre 1 y 1000")
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), *limite)
	defer cancel()
	exp, err := abrir(ctx, "RECONCILIADOR_EXPEDIENTES_DSN")
	if err != nil {
		log.Printf("reconciliador: %v", err)
		return finalizar(nil, err, *alertarSiempre)
	}
	defer exp.Close()
	deriv, err := abrir(ctx, "RECONCILIADOR_DERIVACIONES_DSN")
	if err != nil {
		log.Printf("reconciliador: %v", err)
		return finalizar(nil, err, *alertarSiempre)
	}
	defer deriv.Close()

	inf, err := reconciliacion.Ejecutar(ctx, exp, deriv, reconciliacion.Opciones{
		Modo: m, Gracia: *gracia, MaxRepublicaciones: *max, DirectorioInformes: *dir,
	})
	return finalizar(inf, err, *alertarSiempre)
}

func abrir(ctx context.Context, variable string) (*pgxpool.Pool, error) {
	dsn := strings.TrimSpace(os.Getenv(variable))
	if dsn == "" {
		return nil, fmt.Errorf("falta %s", variable)
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("%s inválido", variable) // no se imprime el DSN
	}
	if cfg.ConnConfig.Password != "" {
		return nil, fmt.Errorf("%s no debe incluir la contraseña: use RECONCILIADOR_DB_PASSWORD", variable)
	}
	cfg.ConnConfig.Password = os.Getenv("RECONCILIADOR_DB_PASSWORD")
	cfg.ConnConfig.RuntimeParams["application_name"] = "reconciliador-d8"
	cfg.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("conectar (%s): %w", variable, err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("conectar (%s): %w", variable, err)
	}
	return pool, nil
}

func finalizar(inf *reconciliacion.Informe, err error, alertarSiempre bool) int {
	codigo := 0
	resumen := map[string]any{"evento": "reconciliacion_d8"}
	if inf != nil {
		resumen["ejecucion"] = inf.Ejecucion
		resumen["modo"] = inf.Modo
		resumen["estado"] = inf.Estado
		resumen["discrepancias"] = len(inf.Discrepancias)
		resumen["huella"] = inf.Huella
		resumen["informe"] = inf.Archivo
		resumen["republicaciones"] = inf.Acciones
		if len(inf.Discrepancias) > 0 {
			codigo = 2
		}
	}
	if err != nil {
		resumen["error"] = err.Error()
		codigo = 1
	}
	b, _ := json.Marshal(resumen)
	if codigo == 0 {
		log.Printf("%s", b)
	} else {
		log.Printf("ALERTA %s", b)
	}
	if codigo != 0 || alertarSiempre {
		if e := reconciliacion.Alertar(reconciliacion.ConfigSMTPDesdeEntorno(),
			reconciliacion.Asunto(inf, err), reconciliacion.Cuerpo(inf, err)); e != nil {
			log.Printf("ALERTA no se pudo enviar el correo: %v", e)
		}
	}
	return codigo
}

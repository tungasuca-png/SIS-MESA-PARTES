package svc

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// La contraseña con caracteres reservados de URL debe llegar intacta a pgx y
// no alterar host, puerto ni base.
func TestPostgresDSNEscapaCaracteresReservados(t *testing.T) {
	const clave = `a/b@c:d?e#f%g h+i`
	cfg, err := pgxpool.ParseConfig(postgresDSN("rol_app", clave, "127.0.0.1", 5433, "base_db", "disable"))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	cc := cfg.ConnConfig
	if cc.User != "rol_app" || cc.Password != clave || cc.Host != "127.0.0.1" || cc.Port != 5433 || cc.Database != "base_db" {
		t.Fatalf("DSN mal interpretado: user=%q host=%q port=%d db=%q clave_ok=%v", cc.User, cc.Host, cc.Port, cc.Database, cc.Password == clave)
	}
	if cc.TLSConfig != nil {
		t.Fatalf("sslmode=disable no se respetó")
	}
}

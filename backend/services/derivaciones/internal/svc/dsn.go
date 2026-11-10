package svc

import (
	"net"
	"net/url"
	"strconv"
)

// postgresDSN arma la URL de conexión escapando usuario, contraseña y base.
// Antes se usaba fmt.Sprintf("postgres://%s:%s@..."), que se rompe si la
// contraseña contiene @ : / ? # % o espacios (habitual en claves aleatorias):
// el servicio no arrancaba o intentaba conectar a otro host.
func postgresDSN(user, password, host string, port int, database, sslmode string) string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     net.JoinHostPort(host, strconv.Itoa(port)),
		Path:     "/" + database,
		RawQuery: url.Values{"sslmode": {sslmode}}.Encode(),
	}
	return u.String()
}

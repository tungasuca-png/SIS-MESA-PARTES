package config

import (
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf

	AuthRpc         zrpc.RpcClientConf
	ExpedientesRpc  zrpc.RpcClientConf
	UsuariosRpc     zrpc.RpcClientConf
	DocumentosRpc   zrpc.RpcClientConf
	DerivacionesRpc zrpc.RpcClientConf
	PsicologiaRpc   zrpc.RpcClientConf
	EstudiantesRpc  zrpc.RpcClientConf

	// ConfiarProxyLocal: solo cuando un proxy inverso en el MISMO equipo
	// (loopback) publica el Gateway; la IP del cliente para los límites de
	// tasa se toma entonces de X-Forwarded-For (última entrada).
	ConfiarProxyLocal bool `json:",optional"`

	// Acceso con Google (solo SOLICITANTES). GoogleCookieSegura = "true" marca
	// las cookies del flujo como Secure (obligatorio en producción con HTTPS).
	// FrontendURL: origen del frontend para las redirecciones; vacío = mismo
	// origen (el frontend llega al Gateway por el proxy de Vite o del servidor).
	GoogleCookieSegura string `json:",optional"`
	FrontendURL        string `json:",optional"`
}

package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"gateway/internal/svc"

	"documentos/documentosclient"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"google.golang.org/grpc"
)

// A-02: límites REALES de la subida de documentos. Se levanta el Gateway con
// las rutas de routes.go y la configuración por defecto de go-zero (la misma
// de producción: el YAML no fija MaxBytes ni Timeout) en un puerto local
// libre; Documentos es un doble que solo cuenta los bytes recibidos.

type documentosSubidaFalsa struct {
	documentosclient.Documentos
	recibidos int
	llamadas  int
}

func (d *documentosSubidaFalsa) UploadDocumento(_ context.Context, in *documentosclient.UploadDocumentoRequest, _ ...grpc.CallOption) (*documentosclient.UploadDocumentoResponse, error) {
	d.llamadas++
	d.recibidos = len(in.Contenido)
	return &documentosclient.UploadDocumentoResponse{Documento: &documentosclient.Documento{Id: "00000000-0000-4000-8000-0000000000d1"}}, nil
}

const (
	maxDocumentoServicio = 5 * 1024 * 1024 // validation.MaxTamanoDocumento (Documentos)
	maxCuerpoSubida      = 7340032         // maxBytes del grupo de UploadDocumento
	maxCuerpoPorDefecto  = 1048576         // RestConf.MaxBytes por defecto (go-zero)
)

func gatewayReal(t *testing.T, sc *svc.ServiceContext) string {
	t.Helper()
	logx.Disable()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	puerto := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()

	var c rest.RestConf
	if err := conf.FillDefault(&c); err != nil {
		t.Fatal(err)
	}
	c.Name, c.Host, c.Port = "test-gateway", "127.0.0.1", puerto
	if c.MaxBytes != maxCuerpoPorDefecto {
		t.Fatalf("MaxBytes por defecto de go-zero cambió: %d", c.MaxBytes)
	}
	server := rest.MustNewServer(c)
	RegisterHandlers(server, sc)
	go server.Start()
	t.Cleanup(server.Stop)

	base := fmt.Sprintf("http://127.0.0.1:%d", puerto)
	for i := 0; i < 100; i++ {
		if conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", puerto), 100*time.Millisecond); err == nil {
			_ = conn.Close()
			return base
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("el Gateway de prueba no arrancó")
	return ""
}

func cuerpoSubida(t *testing.T, bytesArchivo int) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]string{
		"expediente_id":  "00000000-0000-4000-8000-0000000000e1",
		"nombre":         "solicitud-escaneada.pdf",
		"tipo_documento": "ADJUNTO",
		"extension":      "pdf",
		"contenido":      base64.StdEncoding.EncodeToString(make([]byte, bytesArchivo)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func enviar(t *testing.T, url string, cuerpo []byte) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(cuerpo))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token-de-prueba")
	res, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer res.Body.Close()
	return res.StatusCode
}

func TestSubidaDocumento_AdmiteCincoMiBEnBase64(t *testing.T) {
	doc := &documentosSubidaFalsa{}
	base := gatewayReal(t, &svc.ServiceContext{DocumentosClient: doc})

	// Archivo de 5 MiB exactos (el máximo de Documentos): antes la ruta
	// heredaba 1 MiB y respondía 413 a partir de ~750 KB.
	cuerpo := cuerpoSubida(t, maxDocumentoServicio)
	if len(cuerpo) <= maxCuerpoPorDefecto || len(cuerpo) > maxCuerpoSubida {
		t.Fatalf("cuerpo de prueba fuera de rango: %d bytes", len(cuerpo))
	}
	if c := enviar(t, base+"/api/documentos", cuerpo); c != http.StatusOK {
		t.Fatalf("5 MiB en base64 (%d bytes): HTTP %d, se esperaba 200", len(cuerpo), c)
	}
	if doc.llamadas != 1 || doc.recibidos != maxDocumentoServicio {
		t.Fatalf("Documentos recibió %d bytes en %d llamadas", doc.recibidos, doc.llamadas)
	}

	// Un archivo intermedio (~2 MiB) también pasa.
	if c := enviar(t, base+"/api/documentos", cuerpoSubida(t, 2*1024*1024)); c != http.StatusOK {
		t.Fatalf("2 MiB: HTTP %d", c)
	}
}

func TestSubidaDocumento_CuerpoExcesivoRechazadoSinLlegarAlServicio(t *testing.T) {
	doc := &documentosSubidaFalsa{}
	base := gatewayReal(t, &svc.ServiceContext{DocumentosClient: doc})

	cuerpo := cuerpoSubida(t, 6*1024*1024) // 6 MiB -> ~8,4 MB en base64
	if len(cuerpo) <= maxCuerpoSubida {
		t.Fatalf("el cuerpo de prueba debía superar el límite: %d", len(cuerpo))
	}
	if c := enviar(t, base+"/api/documentos", cuerpo); c != http.StatusRequestEntityTooLarge {
		t.Fatalf("cuerpo de %d bytes: HTTP %d, se esperaba 413", len(cuerpo), c)
	}
	if doc.llamadas != 0 {
		t.Fatal("un cuerpo rechazado no debe llegar a Documentos")
	}
}

func TestSubidaDocumento_ElLimiteNoSeExtiendeAOtrasRutas(t *testing.T) {
	// Las demás rutas conservan el límite por defecto (1 MiB): un login de
	// 2 MB se rechaza antes del handler (AuthClient es nil y no se usa).
	base := gatewayReal(t, &svc.ServiceContext{DocumentosClient: &documentosSubidaFalsa{}})
	grande := bytes.Repeat([]byte("a"), 2*1024*1024)
	cuerpo := append(append([]byte(`{"correo":"x@y.example","password":"`), grande...), []byte(`"}`)...)
	if c := enviar(t, base+"/api/auth/login", cuerpo); c != http.StatusRequestEntityTooLarge {
		t.Fatalf("login de %d bytes: HTTP %d, se esperaba 413", len(cuerpo), c)
	}
}

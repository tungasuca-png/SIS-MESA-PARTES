package integration

import (
	"fmt"
	"sync/atomic"
	"time"
)

var dniCounter atomic.Int64

// nextTestDNI devuelve un DNI válido (8 dígitos, texto) único dentro de la
// ejecución y prácticamente único entre ejecuciones. Empieza en "9" para no
// chocar con DNI reales/demo (que comienzan en 1 en los datos actuales).
func nextTestDNI() string {
	n := (time.Now().UnixNano()/1000 + dniCounter.Add(1)) % 10000000
	return fmt.Sprintf("9%07d", n)
}

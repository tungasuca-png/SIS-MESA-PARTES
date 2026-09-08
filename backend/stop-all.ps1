# Baja los 5 procesos que levanta run-all.ps1.
#
# No se usan los PID que guarda run-all.ps1 en logs\pids.json: "go run"
# lanza un proceso "go.exe" que a su vez compila y ejecuta el binario real
# (auth.exe, gateway.exe, etc.) como un proceso aparte, asi que el PID util
# para matar es el del binario, no el del "go.exe" que lo lanzo. Se buscan
# por nombre en su lugar.

$names = @("auth", "expedientes", "usuarios", "documentos", "gateway")

$found = Get-Process -Name $names -ErrorAction SilentlyContinue
if (-not $found) {
    Write-Host "No hay procesos del backend corriendo (auth/expedientes/usuarios/documentos/gateway)." -ForegroundColor Yellow
    exit 0
}

$found | Select-Object Id, Name | Format-Table -AutoSize
$found | Stop-Process -Force

Write-Host "Detenidos." -ForegroundColor Green

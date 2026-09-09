# Levanta los 6 procesos del backend (Auth, Expedientes, Usuarios,
# Documentos, Derivaciones, Gateway) con un solo comando, todos con el mismo
# JWT_SECRET.
#
# Uso:
#   cd backend
#   .\run-all.ps1
#
# Cada servicio corre en segundo plano (via "go run", que compila y
# ejecuta); su salida queda en logs\<servicio>.log. Para bajarlos todos:
#   .\stop-all.ps1

$ErrorActionPreference = "Stop"
$root = $PSScriptRoot

$envFile = Join-Path $root ".env"
if (-not (Test-Path $envFile)) {
    $bytes = New-Object byte[] 32
    [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    $secret = -join ($bytes | ForEach-Object { $_.ToString("x2") })
    "JWT_SECRET=$secret" | Out-File -FilePath $envFile -Encoding utf8 -NoNewline
    Write-Host "No existia backend\.env: se genero un JWT_SECRET nuevo y se guardo ahi." -ForegroundColor Yellow
}

$jwtLine = Get-Content $envFile | Where-Object { $_ -match "^JWT_SECRET=" } | Select-Object -First 1
if (-not $jwtLine) {
    throw "backend\.env existe pero no tiene una linea JWT_SECRET=... Revisa backend\.env.example."
}
$env:JWT_SECRET = ($jwtLine -replace "^JWT_SECRET=", "").Trim()

$logsDir = Join-Path $root "logs"
New-Item -ItemType Directory -Force -Path $logsDir | Out-Null

# Orden: los 5 RPC primero (se registran en etcd), el Gateway al final (les
# apunta a traves de etcd, no importa si tarda un segundo mas en levantar).
$services = @(
    @{ Name = "auth";         Dir = "services/auth";         Main = "auth.go";         Yaml = "etc/auth.yaml";         Port = 8080 }
    @{ Name = "expedientes";  Dir = "services/expedientes";  Main = "expedientes.go";  Yaml = "etc/expedientes.yaml";  Port = 8082 }
    @{ Name = "usuarios";     Dir = "services/usuarios";     Main = "usuarios.go";     Yaml = "etc/usuarios.yaml";     Port = 8083 }
    @{ Name = "documentos";   Dir = "services/documentos";   Main = "documentos.go";   Yaml = "etc/documentos.yaml";   Port = 8084 }
    @{ Name = "derivaciones"; Dir = "services/derivaciones"; Main = "derivaciones.go"; Yaml = "etc/derivaciones.yaml"; Port = 8085 }
    @{ Name = "gateway";      Dir = "gateway";               Main = "gateway.go";      Yaml = "etc/gateway-api.yaml";  Port = 8888 }
)

$pidsFile = Join-Path $logsDir "pids.json"
$started = @()

foreach ($svc in $services) {
    $dir = Join-Path $root $svc.Dir
    $outLog = Join-Path $logsDir "$($svc.Name).log"
    $errLog = Join-Path $logsDir "$($svc.Name).err.log"

    Write-Host "Iniciando $($svc.Name) (puerto $($svc.Port))..." -ForegroundColor Cyan
    $proc = Start-Process -FilePath "go" `
        -ArgumentList @("run", $svc.Main, "-f", $svc.Yaml) `
        -WorkingDirectory $dir `
        -RedirectStandardOutput $outLog `
        -RedirectStandardError $errLog `
        -WindowStyle Hidden `
        -PassThru

    $started += [PSCustomObject]@{ name = $svc.Name; pid = $proc.Id; port = $svc.Port }
}

$started | ConvertTo-Json | Out-File -FilePath $pidsFile -Encoding utf8

Write-Host ""
Write-Host "Los 6 procesos se lanzaron (compilando con 'go run', puede tardar unos segundos)." -ForegroundColor Green
Write-Host "Logs en backend\logs\<servicio>.log - para bajarlos todos: .\stop-all.ps1" -ForegroundColor Green
$started | Format-Table -AutoSize

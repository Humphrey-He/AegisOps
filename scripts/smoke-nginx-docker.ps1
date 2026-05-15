param(
    [string]$BaseUrl = "http://127.0.0.1:8080",
    [string]$Username = "admin",
    [string]$Password = "admin123456",
    [string]$ContainerName = "aegisops-smoke-nginx",
    [int]$SshPort = 2222,
    [int]$HttpPort = 18080
)

$ErrorActionPreference = "Stop"

function Invoke-Aegis {
    param(
        [string]$Method,
        [string]$Path,
        [hashtable]$Headers,
        [object]$Body = $null
    )

    $params = @{
        Method = $Method
        Uri = "$BaseUrl$Path"
    }
    if ($Headers) {
        $params.Headers = $Headers
    }
    if ($null -ne $Body) {
        $params.ContentType = "application/json"
        $params.Body = ($Body | ConvertTo-Json -Depth 20)
    }
    Invoke-RestMethod @params
}

function Wait-Task {
    param(
        [string]$TaskId,
        [hashtable]$Headers,
        [int]$TimeoutSeconds = 60
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        Start-Sleep -Seconds 1
        $task = Invoke-Aegis -Method Get -Path "/api/tasks/$TaskId" -Headers $Headers
        if ($task.data.status -in @("SUCCESS", "FAILED", "CANCELED")) {
            return $task.data
        }
    } while ((Get-Date) -lt $deadline)

    throw "task $TaskId did not finish within $TimeoutSeconds seconds"
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$image = "aegisops/nginx-ssh-smoke:local"
$dockerfileDir = Join-Path $repoRoot "deploy\smoke\nginx-ssh"

Write-Host "Building smoke image $image ..."
docker build -t $image $dockerfileDir | Out-Host
if ($LASTEXITCODE -ne 0) {
    throw "docker build failed with exit code $LASTEXITCODE"
}

$existing = docker ps -aq --filter "name=^/$ContainerName$"
if ($existing) {
    Write-Host "Removing existing container $ContainerName ..."
    docker rm -f $ContainerName | Out-Host
    if ($LASTEXITCODE -ne 0) {
        throw "docker rm failed with exit code $LASTEXITCODE"
    }
}

Write-Host "Starting $ContainerName on SSH port $SshPort and HTTP port $HttpPort ..."
docker run -d --name $ContainerName -p "${SshPort}:22" -p "${HttpPort}:80" $image | Out-Host
if ($LASTEXITCODE -ne 0) {
    throw "docker run failed with exit code $LASTEXITCODE"
}

$deadline = (Get-Date).AddSeconds(30)
do {
    Start-Sleep -Seconds 1
    try {
        $http = Invoke-WebRequest -Uri "http://127.0.0.1:$HttpPort" -UseBasicParsing -TimeoutSec 3
        if ($http.StatusCode -eq 200) {
            break
        }
    } catch {
        if ((Get-Date) -ge $deadline) {
            throw
        }
    }
} while ((Get-Date) -lt $deadline)

$login = Invoke-Aegis -Method Post -Path "/api/auth/login" -Body @{ username = $Username; password = $Password }
$headers = @{ Authorization = "Bearer $($login.data.tokens.accessToken)" }
$stamp = Get-Date -Format "yyyyMMddHHmmss"

$secret = Invoke-Aegis -Method Post -Path "/api/secrets" -Headers $headers -Body @{
    name = "Smoke Nginx SSH Password $stamp"
    type = "SSH_PASSWORD"
    purpose = "host_ssh"
    value = "aegisops123"
    description = "Docker smoke nginx SSH password"
}

$hostNode = Invoke-Aegis -Method Post -Path "/api/hosts" -Headers $headers -Body @{
    name = "Smoke Nginx Docker Host $stamp"
    address = "127.0.0.1"
    sshPort = $SshPort
    sshUser = "root"
    sshSecretId = $secret.data.id
    group = "smoke"
    tags = "docker,nginx,ssh"
}

$hostTest = Invoke-Aegis -Method Post -Path "/api/hosts/$($hostNode.data.id)/test-ssh" -Headers $headers
$hostTask = Wait-Task -TaskId $hostTest.data.taskId -Headers $headers
if ($hostTask.status -ne "SUCCESS") {
    throw "host SSH smoke failed: $($hostTask.errorMessage)"
}

$nginxNode = Invoke-Aegis -Method Post -Path "/api/nginx/nodes" -Headers $headers -Body @{
    name = "Smoke Docker Nginx $stamp"
    hostId = $hostNode.data.id
    configPath = "/etc/nginx/nginx.conf"
    testCommand = "nginx -t"
    reloadCommand = "nginx -s reload"
    description = "Docker-backed Nginx smoke node"
}

$configContent = @"
worker_processes auto;
events {
    worker_connections 1024;
}
http {
    include /etc/nginx/mime.types;
    default_type application/octet-stream;
    sendfile on;
    server {
        listen 80;
        server_name _;
        location / {
            add_header X-AegisOps-Smoke "$stamp";
            return 200 'AegisOps Nginx smoke $stamp\n';
        }
    }
}
"@

$config = Invoke-Aegis -Method Post -Path "/api/nginx/nodes/$($nginxNode.data.id)/configs" -Headers $headers -Body @{
    version = "smoke-$stamp"
    content = $configContent
    message = "Docker Nginx smoke config"
}

$publish = Invoke-Aegis -Method Post -Path "/api/nginx/nodes/$($nginxNode.data.id)/publish" -Headers $headers -Body @{
    configId = $config.data.id
}
$publishTask = Wait-Task -TaskId $publish.data.taskId -Headers $headers
if ($publishTask.status -ne "SUCCESS") {
    throw "nginx publish smoke failed: $($publishTask.errorMessage)"
}

$nginxTest = Invoke-Aegis -Method Post -Path "/api/nginx/nodes/$($nginxNode.data.id)/test" -Headers $headers
$nginxTask = Wait-Task -TaskId $nginxTest.data.taskId -Headers $headers
if ($nginxTask.status -ne "SUCCESS") {
    throw "nginx test smoke failed: $($nginxTask.errorMessage)"
}

$page = Invoke-WebRequest -Uri "http://127.0.0.1:$HttpPort" -UseBasicParsing
$pageBody = $page.Content
if ($pageBody -is [byte[]]) {
    $pageBody = [System.Text.Encoding]::UTF8.GetString($pageBody)
}

[ordered]@{
    container = $ContainerName
    sshPort = $SshPort
    httpPort = $HttpPort
    hostId = $hostNode.data.id
    hostTaskId = $hostTask.id
    nginxNodeId = $nginxNode.data.id
    configId = $config.data.id
    publishTaskId = $publishTask.id
    nginxTestTaskId = $nginxTask.id
    httpStatus = [int]$page.StatusCode
    body = $pageBody.Trim()
} | ConvertTo-Json -Depth 8

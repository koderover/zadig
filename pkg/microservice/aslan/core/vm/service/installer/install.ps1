param(
    [Parameter(Mandatory = $true)][string]$Token,
    [string]$ServerUrl = '__ZADIG_SERVER_URL__',
    [string]$DownloadUrl,
    [string]$ResourceUrl = '__ZADIG_RESOURCE_URL__',
    [string]$Version = '__ZADIG_AGENT_VERSION__',
    [string]$WorkDir,
    [switch]$Upgrade
)

$ErrorActionPreference = 'Stop'
if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) { throw 'This installer requires Windows; use install.sh on Linux or macOS' }
$architecture = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
if ($architecture -ne 'AMD64') { throw "Unsupported Windows CPU architecture: $architecture; this release supports amd64" }
if (-not $DownloadUrl) {
  if ($Version -notmatch '^[a-zA-Z0-9][a-zA-Z0-9._+-]*$') { throw 'Invalid agent release version' }
  $DownloadUrl = $ResourceUrl.TrimEnd('/') + '/zadig-agent-windows-amd64-v' + $Version + '.exe'
}
foreach ($rawUrl in @($ServerUrl, $DownloadUrl)) {
    $parsed = $null
    if (-not [Uri]::TryCreate($rawUrl, [UriKind]::Absolute, [ref]$parsed) -or $parsed.Scheme -notin @('http', 'https') -or -not $parsed.Host -or $parsed.UserInfo -or $parsed.Query -or $parsed.Fragment) { throw 'Installation requires an HTTP(S) URL without credentials, query or fragment' }
}
$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw 'Run PowerShell as administrator' }
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$agentHome = $HOME
$agent = Join-Path $agentHome 'zadig-agent.exe'
$settings = Join-Path $agentHome '.zadig-agent\service.json'
$service = Get-Service -Name zadig-agent -ErrorAction SilentlyContinue
if ($service) {
  $config = Get-CimInstance Win32_Service -Filter "Name = 'zadig-agent'"
  if ($config.StartName -notin @('LocalSystem', 'NT AUTHORITY\SYSTEM')) { throw 'Existing service uses another account; configure LocalSystem before updating, keeping its executable and agent configuration' }
  $agent = if ($config.PathName -match '^"([^"]+)"') { $Matches[1] } else { ($config.PathName -split '\s+', 2)[0] }
  $settingsArg = [regex]::Match($config.PathName, '(?:^|\s)--settings\s+(?:"([^"]+)"|(\S+))')
  if (-not $settingsArg.Success) { throw 'Existing service settings path is missing; repair the service command before updating' }
  $settings = if ($settingsArg.Groups[1].Success) { $settingsArg.Groups[1].Value } else { $settingsArg.Groups[2].Value }
  if (-not [IO.Path]::IsPathRooted($agent) -or -not [IO.Path]::IsPathRooted($settings)) { throw 'Existing service paths must be absolute' }
  if ((Get-Item -Force -LiteralPath $settings).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Refusing linked service settings' }
  $registered = Get-Content -Raw -LiteralPath $settings | ConvertFrom-Json
  $agentHome = $registered.home
  if (-not [IO.Path]::IsPathRooted($agentHome) -or $registered.user -ne 'LocalSystem' -or $registered.backend -ne 'windows' -or -not [string]::Equals($registered.executable, $agent, [StringComparison]::OrdinalIgnoreCase) -or -not [string]::Equals((Join-Path $agentHome '.zadig-agent\service.json'), $settings, [StringComparison]::OrdinalIgnoreCase)) { throw 'Existing service settings do not match the service command; repair them before updating' }
}
$autoStart = if ($service) { $service.StartType -eq 'Automatic' } else { $true }
$download = "$agent.download-$([guid]::NewGuid().ToString('N')).exe"
foreach ($path in @($agentHome, (Split-Path -Parent $agent), (Split-Path -Parent $settings))) {
  if ((Test-Path -LiteralPath $path) -and ((Get-Item -Force -LiteralPath $path).Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw "Refusing linked agent directory: $path" }
}
if ((Test-Path -LiteralPath $agent) -and ((Get-Item -Force -LiteralPath $agent).Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw 'Refusing linked agent executable' }
if (-not $Upgrade -and ($service -or (Test-Path -LiteralPath $agent))) { throw 'Agent already exists; use the recovery or upgrade command' }
$previousProfile = $env:USERPROFILE
Push-Location -LiteralPath $agentHome
try {
  $env:USERPROFILE = $agentHome
  Invoke-WebRequest -UseBasicParsing -Uri $downloadUrl -OutFile $download
  if ($Upgrade) {
    if ($service -or (Test-Path -LiteralPath $agent)) {
      & $download stop
      if ($LASTEXITCODE -ne 0) { throw 'Could not stop the existing agent' }
      if ($service -and $service.Status -ne 'Stopped') { Stop-Service -Name zadig-agent; $service.WaitForStatus('Stopped', [TimeSpan]::FromSeconds(60)) }
      Start-Sleep -Seconds 6
      & $download wait-for-stop
      if ($LASTEXITCODE -ne 0) { throw 'Existing agent has not stopped' }
      if (Test-Path -LiteralPath $agent) {
        $backup = "$agent.previous-$([guid]::NewGuid().ToString('N'))"
        Move-Item -LiteralPath $agent -Destination $backup
        Write-Host "Previous executable: $backup"
      }
    }
  }
  Move-Item -LiteralPath $download -Destination $agent
  $agentArgs = @('install', '--service-user', 'LocalSystem', "--auto-start=$autoStart", '--server-url', $ServerUrl, '--token', $Token)
  if ($WorkDir -and (-not $Upgrade -or -not (Test-Path -LiteralPath (Join-Path $agentHome '.zadig-agent\agent_config.yaml')))) { $agentArgs += @('--work-dir', $WorkDir) }
  & $agent @agentArgs
  if ($LASTEXITCODE -ne 0) { throw 'Agent installation/start failed' }
} finally {
  $env:USERPROFILE = $previousProfile
  Pop-Location
  Remove-Item -LiteralPath $download -Force -ErrorAction SilentlyContinue
}

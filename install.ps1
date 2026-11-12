# No administrator access, machine-wide PATH changes or global Python install.
$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath $PSScriptRoot
try {
    $RuntimeArch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
    switch ($RuntimeArch) {
        'X64' { $Target = 'x86_64-pc-windows-msvc'; $Expected = '0cdc5b38fe5a8168ed6a25814f5ba43c21d313dd3187314fccb37b7cec46e021' }
        'Arm64' { $Target = 'aarch64-pc-windows-msvc'; $Expected = '505d9e65e7692bac3ddf98c66754aa26c5050c409c184af30a881a0a72cc85e3' }
        default { throw 'This launcher requires 64-bit Windows. See docs/installer-hosts.md for manual Python setup.' }
    }
    $Cache = Join-Path $PSScriptRoot '.installer'
    New-Item -ItemType Directory -Force -Path $Cache | Out-Null
    $Uv = Join-Path $Cache "uv-0.13.7-$Target.exe"
    if (-not (Test-Path -LiteralPath $Uv)) {
        Write-Host 'Preparing the installer (first launch needs Internet access)…'
        $Temp = Join-Path $Cache ([guid]::NewGuid().ToString())
        New-Item -ItemType Directory -Path $Temp | Out-Null
        try {
            [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
            $Archive = Join-Path $Temp 'uv.zip'
            Invoke-WebRequest -UseBasicParsing -Uri "https://github.com/astral-sh/uv/releases/download/0.13.7/uv-$Target.zip" -OutFile $Archive -TimeoutSec 180
            if ((Get-FileHash -LiteralPath $Archive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $Expected) {
                throw 'Runtime manager checksum mismatch. Nothing was executed.'
            }
            Expand-Archive -LiteralPath $Archive -DestinationPath $Temp
            Move-Item -LiteralPath (Join-Path $Temp 'uv.exe') -Destination $Uv
        } finally { Remove-Item -LiteralPath $Temp -Force -Recurse }
    }
    $env:UV_PYTHON_INSTALL_DIR = Join-Path $Cache 'python'
    $env:UV_CACHE_DIR = Join-Path $Cache 'cache'
    $env:UV_NO_MODIFY_PATH = '1'
    $Python = Join-Path $PSScriptRoot '.venv\Scripts\python.exe'
    if (-not (Test-Path -LiteralPath $Python)) {
        & $Uv venv --managed-python --python 3.13 .venv
        if ($LASTEXITCODE -ne 0) { throw 'Python setup failed. See docs/installer-hosts.md.' }
    }
    & $Python -c 'import sys; assert sys.version_info >= (3,12)'
    if ($LASTEXITCODE -ne 0) { throw 'The existing .venv is too old. Rename it and launch again.' }
    & $Python -c "import importlib.util,sys; sys.exit(importlib.util.find_spec('paramiko') is None)"
    if ($LASTEXITCODE -ne 0) {
        & $Uv pip install --python $Python -r requirements.txt
        if ($LASTEXITCODE -ne 0) { throw 'Dependency download failed. Check Internet access and retry.' }
    }
    & $Python -m flasher
    if ($LASTEXITCODE -ne 0) { throw 'Installer stopped. Check the error above before retrying.' }
} catch {
    Write-Host "Setup stopped: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host 'See docs/installer-hosts.md or https://github.com/Spectre-Works/kdbox-xbt/issues'
    exit 1
}

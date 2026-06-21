# Windows 本地启动与开机自启

项目默认本地地址：

```powershell
http://localhost:3007
```

## 一键启动

```powershell
cd D:\project\cctapi
powershell -ExecutionPolicy Bypass -File scripts\start-cctapi.ps1
```

只启动服务，不自动打开浏览器：

```powershell
powershell -ExecutionPolicy Bypass -File scripts\start-cctapi.ps1 -NoBrowser
```

## 停止服务

```powershell
powershell -ExecutionPolicy Bypass -File scripts\stop-cctapi.ps1
```

## 安装开机自启

安装后，Windows 登录时会自动启动 `one-api.exe`，端口为 `3007`。

```powershell
powershell -ExecutionPolicy Bypass -File scripts\install-cctapi-autostart.ps1
```

## 取消开机自启

```powershell
powershell -ExecutionPolicy Bypass -File scripts\uninstall-cctapi-autostart.ps1
```

## 日志

启动脚本会把输出写到：

```text
logs/one-api.stdout.log
logs/one-api.stderr.log
```


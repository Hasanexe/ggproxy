# GGProxy

GGProxy is a lightweight SOCKS5/HTTP forward proxy written in Go. It aims for minimal overhead, no caching, no traffic modification, and no filtering.

## Features

- **SOCKS5** or **HTTP** modes (`proxy_mode`).
- IP-based allowlisting via `allowed_ip` (CIDR, IPv4 only).
- Optional authentication (HTTP Basic Auth and SOCKS5 username/password).
- Minimal logging – no traffic inspection.

## Installation

Download the latest `.deb` package from [Releases](https://github.com/hasanexe/ggproxy/releases).

Install using:

```bash
sudo dpkg -i ggproxy_<version>.deb
```

## Configuration

By default, GGProxy reads `/etc/ggproxy.conf` on Linux or `ggproxy.conf` in the current directory on Windows. You can also pass `--config=/path/to/ggproxy.conf`. Example:

```ini
proxy_mode = http
port = 3128
log_level = debug
allowed_ip = 192.168.1.0/24
allowed_ip = 10.0.0.0/8

idle_timeout = 30s
buffer_size = 65536
auth_user = username
auth_pass = password
```

**Configuration fields**:

- `proxy_mode`: `http` or `socks` (default: `http`)
- `port`: Listening port (default: `3128`)
- `log_level`: `debug`, `basic`, or `off` (default: `basic`)
- `allowed_ip`: One per line, CIDR format (IPv4 only)
- `idle_timeout`: Connection idle timeout (default: `30s`)
- `buffer_size`: Internal buffer size for copy operations (default: `32KB`)
- `auth_user` / `auth_pass`: Optional credentials for authentication (both required if used)
- `log_level=off`: Disables log output (messages are still drained internally to avoid blocking)

## Usage

### Linux

GGProxy runs automatically as a systemd service. Manage it with:

```bash
sudo systemctl status ggproxy
sudo systemctl restart ggproxy
sudo systemctl stop ggproxy
```

### Windows

Run the executable directly with an optional config file:

```cmd
gg.exe --config=ggproxy.conf
```

## Testing

### HTTP Mode with Authentication

```bash
curl -U username:password -x http://127.0.0.1:3128 http://example.com
```

### SOCKS5 Mode with Authentication

```bash
curl -U username:password --socks5 127.0.0.1:3128 http://example.com
```

### View Logs

On Linux, check logs via journald:

```bash
journalctl -u ggproxy -f
```

## Architecture

GGProxy uses a goroutine-based concurrent architecture:

- **Connection Handling**: Each client connection is handled in its own goroutine
- **Bidirectional Tunneling**: Symmetric goroutines manage client→remote and remote→client data flow
- **Buffer Pooling**: Efficient memory management via `sync.Pool` during data copying
- **Async Logging**: Non-blocking log writes to stdout via buffered channel (captured by systemd on Linux)

## License

GGProxy is licensed under the [MIT License](https://opensource.org/licenses/MIT).
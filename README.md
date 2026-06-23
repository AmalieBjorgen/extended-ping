## Netprobe (`netprobe`)

Netprobe is a Go command-line tool similar to PowerShell's `Test-NetConnection`. It allows you to perform TCP port connectivity checks and fallback to unprivileged ICMP pings.

### Usage

```bash
go build -o netprobe .
./netprobe <host> [options]
```

### Options

* `-p, --port <port>`: TCP port to check (e.g., `80`, `443`).
* `-t, --timeout <timeout>`: Timeout duration (e.g., `3s`, `500ms`, `5`).
* `-c, --count <count>`: Number of attempts to perform (default: `1`).
* `--cert`: Inspect TLS certificate details (defaults to port `443` if no port is specified).
* `-h, --help`: Show help message.

### Fallback Behavior

If you specify a port (e.g., `./netprobe google.com -p 443`), `netprobe` will first attempt a TCP connection to that port. If the TCP connection fails, it will automatically fallback to an ICMP ping. If no port is specified, it runs a standard ICMP ping directly.

On Linux and macOS, the ICMP ping runs unprivileged via `udp4` sockets, so it does not require `root` or `sudo` permissions on systems that allow user ping group ranges.

### Certificate Inspection

If you pass the `--cert` flag, `netprobe` will connect via TLS to the host and port (defaulting to port `443` if `-p` is omitted), retrieve the certificate chain, and display key details such as the Subject, Issuer, Validity Period, Time until expiration, System verification status, and Subject Alternative Names (SANs).



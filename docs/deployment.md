# Personal VPS deployment

Image Tool Box is designed to run behind a reverse proxy. It listens on localhost, while Nginx or Caddy terminates HTTPS and proxies requests to it:

```text
Internet (HTTPS) -> Nginx or Caddy -> 127.0.0.1:8080 -> itb serve
```

`itb` does not implement TLS, ACME certificate issuance, user management, or S3 HTTP APIs.

## Token

Set one long random token in a file that only the service account can read:

```bash
sudo install -d -m 700 /etc/itb
sudo sh -c 'printf "%s\\n" "ITB_API_TOKEN=replace-with-a-long-random-token" > /etc/itb/itb.env'
sudo chmod 600 /etc/itb/itb.env
```

Do not put the token on the `itb serve` command line. `--no-auth` is only for local development and refuses non-loopback addresses.

## systemd

```ini
[Unit]
Description=Image Tool Box API
After=network.target

[Service]
Type=simple
User=itb
Group=itb
EnvironmentFile=/etc/itb/itb.env
ExecStart=/usr/local/bin/itb serve --addr 127.0.0.1:8080
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

After installing the unit, run `sudo systemctl daemon-reload`, `sudo systemctl enable --now itb`, and verify `curl http://127.0.0.1:8080/api/v1/health`.

`SIGTERM` triggers a graceful shutdown. The server stops accepting new work and gives in-flight requests up to ten seconds to finish.

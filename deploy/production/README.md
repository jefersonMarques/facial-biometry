# FaceProof Production Deployment

This profile is the Linux Production Candidate perimeter for a single FaceProof instance.

## Network model

```text
Internet
   |
   | TCP 443 (HTTPS)
   v
Caddy
   |
   | HTTP loopback only
   v
127.0.0.1:5173  FaceProof Web Gateway
   |
   | HTTP loopback only
   v
127.0.0.1:8180  FaceProof Go API
   |
   +--> libfaceproof_core.so (in-process)
   +--> encrypted identity/template state
   +--> optional external PostgreSQL analytics

127.0.0.1:5174  Admin UI (optional, never publicly proxied)
```

Only TCP 80/443 should be reachable from the Internet. Port 80 exists only for ACME/HTTPS redirect when managed by Caddy. Ports 5173, 5174 and 8180 must remain loopback-only.

The admin UI is intentionally absent from the public Caddy configuration. Access it through an SSH tunnel or a private VPN:

```bash
ssh -L 5174:127.0.0.1:5174 user@faceproof-server
```

Then use `http://127.0.0.1:5174` locally.

## Filesystem layout

```text
/opt/faceproof/
  bin/
    faceproof-api
    faceproof-web-gateway
  lib/
    libfaceproof_core.so
  models/
    faceproof-standard.fpmp
  web/
    sdk/
    admin/

/etc/faceproof/
  faceproof.env
  tenants.json

/var/lib/faceproof/
  identity-checks/
  templates/
  model-cache/
```

Use a dedicated unprivileged Linux account named `faceproof`.

Recommended ownership:

```bash
sudo chown -R root:faceproof /opt/faceproof /etc/faceproof
sudo chmod -R go-w /opt/faceproof
sudo chmod 750 /etc/faceproof
sudo chmod 640 /etc/faceproof/faceproof.env /etc/faceproof/tenants.json

sudo chown -R faceproof:faceproof /var/lib/faceproof
sudo chmod 700 /var/lib/faceproof
```

## Build the Go binaries

From `services/api`:

```bash
go test ./...
go build -trimpath -o /opt/faceproof/bin/faceproof-api ./cmd/server
go build -trimpath -o /opt/faceproof/bin/faceproof-web-gateway ./cmd/devgateway
```

Build the Secure Core using the repository build script and place the validated `libfaceproof_core.so` at:

```text
/opt/faceproof/lib/libfaceproof_core.so
```

Deploy the signed production Model Pack to:

```text
/opt/faceproof/models/faceproof-standard.fpmp
```

The production signing private key must not exist on the FaceProof server.

Build the browser SDK before copying `apps/web-sdk` to `/opt/faceproof/web/sdk`:

```bash
cd apps/web-sdk
npm ci
npm run typecheck
npm run build
npm run test:liveness-v2
```

Copy `apps/admin` to `/opt/faceproof/web/admin`.

## Runtime configuration

Copy:

```text
deploy/production/faceproof.env.example
```

to:

```text
/etc/faceproof/faceproof.env
```

Replace every placeholder before starting the service.

Production mode is fail-closed:

- `FACEPROOF_API_ADDR` must be loopback;
- `FACEPROOF_ALLOWED_ORIGIN` must be HTTPS;
- `FACEPROOF_IDENTITY_VERIFY_URL` must be HTTPS;
- Secure Core, signed Model Pack and tenant registry are mandatory;
- the web/admin gateway refuses public listen addresses;
- the gateway refuses non-loopback API upstreams.

The current environment file is a Production Candidate secret mechanism. KMS/HSM/systemd credential integration remains a later hardening milestone.

## systemd

Install the units:

```bash
sudo cp deploy/production/systemd/faceproof-api.service /etc/systemd/system/
sudo cp deploy/production/systemd/faceproof-web.service /etc/systemd/system/
sudo cp deploy/production/systemd/faceproof-admin.service /etc/systemd/system/
sudo systemctl daemon-reload
```

Start the public application path:

```bash
sudo systemctl enable --now faceproof-api
sudo systemctl enable --now faceproof-web
```

Start the admin UI only when required:

```bash
sudo systemctl enable --now faceproof-admin
```

The units run as the unprivileged `faceproof` account with a read-only system/application filesystem and write access restricted to `/var/lib/faceproof`.

## Caddy / TLS

Install Caddy using your operating-system package policy.

Copy `deploy/production/Caddyfile` to `/etc/caddy/Caddyfile` and replace:

```text
faceproof.example.com
```

with the real public hostname.

The hostname must resolve to the production server before certificate issuance.

Validate and reload:

```bash
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

Caddy terminates TLS and proxies to `127.0.0.1:5173`. It also adds HSTS. The Go API is never a public listener.

## Firewall

The host/network firewall should permit only the services actually required by the deployment. For the public FaceProof perimeter this normally means:

```text
22/tcp   administrative SSH, preferably source-restricted
80/tcp   ACME + redirect to HTTPS
443/tcp  public FaceProof HTTPS
```

Do not expose:

```text
5173/tcp
5174/tcp
8180/tcp
8090/tcp
```

Port 8090 is not used by the production C++ runtime.

## Validation

After starting API, web gateway and Caddy:

```bash
bash scripts/validate-production-network.sh
```

Optionally validate the public HTTPS path too:

```bash
FACEPROOF_PUBLIC_URL=https://faceproof.example.com \
  bash scripts/validate-production-network.sh
```

The validator fails if API/web/admin internal ports are exposed outside loopback.

Also inspect listeners manually:

```bash
ss -ltnp | grep -E ':(80|443|5173|5174|8180|8090)\b'
```

Expected production topology:

```text
:80/:443          Caddy
127.0.0.1:5173    FaceProof web gateway
127.0.0.1:5174    optional admin gateway
127.0.0.1:8180    FaceProof API
8090              absent
```

## Production boundary

This profile establishes TLS termination, reverse proxying and single-host port isolation.

It does not yet claim:

- distributed rate limiting across multiple FaceProof hosts;
- shared/distributed biometric session storage;
- shared quota authority for multiple API replicas;
- KMS/HSM-backed runtime secret retrieval;
- independent external security certification.

Those remain explicit roadmap items.

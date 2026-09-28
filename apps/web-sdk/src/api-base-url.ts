export function resolveApiBaseUrl(): string {
    const override = new URLSearchParams(window.location.search).get("api");
    if (override && isLocalHostname(window.location.hostname)) {
        try {
            const url = new URL(override);
            if ((url.protocol === "http:" || url.protocol === "https:") && isLocalHostname(url.hostname)) {
                return url.origin;
            }
        } catch {
            // Ignora override inválido e mantém o mesmo origin.
        }
    }

    return window.location.origin;
}

function isLocalHostname(hostname: string): boolean {
    return hostname === "localhost" || hostname === "127.0.0.1" || hostname === "::1";
}

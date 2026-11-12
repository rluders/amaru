import createOpenApiClient from "openapi-fetch";
import type { components, paths } from "./generated/schema.ts";

export interface ClientOptions {
  /** Platform API root including the major version, for example http://127.0.0.1:7777/v1. */
  baseUrl: string;
  /** Bearer token. Sent as an Authorization header only; never put it in a URL for HTTP calls. */
  token?: string;
  /** Browser transport: read the CSRF value obtained from GET /auth/session from memory. Mutations require it. */
  csrfToken?(): string | undefined;
  /** Replaces globalThis.fetch (tests, custom transports). */
  fetch?: typeof globalThis.fetch;
}

/** Typed HTTP client: every path, parameter, body and response comes from the generated contract types. */
export function createClient(options: ClientOptions) {
  if (options.token && options.csrfToken) throw new Error("Choose native bearer or browser session authentication.");
  const browser = !options.token;
  const client = createOpenApiClient<paths>({
    baseUrl: options.baseUrl,
    credentials: browser ? "same-origin" : "omit",
    redirect: "error",
    ...(options.fetch ? { fetch: options.fetch } : {}),
    ...(options.token ? { headers: { Authorization: `Bearer ${options.token}` } } : {}),
  });
  client.use({
    async onRequest({ request, schemaPath, options: transport }) {
      if (schemaPath === "/health") {
        request.headers.delete("Authorization");
        request.headers.delete("Cookie");
        request.headers.delete("X-Amaru-CSRF");
        return new Request(request, { credentials: "omit" });
      }
      // Check every operation: a restarted/replaced Instance must negotiate again.
      await checkCompatibility(transport.baseUrl, transport.fetch, request.signal);
      if (schemaPath === "/auth/launch") {
        // openapi-fetch concatenates baseUrl and path; it does not apply path servers.
        const url = new URL(request.url);
        url.pathname = "/auth/launch";
        request.headers.delete("Authorization");
        request.headers.delete("Cookie");
        return new Request(url, { method: request.method, headers: request.headers, credentials: "omit", redirect: "manual", signal: request.signal });
      }
      if (browser && request.method !== "GET") {
        const csrf = options.csrfToken?.();
        if (!csrf) throw new Error("Browser mutation requires a session CSRF token.");
        request.headers.set("X-Amaru-CSRF", csrf);
      }
    },
  });
  return client;
}

export type Client = ReturnType<typeof createClient>;

export type CompatibilityFailure = "connection" | "invalid-health" | "incompatible";

/** Safe to display as a connection/compatibility state; contains no URL, credentials or response body. */
export class CompatibilityError extends Error {
  readonly kind: CompatibilityFailure;

  constructor(kind: CompatibilityFailure) {
    super({
      connection: "Cannot connect to the Instance health endpoint.",
      "invalid-health": "The Instance returned an invalid health response.",
      incompatible: "The Instance protocol is incompatible; this client supports protocol major 1.",
    }[kind]);
    this.name = "CompatibilityError";
    this.kind = kind;
  }
}

async function checkCompatibility(baseUrl: string, fetch: typeof globalThis.fetch, signal: AbortSignal): Promise<void> {
  const probe = createOpenApiClient<paths>({ baseUrl, fetch });
  let health: components["schemas"]["Health"] | undefined;
  try {
    const result = await probe.GET("/health", { signal, redirect: "error", cache: "no-store", credentials: "omit" });
    if (!result.response.ok) throw new CompatibilityError("connection");
    health = result.data;
  } catch (error) {
    if (error instanceof CompatibilityError) throw error;
    if (error instanceof SyntaxError) throw new CompatibilityError("invalid-health");
    throw new CompatibilityError("connection");
  }
  // Generated types describe valid wire data; they do not validate an untrusted response.
  if (!health || typeof health !== "object" ||
      !Number.isSafeInteger(health.protocolVersion) || health.protocolVersion < 1 ||
      health.status !== "ok" || typeof health.platformVersion !== "string" || !health.platformVersion ||
      typeof health.startedAt !== "string" || !Number.isFinite(Date.parse(health.startedAt)) ||
      !Number.isSafeInteger(health.uptimeSec) || health.uptimeSec < 0) {
    throw new CompatibilityError("invalid-health");
  }
  if (health.protocolVersion !== 1) throw new CompatibilityError("incompatible");
}

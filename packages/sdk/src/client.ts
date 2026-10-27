import createOpenApiClient from "openapi-fetch";
import type { paths } from "./generated/schema.ts";

export interface ClientOptions {
  /** Platform API root including the major version, for example http://127.0.0.1:7777/v1. */
  baseUrl: string;
  /** Bearer token. Sent as an Authorization header only; never put it in a URL for HTTP calls. */
  token?: string;
  /** Replaces globalThis.fetch (tests, custom transports). */
  fetch?: typeof globalThis.fetch;
}

/** Typed HTTP client: every path, parameter, body and response comes from the generated contract types. */
export function createClient(options: ClientOptions) {
  return createOpenApiClient<paths>({
    baseUrl: options.baseUrl,
    ...(options.fetch ? { fetch: options.fetch } : {}),
    ...(options.token ? { headers: { Authorization: `Bearer ${options.token}` } } : {}),
  });
}

export type Client = ReturnType<typeof createClient>;

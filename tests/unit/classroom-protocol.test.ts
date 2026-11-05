import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  createUUIDv7,
  deriveRealtimeURL,
  isUUIDv7,
} from "@/app/lib/realtime/classroom-protocol";

beforeEach(() => vi.stubEnv("NEXT_PUBLIC_REALTIME_URL", ""));
afterEach(() => vi.unstubAllEnvs());

describe("classroom realtime protocol helpers", () => {
  it("derives the secure host-only socket URL without exposing a session ID", () => {
    const url = deriveRealtimeURL("https://api.example.test/api/v1");
    expect(url).toBe("wss://api.example.test/ws/v1");
    expect(url).not.toContain("session");
  });

  it("keeps local HTTP development on ws and preserves the API port", () => {
    expect(deriveRealtimeURL("http://127.0.0.1:8080/api/v1")).toBe("ws://127.0.0.1:8080/ws/v1");
  });

  it("uses a separately configured realtime service without exposing tickets in its URL", () => {
    vi.stubEnv("NEXT_PUBLIC_REALTIME_URL", "ws://localhost:8081/ws/v1");
    expect(deriveRealtimeURL("http://localhost:8080/api/v1")).toBe("ws://localhost:8081/ws/v1");
    expect(deriveRealtimeURL("https://api.example.test/api/v1", "wss://api.example.test:8443/ws/v1")).toBe("wss://api.example.test:8443/ws/v1");
  });

  it.each([
    "https://realtime.example.test/ws/v1", "ws://localhost:8081/ws/v2", "ws://localhost:8081/ws/v1/",
    "wss://user:secret@realtime.example.test/ws/v1", "wss://realtime.example.test/ws/v1?ticket=secret",
    "wss://realtime.example.test/ws/v1#secret", "ws://localhost:8081/ws/v1?", "not a URL",
  ])("rejects an unsafe or incompatible realtime endpoint: %s", (url) => {
    expect(() => deriveRealtimeURL("https://api.example.test/api/v1", url)).toThrow(/realtime URL/);
  });

  it("rejects an API URL outside the exact versioned base path", () => {
    expect(() => deriveRealtimeURL("https://api.example.test/v1"))
      .toThrow(/followed by \/api\/v1/);
  });

  it("creates command identifiers that satisfy the checked-in UUIDv7 schema", () => {
    const commandId = createUUIDv7(1_700_000_000_000);
    expect(isUUIDv7(commandId)).toBe(true);
    expect(commandId.slice(14, 15)).toBe("7");
    expect(["8", "9", "a", "b"]).toContain(commandId.slice(19, 20));
  });
});

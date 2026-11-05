import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { useSession } from "@/app/lib/use-session";
const mocks = vi.hoisted(() => ({ get: vi.fn(), clear: vi.fn() }));
vi.mock("@/app/lib/auth-client", () => ({ getSession: mocks.get, clearSession: mocks.clear }));
vi.mock("@/app/lib/demo/mode", () => ({ isDemoMode: () => false }));
function SessionView() { const state = useSession(); return <p>{state.session?.authenticated ? "Signed in" : "Checking session"}</p>; }
afterEach(() => { cleanup(); vi.unstubAllGlobals(); });
it("immediately invalidates authentication on another tab's explicit logout", () => {
  const channels: { onmessage?: (event: { data: string }) => void }[] = [];
  vi.stubGlobal("BroadcastChannel", class { onmessage?: (event: { data: string }) => void; constructor() { channels.push(this); } close() {} });
  mocks.get.mockReturnValue(new Promise(() => undefined));
  render(<SessionView />);
  expect(screen.getByText("Checking session")).toBeInTheDocument();
  expect(mocks.get).toHaveBeenCalledOnce();
  act(() => channels[0].onmessage?.({ data: "signed-out" }));
  expect(mocks.clear).toHaveBeenCalledOnce();
  expect(mocks.get).toHaveBeenCalledOnce();
});

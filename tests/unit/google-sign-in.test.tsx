import { StrictMode } from "react";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import GoogleSignIn from "@/app/components/forms/GoogleSignIn";

const mocks = vi.hoisted(() => ({ configuration: vi.fn(), login: vi.fn(), initialize: vi.fn(), renderButton: vi.fn() }));
vi.mock("@/app/lib/auth-client", () => ({ getGoogleConfiguration: mocks.configuration, signInWithGoogle: mocks.login }));
vi.mock("next/script", () => ({ default: ({ onReady }: { onReady: () => void }) => <button onClick={onReady}>Load Google</button> }));

beforeEach(() => {
  vi.resetAllMocks();
  vi.stubGlobal("google", { accounts: { id: { initialize: mocks.initialize, renderButton: mocks.renderButton } } });
});
afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

describe("Google sign-in", () => {
  it("shares the initial nonce request across Strict Mode effects and renews a failed challenge", async () => {
    mocks.configuration.mockResolvedValueOnce({ enabled: true, clientId: "client", nonce: "first" })
      .mockResolvedValueOnce({ enabled: true, clientId: "client", nonce: "renewed" });
    mocks.login.mockRejectedValue(new Error("Challenge expired. Try again."));
    const success = vi.fn();
    const user = userEvent.setup();
    render(<StrictMode><GoogleSignIn onSuccess={success} /></StrictMode>);
    await user.click(await screen.findByRole("button", { name: "Load Google" }));
    expect(mocks.configuration).toHaveBeenCalledTimes(1);
    expect(mocks.initialize).toHaveBeenLastCalledWith(expect.objectContaining({ nonce: "first", client_id: "client" }));
    const callback = mocks.initialize.mock.calls.at(-1)![0].callback;
    await act(async () => { await callback({ credential: "signed-google-credential" }); });
    expect(mocks.login).toHaveBeenCalledWith("signed-google-credential");
    await waitFor(() => expect(mocks.initialize).toHaveBeenLastCalledWith(expect.objectContaining({ nonce: "renewed" })));
    expect(success).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent("Challenge expired");
  });

  it("does not load Google's script when no client is configured", async () => {
    mocks.configuration.mockResolvedValue({ enabled: false });
    const { container } = render(<GoogleSignIn onSuccess={vi.fn()} />);
    await waitFor(() => expect(container).toBeEmptyDOMElement());
    expect(mocks.initialize).not.toHaveBeenCalled();
  });
});

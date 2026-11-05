"use client";

import { useEffect, useState } from "react";
import { demoAction, fetchWorkspace } from "@/app/lib/demo/client";
import type { DemoWorkspace } from "@/app/lib/demo/data";
import { Panel } from "./WorkspaceUI";

/** Sample billing stays separate from the shared academy and learning contracts. */
export default function DemoBilling() {
  const [workspace, setWorkspace] = useState<DemoWorkspace | null>(null);
  const [checkout, setCheckout] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  useEffect(() => {
    let active = true;
    const load = () => { void fetchWorkspace().then((data) => { if (active) setWorkspace(data); }).catch((cause) => { if (active) setError(cause instanceof Error ? cause.message : "Sample invoices could not be loaded."); }); };
    load(); window.addEventListener("kingsgambit:demo-change", load);
    return () => { active = false; window.removeEventListener("kingsgambit:demo-change", load); };
  }, []);
  async function pay(invoiceId: string, outcome: "success" | "failure") {
    setBusy(true); setError(""); setNotice("");
    try { setWorkspace(await demoAction("payInvoice", { invoiceId, outcome })); setCheckout(""); setNotice("Demo payment succeeded. The invoice and class credits are updated."); }
    catch (cause) { setError(cause instanceof Error ? cause.message : "Payment could not be completed."); }
    finally { setBusy(false); }
  }
  return <div className="academy-workspace-v2"><Panel title="Plans & invoices" action={workspace && <strong>{workspace.family.credits} class credits</strong>}>
    <p>Local sample billing. These payment buttons never charge money.</p>
    {error && <p className="form-message form-message-error" role="alert">{error}</p>}{notice && <p role="status">{notice}</p>}
    <div className="academy-stack">{workspace?.invoices.map((invoice) => <article className="academy-record" key={invoice.id}>
      <span className="workspace-badge">{invoice.id}</span><h3>{invoice.description}</h3><p>{new Intl.NumberFormat(undefined, { style: "currency", currency: invoice.currency }).format(invoice.amountMinor / 100)} · {invoice.status}</p>
      {invoice.status === "due" && <button type="button" onClick={() => setCheckout(invoice.id)}>Review payment →</button>}
      {checkout === invoice.id && invoice.status === "due" && <div className="academy-stack"><p>This sample payment adds {invoice.credits} class credits.</p><div className="academy-actions"><button className="button button-dark" disabled={busy} onClick={() => void pay(invoice.id, "success")}>Simulate successful payment</button><button disabled={busy} onClick={() => void pay(invoice.id, "failure")}>Simulate failed payment</button><button disabled={busy} onClick={() => setCheckout("")}>Close payment</button></div></div>}
    </article>)}</div>
  </Panel></div>;
}

"use client";

import { useState, useSyncExternalStore } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import BrandLogo from "@/app/components/BrandLogo";
import { demoAccounts } from "@/app/lib/demo/data";
import { loginDemo, resetDemo } from "@/app/lib/demo/store";
import { landingPathForRoles, roleLabels } from "@/app/lib/role-routing";
import type { AcademyRole } from "@/app/lib/api/types";
import "./demo.css";

const subscribeReady = () => () => {};
const clientReady = () => true;
const serverReady = () => false;

export default function DemoLauncher() {
  const router = useRouter();
  const ready = useSyncExternalStore(subscribeReady, clientReady, serverReady);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [resetting, setResetting] = useState(false);
  const [busy, setBusy] = useState<AcademyRole | null>(null);

  function enter(role: AcademyRole) {
    try {
      setBusy(role); setError("");
      const session = loginDemo(role);
      router.push(landingPathForRoles(session.user.roles));
    } catch (error) { setBusy(null); setError(error instanceof Error ? error.message : "Could not open the sample account."); }
  }

  function reset() {
    try { resetDemo(); setResetting(false); setNotice("Sample academy restored. The first class is ready to join now."); }
    catch (error) { setError(error instanceof Error ? error.message : "Could not reset sample data."); }
  }

  return (
    <main className="demo-launcher">
      <Link href="/" className="demo-back"><BrandLogo size={42} />← Back to the academy</Link>
      <header><span className="eyebrow">Your local academy</span><h1>Make your next move.</h1><p>Explore the complete learning journey with sample accounts. Choose a role to get started.</p></header>
      <div className="demo-explainer"><strong>Local demo · fictional data</strong><p>Changes stay in this browser. No emails are sent and no money is charged. Open a second tab with another role to try the shared classroom and hand work from student to coach.</p></div>
      {error && <p className="form-message form-message-error" role="alert">{error}</p>}
      {notice && <p className="form-message" role="status">{notice}</p>}
      <div className="demo-accounts">{demoAccounts.map((account, index) => <button key={account.role} className="demo-account" disabled={!ready || busy !== null} onClick={() => enter(account.role)}><span className="demo-account-index">0{index + 1}</span><span className="demo-account-role">{roleLabels[account.role]}</span><strong>{account.name}</strong><span>{account.description}</span><small>{account.email}</small><span className="demo-account-enter">{busy === account.role ? "Opening…" : "Open workspace"}<span aria-hidden="true">↗</span></span></button>)}</div>
      <section className="demo-footer"><div><h2>A fresh board, whenever you need it.</h2><p>Reset restores sample records and gives the first class a new join window. Student access code: <strong>DEMO2700</strong>.</p></div>{resetting ? <div className="demo-reset"><p>Replace all changes in this browser with the original samples?</p><button className="button button-dark" onClick={reset}>Reset sample academy</button><button className="text-link" onClick={() => setResetting(false)}>Keep my changes</button></div> : <button className="button button-outline" onClick={() => setResetting(true)}>Reset demo data</button>}</section>
    </main>
  );
}

"use client";

import { useRef, useState } from "react";
import type { LearningWorkspace } from "@/app/lib/academy-types";
import { downloadLearningFile, uploadLearningFile } from "@/app/lib/learning-files";
import { Field } from "./WorkspaceUI";

export function FileDownload({ id, filename, children }: { id: string; filename: string; children?: React.ReactNode }) {
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  return <span><button type="button" disabled={busy} onClick={async () => {
    setBusy(true); setError("");
    try { await downloadLearningFile(id, filename); }
    catch (cause) { setError(cause instanceof Error ? cause.message : "The file could not be downloaded."); }
    finally { setBusy(false); }
  }}>{busy ? "Downloading…" : children || filename}</button>{error && <span role="alert">{error}</span>}</span>;
}

export function FileUpload({ assignmentId, certificateId, onUploaded, onBusyChange, children }: {
  assignmentId?: string; certificateId?: string; onUploaded?: (workspace: LearningWorkspace) => void; onBusyChange?: (busy: boolean) => void; children?: React.ReactNode;
}) {
	const pending = useRef<{ fingerprint: string; key: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  return <form className="academy-form" onSubmit={async (event) => {
    event.preventDefault(); const element = event.currentTarget;
    const form = new FormData(element); setBusy(true); onBusyChange?.(true); setError(""); setNotice("");
    if (assignmentId) form.set("assignmentId", assignmentId);
    if (certificateId) form.set("certificateId", certificateId);
    const fingerprint = JSON.stringify([...form.entries()].map(([name, value]) => [name, typeof value === "string" ? value : [value.name, value.size, value.lastModified]]));
    if (pending.current?.fingerprint !== fingerprint) pending.current = { fingerprint, key: crypto.randomUUID() };
    try { const result = await uploadLearningFile(form, pending.current.key); onUploaded?.(result.workspace); element.reset(); pending.current = null; setNotice(`${result.file.filename} uploaded and checked.`); }
    catch (cause) { setError(cause instanceof Error ? cause.message : "The file could not be uploaded."); }
    finally { setBusy(false); onBusyChange?.(false); }
  }}>{children}<Field label={certificateId ? "Certificate PDF" : assignmentId ? "Practice evidence" : "Resource file"}><input type="file" name="file" required disabled={busy} accept={certificateId ? ".pdf" : ".pdf,.png,.jpg,.jpeg,.pgn,.fen"} onChange={(event) => {
    const file = event.target.files?.[0];
    const max = file && /\.(png|jpe?g)$/i.test(file.name) ? 10 * 1024 * 1024 : file && /\.(pgn|fen)$/i.test(file.name) ? 512 * 1024 : 20 * 1024 * 1024;
    event.target.setCustomValidity(file && file.size > max ? `Choose a file smaller than ${Math.round(max / 1024)} KB.` : "");
    event.target.reportValidity();
  }} /><small>{certificateId ? "PDF up to 20 MB." : "PDF up to 20 MB, PNG/JPEG up to 10 MB, or PGN/FEN up to 512 KB."} Files are checked before they become available.</small></Field>
    <button type="submit" className="button button-quiet" disabled={busy}>{busy ? "Uploading and checking…" : "Upload file"}</button>
    {error && <p role="alert">{error}</p>}{notice && <p role="status">{notice}</p>}
  </form>;
}

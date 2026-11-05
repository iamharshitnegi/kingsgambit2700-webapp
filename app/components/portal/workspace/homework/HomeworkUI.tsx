"use client";

import { useContext, useEffect, useRef, type ReactNode } from "react";
import { Icon } from "@/app/components/Icon";
import type { Assignment, LearningMetrics } from "@/app/lib/academy-types";
import { exportStudy, newStudy } from "@/app/lib/chess/study";
import TeachingBoard from "../TeachingBoard";
import { FileDownload } from "../LearningFiles";
import { WorkspaceFeedback, download } from "../WorkspaceUI";
import { assignmentStatus, statusLabels } from "./model";
import s from "./homework.module.css";

export function Status({ item }: { item: Assignment }) {
  const status = assignmentStatus(item);
  return <span className={`${s.status} ${s[status] || ""}`}><Icon name={status === "reviewed" ? "checkCircle" : status === "submitted" ? "clock" : status === "returned" ? "repeat" : "homework"} size={13} />{statusLabels[status] || status}</span>;
}

export function HomeworkDialog({ title, subtitle, close, children, wide = false, busy = false }: { title: string; subtitle: string; close: () => void; children: ReactNode; wide?: boolean; busy?: boolean }) {
  const ref = useRef<HTMLDialogElement>(null);
  const feedback = useContext(WorkspaceFeedback);
  const disabled = busy || feedback.busy;
  useEffect(() => {
    const dialog = ref.current;
    const overflow = document.body.style.overflow;
    dialog?.showModal(); document.body.style.overflow = "hidden";
    return () => { dialog?.close(); document.body.style.overflow = overflow; };
  }, []);
  return <dialog ref={ref} className={`${s.homework} ${s.dialog} ${wide ? s.wideDialog : ""}`} aria-label={title} onCancel={event => { event.preventDefault(); if (!disabled) close(); }}>
    <header className={s.dialogHeader}><div><span className={s.eyebrow}>{subtitle}</span><h2>{title}</h2></div><button className={s.iconButton} type="button" disabled={disabled} onClick={close} aria-label="Close dialog"><Icon name="close" size={20} /></button></header>
    {feedback.error && <p className={s.error} role="alert">{feedback.error}</p>}
    {children}
  </dialog>;
}

export function Metrics({ metrics }: { metrics: LearningMetrics }) {
  const items = [["Solved", metrics.completed], ["First try", metrics.firstTry], ["Accuracy", metrics.accuracy === null ? "—" : `${Math.round(metrics.accuracy)}%`], ["Practice time", `${Math.floor(metrics.seconds / 60)}m ${metrics.seconds % 60}s`]];
  return <div><dl className={s.metrics}>{items.map(([name, number]) => <div key={name}><dt>{name}</dt><dd>{number}</dd></div>)}</dl><details className={s.disclosure}><summary>More practice statistics</summary><dl className={s.metrics}>{[["Average per attempted puzzle", metrics.averageSeconds === null ? "—" : `${Math.round(metrics.averageSeconds)}s`], ["In progress", metrics.inProgress], ["Not solved", metrics.notSolved], ["Not attempted", metrics.notAttempted], ["Incorrect tries", metrics.incorrect], ["Hints", metrics.hints], ["Solutions revealed", metrics.reveals]].map(([name, number]) => <div key={name}><dt>{name}</dt><dd>{number}</dd></div>)}</dl></details></div>;
}

export function AssignedMaterials({ assignment }: { assignment: Assignment }) {
  if (!assignment.resources?.length) return null;
  return <section className={s.materials}><h3>Assigned material</h3>{assignment.resources.map(resource => <details className={s.disclosure} key={resource.id}><summary>{resource.title} · version {resource.version}</summary>
    {resource.kind === "pgn" ? <TeachingBoard pgn={resource.content} title={resource.title} /> : resource.kind === "fen" ? <TeachingBoard pgn={exportStudy(newStudy(resource.content))} title={resource.title} /> : resource.kind === "study" ? <p className={s.prose}>{resource.content}</p> : null}
    {resource.fileId ? <FileDownload id={resource.fileId} filename={resource.filename || resource.title} /> : <button className={s.secondaryButton} type="button" onClick={() => download(`${resource.title}.${resource.kind === "study" ? "txt" : resource.kind}`, resource.content)}>Download material <Icon name="external" size={14} /></button>}
  </details>)}</section>;
}

"use client";

import { Icon } from "@/app/components/Icon";

import {
  createContext,
  Children,
  cloneElement,
  isValidElement,
  useContext,
  useEffect,
  useId,
  useRef,
  type FormEvent,
  type ReactNode,
  type ReactElement,
} from "react";

export function Field({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  const generatedId = useId();
  const controls = Children.toArray(children);
  const control = controls.find(
    (child) =>
      isValidElement(child) &&
      typeof child.type === "string" &&
      ["input", "select", "textarea", "output"].includes(child.type),
  ) as ReactElement<{ id?: string }> | undefined;
  const id = control?.props.id || generatedId;
  return (
    <div className="academy-field">
      <label htmlFor={id}>{label}</label>
      {controls.map((child) =>
        child === control ? cloneElement(control, { id }) : child,
      )}
    </div>
  );
}
export function Empty({ children }: { children: ReactNode }) {
  return <div className="academy-empty"><Icon name="book" size={30} className="academy-empty-icon" /><div className="academy-empty-copy">{children}</div></div>;
}
export function Badge({ children }: { children: ReactNode }) {
  return <span className="workspace-badge">{children}</span>;
}
export function Panel({
  title,
  action,
  children,
}: {
  title: string;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="workspace-panel">
      <div className="workspace-section-heading">
        <h2>{title}</h2>
        {action}
      </div>
      {children}
    </section>
  );
}
export const WorkspaceFeedback = createContext({ error: "", busy: false });
export function Modal({
  title,
  close,
  children,
  independent = false,
}: {
  title: string;
  close: () => void;
  children: ReactNode;
  independent?: boolean;
}) {
  const feedback = useContext(WorkspaceFeedback);
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const dialog = ref.current;
    dialog?.showModal();
    return () => dialog?.close();
  }, []);
  return (
    <dialog
      ref={ref}
      className="academy-dialog"
      aria-label={title}
      onCancel={(event) => {
        event.preventDefault();
        close();
      }}
    >
      <div className="academy-dialog-header">
        <h2>{title}</h2>
        <button type="button" onClick={close} aria-label="Close dialog">
          <Icon name="close" size={20} />
        </button>
      </div>
      {!independent && feedback.error && (
        <p className="form-message form-message-error" role="alert">
          {feedback.error}
        </p>
      )}
      <fieldset disabled={!independent && feedback.busy}>{children}</fieldset>
    </dialog>
  );
}
export function formValues(event: FormEvent<HTMLFormElement>) {
  event.preventDefault();
  return new FormData(event.currentTarget);
}
export function value(form: FormData, name: string) {
  return String(form.get(name) || "").trim();
}
export function label(value: string) {
  return value.replaceAll("_", " ");
}
export function time(value: string, timeZone?: string) {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone,
  }).format(new Date(value));
}
export function localDate(value: string, timeZone: string) {
  return new Intl.DateTimeFormat("en-CA", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(new Date(value));
}
export function localInput(value: string, timeZone: string) {
  return new Intl.DateTimeFormat("sv-SE", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  })
    .format(new Date(value))
    .replace(" ", "T");
}
export function download(name: string, content: string, type = "text/plain") {
  const url = URL.createObjectURL(new Blob([content], { type }));
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

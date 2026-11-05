"use client";

export default function SkipLink() {
  return <a className="skip-link" href="#main-content" onClick={(event) => {
    const main = document.querySelector("main");
    if (!main) return;
    event.preventDefault();
    main.tabIndex = -1;
    main.focus();
    main.scrollIntoView({ block: "start" });
  }}>Skip to main content</a>;
}

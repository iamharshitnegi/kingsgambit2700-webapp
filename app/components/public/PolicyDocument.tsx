import sanitizeHtml from "sanitize-html";
import type { Policy } from "@/app/lib/api/types";
import PageHero from "./PageHero";

const policySanitizer = {
  allowedTags: ["h2", "h3", "p", "ul", "ol", "li", "strong", "em", "a", "blockquote"],
  allowedAttributes: { a: ["href"] },
  allowedSchemes: ["http", "https", "mailto"],
};

export default function PolicyDocument({ policy }: { policy: Policy }) {
  const safePolicyHTML = policy.contentHtml
    ? sanitizeHtml(policy.contentHtml, policySanitizer)
    : "";

  return (
    <>
      <PageHero
        eyebrow="Academy policy"
        title={policy.title}
        description={policy.summary || "Read the current published academy policy."}
        aside={(
          <dl className="policy-version">
            <div><dt>Version</dt><dd>{policy.version}</dd></div>
            {policy.effectiveAt && <div><dt>Effective</dt><dd>{new Date(policy.effectiveAt).toLocaleDateString("en", { day: "numeric", month: "long", year: "numeric" })}</dd></div>}
            {policy.reviewStatus && <div><dt>Review status</dt><dd>{policy.reviewStatus.replaceAll("_", " ")}</dd></div>}
          </dl>
        )}
      />
      <article className="policy-document">
        {safePolicyHTML && <div className="policy-html" dangerouslySetInnerHTML={{ __html: safePolicyHTML }} />}
        {policy.body && policy.body.split(/\n\n+/).map((paragraph) => <p key={paragraph.slice(0, 48)}>{paragraph}</p>)}
        {policy.sections?.map((section) => (
          <section key={section.heading}>
            <h2>{section.heading}</h2>
            {section.body.split(/\n\n+/).map((paragraph) => <p key={paragraph.slice(0, 48)}>{paragraph}</p>)}
          </section>
        ))}
      </article>
    </>
  );
}

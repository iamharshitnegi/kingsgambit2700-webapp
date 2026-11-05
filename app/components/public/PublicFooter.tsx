import Link from "next/link";
import BrandLogo from "@/app/components/BrandLogo";
import type { OrganizationSettings, PolicySummary } from "@/app/lib/api/types";

export default function PublicFooter({
  organization,
  policies,
}: {
  organization: OrganizationSettings;
  policies: PolicySummary[];
}) {
  const whatsappHref = `https://wa.me/${organization.whatsappNumber.replace(/\D/g, "")}`;

  return (
    <footer className="site-footer">
      <div className="footer-lead">
        <div className="footer-chess-tile" aria-hidden="true">♘</div>
        <div>
          <span className="eyebrow eyebrow-light">Your next move</span>
          <h2>Great things start with a first move.</h2>
        </div>
        <Link href="/free-trial" className="button button-cream">Book a free trial <span aria-hidden="true">↗</span></Link>
      </div>
      <div className="footer-grid">
        <div className="footer-about">
          <Link href="/" className="brand-lockup brand-lockup-inverse" aria-label={`${organization.name} home`}>
            <BrandLogo size={48} />
            <span translate="no">KingsGambit <em>2700</em><small>The chess academy</small></span>
          </Link>
          <p>Live online chess coaching, structured around clear learning goals and careful human guidance.</p>
        </div>
        <div>
          <h3>Explore</h3>
          <ul>
            <li><Link href="/programs">Programs</Link></li>
            <li><Link href="/curriculum">Curriculum</Link></li>
            <li><Link href="/coaches">Coaches</Link></li>
            <li><Link href="/pricing">Pricing</Link></li>
            <li><Link href="/blog">Journal</Link></li>
          </ul>
        </div>
        <div>
          <h3>Policies</h3>
          <ul>
            {policies.map((policy) => (
              <li key={policy.slug}><Link href={`/policies/${policy.slug}`}>{policy.title}</Link></li>
            ))}
          </ul>
        </div>
        <div>
          <h3>Contact</h3>
          <ul>
            <li><a href={`mailto:${organization.contactEmail}`}>{organization.contactEmail}</a></li>
            <li><a href={whatsappHref} target="_blank" rel="noreferrer">WhatsApp {organization.whatsappDisplay}</a></li>
            {organization.instagramUrl && <li><a href={organization.instagramUrl} target="_blank" rel="noreferrer">Instagram</a></li>}
          </ul>
        </div>
      </div>
      <div className="footer-bottom">
        <span>© {new Date().getFullYear()} {organization.name}</span>
        <span>Learn well. Think ahead. Enjoy the game.</span>
      </div>
    </footer>
  );
}

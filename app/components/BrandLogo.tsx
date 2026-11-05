import Image from "next/image";

/** The original academy artwork, framed in a circle without changing its source. */
export default function BrandLogo({ size = 44, priority = false, className = "" }: {
  size?: number;
  priority?: boolean;
  className?: string;
}) {
  return (
    <span className={`kg-brand-logo ${className}`} style={{ width: size, height: size }}>
      <Image src="/logo.png" alt="" width={size} height={size} preload={priority}
        style={{ width: "100%", height: "100%", objectFit: "contain", padding: "7%" }} />
    </span>
  );
}

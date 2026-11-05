export type AcademyRole =
  | "guardian"
  | "student"
  | "coach"
  | "academic_manager"
  | "operations_admin"
  | "finance_admin"
  | "super_admin";

export type AgeBand = "4_7" | "8_12" | "13_17" | "18_plus";
export type TrialFormat = "group" | "one_to_one";

export type SessionUser = {
  id: string;
  email?: string;
  displayName?: string;
  roles: AcademyRole[];
};

export type SessionResponse = {
  authenticated: boolean;
  restoreAvailable?: boolean;
  mfaRequired?: boolean;
  csrfToken?: string;
  user?: SessionUser;
};

export type MFAStatus = {
  required: boolean;
  enrolled: boolean;
  enrollmentPending: boolean;
  verifiedThisSession: boolean;
  challengeExpiresAt?: string;
  csrfToken: string;
};

export type TOTPEnrollment = {
  secret: string;
  otpauthUri: string;
  expiresAt: string;
};

export type MFAEnrollmentCompletion = SessionResponse & {
  authenticated: true;
  csrfToken: string;
  user: SessionUser;
  recoveryCodes: string[];
};

export type MFAChallenge =
  | { code: string; recoveryCode?: never }
  | { code?: never; recoveryCode: string };

export type ProblemDetail = {
  type?: string;
  title: string;
  status: number;
  detail?: string;
  code?: string;
  requestId?: string;
  errors?: Array<{ field: string; message: string }>;
};

export type OrganizationSettings = {
  name: string;
  legalName?: string;
  contactEmail: string;
  whatsappNumber: string;
  whatsappDisplay: string;
  primaryTimeZone: string;
  siteUrl: string;
  instagramUrl?: string;
  facebookUrl?: string;
  youtubeUrl?: string;
  verifiedStats?: Array<{ label: string; value: number; suffix?: string }>;
};

export type Program = {
  id: string;
  slug: string;
  name: string;
  summary: string;
  formats: string[];
  ageBands: string[];
  levelSlugs: string[];
  capacity?: number;
  durationMinutes?: number;
  startingPrice?: Money & { interval?: string };
};

export type Coach = {
  id: string;
  slug: string;
  displayName: string;
  bio: string;
  languages: string[];
  title?: string;
  rating?: number;
  ratingSource?: string;
  photoUrl?: string;
  verifiedAt?: string;
};

export type CurriculumLevel = {
  slug: string;
  name: string;
  summary: string;
  ageGuidance?: string;
  ratingGuidance?: string;
  outcomes: string[];
};

export type CurriculumTrack = {
  slug: string;
  name: string;
  summary: string;
  levels: CurriculumLevel[];
};

export type Curriculum = {
  version: string;
  tracks: CurriculumTrack[];
};

export type Money = {
  amountMinor: number;
  currency: string;
};

export type PricingPlan = {
  planId: string;
  programSlug: string;
  name: string;
  format: string;
  billingModel: string;
  amountMinor: number;
  currency?: string;
  interval?: string;
  classQuantity?: number;
  capacity?: number;
  taxInclusive: boolean;
  features: string[];
};

export type PricingCatalog = {
  currency: string;
  items: PricingPlan[];
};

export type PolicySummary = {
  slug: string;
  title: string;
  summary?: string;
  version: string;
  effectiveAt?: string;
  reviewStatus?: string;
};

export type Policy = PolicySummary & {
  body?: string;
  contentHtml?: string;
  sections?: Array<{ heading: string; body: string }>;
};

export type LeadInput = {
  guardianName: string;
  email?: string;
  phone?: string;
  studentAgeBand: AgeBand;
  experience?: string;
  rating?: number;
  country: string;
  timeZone: string;
  preferredFormat?: TrialFormat;
  preferredTimes?: string[];
  contactConsent: boolean;
  marketingConsent?: boolean;
  source?: string;
  campaign?: string;
  medium?: string;
  term?: string;
  content?: string;
  landingPage?: string;
  referrer?: string;
};

export type LeadResponse = {
  id: string;
  status: "new" | string;
  createdAt: string;
};

export type TrialSlot = {
  id: string;
  startsAt: string;
  endsAt: string;
  format: TrialFormat;
  availableSeats?: number;
  coachDisplayName?: string;
};

export type TrialBookingInput = {
  slotId: string;
  leadId: string;
  guardianName: string;
  email: string;
  phone?: string;
  studentName?: string;
  studentAgeBand: AgeBand;
  timeZone: string;
  contactConsent: true;
};

export type TrialBookingResponse = {
  id: string;
  status: "confirmed" | string;
  startsAt: string;
  endsAt: string;
  timeZone: string;
};

export type PortalOverview = {
  heading?: string;
  notice?: string;
  nextClass?: {
    id: string;
    title: string;
    startsAt: string;
    endsAt: string;
    joinable: boolean;
  };
  tasks?: Array<{ id: string; title: string; dueAt?: string; status: string }>;
};

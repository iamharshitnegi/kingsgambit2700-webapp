import type {
  Curriculum,
  OrganizationSettings,
  Policy,
  PolicySummary,
  Program,
} from "./api/types";

export const fallbackOrganization: OrganizationSettings = {
  name: "KingsGambit2700",
  contactEmail: "myemailid2700@gmail.com",
  whatsappNumber: "+918679762608",
  whatsappDisplay: "+91 86797 62608",
  primaryTimeZone: "Asia/Kolkata",
  siteUrl: "https://kingsgambit2700.in",
  instagramUrl: "https://www.instagram.com/kingsgambit2700/",
};

export const fallbackPrograms: Program[] = [
  {
    id: "small-group",
    slug: "small-group",
    name: "Small-group academy",
    summary:
      "A structured learning path with live instruction, purposeful practice, and room for every student to participate.",
    formats: ["Small group"],
    ageBands: ["School-age learners", "Teens"],
    levelSlugs: ["foundation", "development", "performance"],
    capacity: 6,
  },
  {
    id: "one-to-one",
    slug: "one-to-one",
    name: "One-to-one coaching",
    summary:
      "Individual sessions shaped around the learner’s current games, goals, schedule, and tournament preparation needs.",
    formats: ["One to one"],
    ageBands: ["Children", "Teens", "Adults"],
    levelSlugs: ["foundation", "development", "performance"],
    capacity: 1,
  },
  {
    id: "assessment",
    slug: "trial-assessment",
    name: "Trial and level assessment",
    summary:
      "A first conversation and practical chess assessment to identify a suitable starting point before enrolment.",
    formats: ["Online assessment"],
    ageBands: ["Children", "Teens", "Adults"],
    levelSlugs: [],
    capacity: 1,
  },
];

export const fallbackCurriculum: Curriculum = {
  version: "public-framework-v1",
  tracks: [
    {
      slug: "foundation",
      name: "Foundation",
      summary:
        "Build board confidence, sound habits, and the ability to finish a complete game thoughtfully.",
      levels: [
        {
          slug: "foundation-1",
          name: "Board foundations",
          summary: "Rules, piece coordination, checks, and safe decision-making.",
          ageGuidance: "Placement is based on readiness, not age alone.",
          outcomes: [
            "Play legal moves and explain basic piece value",
            "Recognise check, mate, and simple threats",
            "Use an opening routine without memorising long lines",
          ],
        },
        {
          slug: "foundation-2",
          name: "Tactical foundations",
          summary: "Core mating patterns, captures, threats, and calculation habits.",
          outcomes: [
            "Spot common one- and two-move tactics",
            "Complete basic king-and-piece checkmates",
            "Review a game using clear questions",
          ],
        },
      ],
    },
    {
      slug: "development",
      name: "Development",
      summary:
        "Connect tactics with planning, calculation, endgames, and reflective game analysis.",
      levels: [
        {
          slug: "development-1",
          name: "Calculation and planning",
          summary: "Candidate moves, forcing sequences, and practical positional choices.",
          outcomes: [
            "Calculate short forcing variations accurately",
            "Identify plans from pawn structure and piece activity",
            "Convert fundamental king-and-pawn endings",
          ],
        },
        {
          slug: "development-2",
          name: "Competitive habits",
          summary: "Clock management, preparation, resilience, and deeper game review.",
          outcomes: [
            "Prepare a practical opening repertoire",
            "Manage time across a full tournament game",
            "Turn mistakes into a focused practice plan",
          ],
        },
      ],
    },
    {
      slug: "performance",
      name: "Performance",
      summary:
        "Individualised preparation for ambitious tournament players and advanced competitors.",
      levels: [
        {
          slug: "performance-1",
          name: "Advanced performance",
          summary: "Deep calculation, strategic imbalances, and tournament preparation.",
          outcomes: [
            "Compare complex candidate moves with discipline",
            "Build opponent- and event-specific preparation",
            "Evaluate progress using games and coach evidence",
          ],
        },
      ],
    },
  ],
};

export const fallbackPolicySummaries: PolicySummary[] = [
  { slug: "privacy", title: "Privacy notice", version: "Pending legal review", reviewStatus: "draft" },
  { slug: "terms", title: "Terms of service", version: "Pending legal review", reviewStatus: "draft" },
  { slug: "cancellation-refund", title: "Cancellation and refund", version: "Pending legal review", reviewStatus: "draft" },
  { slug: "child-safety", title: "Child safety", version: "Pending safeguarding review", reviewStatus: "draft" },
  { slug: "recording", title: "Class recording", version: "Pending legal review", reviewStatus: "draft" },
];

export const fallbackPolicies: Record<string, Policy> = Object.fromEntries(
  fallbackPolicySummaries.map((policy) => [
    policy.slug,
    {
      ...policy,
      summary:
        "The reviewed policy text is not available from the academy service right now. Please contact the academy before relying on this page.",
      sections: [
        {
          heading: "Status",
          body: "This public fallback is intentionally not presented as an approved legal policy. The effective, versioned document will appear here when published by the academy.",
        },
        {
          heading: "Questions",
          body: "Contact the academy for the current document, or for help with consent, cancellation, refunds, safeguarding, recordings, or personal-data requests.",
        },
      ],
    },
  ]),
);

export type CityContent = {
  name: string;
  timeZone: string;
  timeZoneLabel: string;
  headline: string;
  subheadline: string;
  description: string;
};

export const CITY_DATA: Record<string, CityContent> = {
  dubai: {
    name: "Dubai",
    timeZone: "Asia/Dubai",
    timeZoneLabel: "Gulf Standard Time",
    headline: "Online chess coaching for learners in Dubai",
    subheadline: "Live assessment and coaching paths with times shown clearly in your local time zone.",
    description: "Explore the same academy curriculum and coach-matching process online, with published trial availability converted for Dubai.",
  },
  bengaluru: {
    name: "Bengaluru",
    timeZone: "Asia/Kolkata",
    timeZoneLabel: "India Standard Time",
    headline: "Online chess coaching for learners in Bengaluru",
    subheadline: "A structured path from first assessment to purposeful live learning.",
    description: "Explore small-group and one-to-one paths, then begin with a live level assessment shown in India Standard Time.",
  },
  delhi: {
    name: "Delhi",
    timeZone: "Asia/Kolkata",
    timeZoneLabel: "India Standard Time",
    headline: "Online chess coaching for learners in Delhi NCR",
    subheadline: "Live instruction, a visible level framework, and a thoughtful first assessment.",
    description: "Review the academy’s public curriculum and coaching formats before choosing a published assessment time in India Standard Time.",
  },
  singapore: {
    name: "Singapore",
    timeZone: "Asia/Singapore",
    timeZoneLabel: "Singapore Standard Time",
    headline: "Online chess coaching for learners in Singapore",
    subheadline: "A clear curriculum and online assessment process, with local-time scheduling.",
    description: "Explore live small-group and one-to-one paths, with published trial availability converted for Singapore.",
  },
  london: {
    name: "London",
    timeZone: "Europe/London",
    timeZoneLabel: "UK local time",
    headline: "Online chess coaching for learners in London",
    subheadline: "Structured online learning with times displayed in the learner’s own time zone.",
    description: "Start with a live assessment, review the appropriate curriculum level, and discuss a suitable online format for the learner.",
  },
};

"use client";

import React, { useState } from 'react';
import Link from 'next/link';

type SubLevel = {
  id: string;
  title: string;
  link: string;
  points: string[];
};

type CourseDivision = {
  id: string;
  title: string;
  badge?: string;
  ageGroup: string;
  iconClass: string;
  iconBg: string;
  iconColor: string;
  borderColor: string;
  textColor: string;
  isPopular?: boolean;
  levels: SubLevel[];
};

const courseDivisions: CourseDivision[] = [
  {
    id: 'beginner',
    title: 'Beginner',
    ageGroup: 'Age 5-10 • No prior experience',
    iconClass: 'fa-solid fa-chess-pawn',
    iconBg: 'bg-green-100',
    iconColor: 'text-green-600',
    borderColor: 'border-green-500',
    textColor: 'text-green-600',
    levels: [
      {
        id: 'beg-1',
        title: 'Level 1',
        link: '#',
        points: ['Learn piece movements', 'Understand basic rules', 'Play your first game']
      },
      {
        id: 'beg-2',
        title: 'Level 2',
        link: '#',
        points: ['Basic checkmates (Ladder, Queen)', 'Value of pieces', 'Board vision basics']
      },
      {
        id: 'beg-3',
        title: 'Level 3',
        link: '#',
        points: ['Opening principles', 'Simple tactics (Forks, Pins)', 'Playing full games with clock']
      }
    ]
  },
  {
    id: 'intermediate',
    title: 'Intermediate',
    ageGroup: 'Rating < 1200 • Tactics focus',
    iconClass: 'fa-solid fa-chess-knight',
    iconBg: 'bg-amber-100',
    iconColor: 'text-brand-accent',
    borderColor: 'border-brand-accent',
    textColor: 'text-brand-accent',
    isPopular: true,
    levels: [
      {
        id: 'int-1',
        title: 'Level 1',
        link: '#',
        points: ['Advanced opening principles', 'Identifying tactical motifs', 'Basic endgames (Pawn & King)']
      },
      {
        id: 'int-2',
        title: 'Level 2',
        link: '#',
        points: ['Complex combinations', 'Mating patterns', 'Evaluating pawn structures']
      },
      {
        id: 'int-3',
        title: 'Level 3',
        link: '#',
        points: ['Candidate moves calculation', 'Positional play basics', 'Rook endgames']
      }
    ]
  },
  {
    id: 'advanced',
    title: 'Advanced',
    ageGroup: 'Rating 1400+ • Strategy focus',
    iconClass: 'fa-solid fa-chess-queen',
    iconBg: 'bg-red-100',
    iconColor: 'text-red-600',
    borderColor: 'border-red-500',
    textColor: 'text-red-600',
    levels: [
      {
        id: 'adv-1',
        title: 'Level 1',
        link: '#',
        points: ['Deep opening repertoire', 'Prophylactic thinking', 'Advanced pawn structures']
      },
      {
        id: 'adv-2',
        title: 'Level 2',
        link: '#',
        points: ['Dynamic vs Static imbalances', 'Complex calculation training', 'Attacking the King']
      },
      {
        id: 'adv-3',
        title: 'Level 3',
        link: '#',
        points: ['Master level game analysis', 'Tournament preparation', 'Psychology of competition']
      }
    ]
  }
];

export default function CourseLevels() {
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const toggleLevel = (id: string) => {
    setExpandedId(prev => (prev === id ? null : id));
  };

  return (
    <div className="flex flex-col w-full max-w-4xl mx-auto border-t border-brand-border">
      {courseDivisions.map((division, index) => (
        <div key={division.id} className="border-b border-brand-border">
          <div 
            onClick={() => toggleLevel(division.id)} 
            className="flex items-center justify-between py-6 px-2 cursor-pointer hover:bg-black/5 transition"
          >
            <div className="flex items-baseline gap-6 md:gap-8">
              <span className="font-serif text-3xl md:text-4xl text-brand-goldLight">
                0{index + 1}
              </span>
              <div>
                <div className="flex items-center gap-3">
                  <span className="text-brand-charcoal text-base md:text-lg font-bold">{division.title}</span>
                  {division.isPopular && <span className="px-2 py-0.5 bg-brand-gold/10 text-brand-gold text-[10px] font-bold uppercase tracking-wider rounded-sm">Recommended</span>}
                </div>
                <div className="text-stone-500 text-sm md:text-base mt-1">{division.ageGroup}</div>
              </div>
            </div>
            <div className="flex items-center gap-6">
              <Link href="/contact" className="hidden sm:block text-brand-gold text-sm font-semibold hover:underline" onClick={(e) => e.stopPropagation()}>
                Enquire
              </Link>
              <i className={`fa-solid fa-chevron-down text-brand-goldLight text-base transition-transform duration-300 ${expandedId === division.id ? 'rotate-180' : ''}`}></i>
            </div>
          </div>
          
          <div className={`overflow-hidden transition-all duration-300 ease-in-out ${expandedId === division.id ? 'max-h-96 opacity-100 pb-6' : 'max-h-0 opacity-0'}`}>
            <div className="pl-16 md:pl-20 pr-4 space-y-5">
              {division.levels.map((level) => (
                <div key={level.id}>
                  <h4 className="text-sm font-bold text-brand-charcoal mb-2">{level.title}</h4>
                  <ul className="list-disc pl-5 space-y-1.5 text-stone-600 text-sm md:text-base">
                    {level.points.map((point, i) => (
                      <li key={i}>{point}</li>
                    ))}
                  </ul>
                </div>
              ))}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

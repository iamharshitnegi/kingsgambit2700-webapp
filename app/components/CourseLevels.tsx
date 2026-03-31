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
    <div className="grid md:grid-cols-3 gap-8">
      {courseDivisions.map((division) => (
        <div key={division.id} className={`bg-white p-8 rounded-xl shadow-md hover:shadow-xl transition border-t-4 ${division.borderColor} ${division.isPopular ? 'transform md:-translate-y-4 relative' : ''}`}>
          {division.isPopular && (
            <div className="absolute top-0 right-0 bg-brand-accent text-white text-xs px-3 py-1 rounded-bl-lg font-bold">
              POPULAR
            </div>
          )}
          
          <div className={`w-14 h-14 ${division.iconBg} rounded-full flex items-center justify-center mb-6 ${division.iconColor} text-2xl`}>
            <i className={division.iconClass}></i>
          </div>
          
          <h3 className="text-xl font-bold mb-2">{division.title}</h3>
          <p className="text-sm text-gray-500 mb-4">{division.ageGroup}</p>
          
          {/* Sub-Levels Accordion */}
          <div className="mb-6 space-y-2">
            {division.levels.map((level) => (
              <div key={level.id} className="border border-slate-100 rounded-lg overflow-hidden shadow-sm transition-all duration-300">
                <div className="flex items-center justify-between bg-slate-50 hover:bg-slate-100 transition pr-2">
                  {/* Clickable Title separate from toggle */}
                  <Link href={level.link} className={`flex-grow py-3 px-4 text-sm font-semibold hover:underline ${division.textColor}`}>
                    {level.title}
                  </Link>

                  {/* Accessible touch-target minimum 44x44 for dropdown toggle */}
                  <button 
                    onClick={() => toggleLevel(level.id)}
                    className="w-11 h-11 flex items-center justify-center text-slate-400 hover:text-slate-600 rounded-lg hover:bg-slate-200 transition focus:outline-none focus:ring-2 focus:ring-slate-300"
                    aria-expanded={expandedId === level.id}
                    aria-label={`Toggle details for ${level.title}`}
                  >
                    <i className={`fa-solid fa-chevron-down text-sm transition-transform duration-300 ${expandedId === level.id ? 'rotate-180' : ''}`}></i>
                  </button>
                </div>
                
                {/* Accordion Content */}
                <div 
                  className={`overflow-hidden transition-all duration-300 ease-in-out ${
                    expandedId === level.id ? 'max-h-40 opacity-100' : 'max-h-0 opacity-0'
                  }`}
                >
                  <div className="p-4 bg-white border-t border-slate-100">
                    <ul className="text-sm text-gray-600 list-disc pl-4 space-y-1">
                      {level.points.map((point, i) => (
                        <li key={i}>{point}</li>
                      ))}
                    </ul>
                  </div>
                </div>
              </div>
            ))}
          </div>
          
          <Link href="/contact" className={`font-bold hover:underline ${division.textColor}`}>
            Enquire Now →
          </Link>
        </div>
      ))}
    </div>
  );
}

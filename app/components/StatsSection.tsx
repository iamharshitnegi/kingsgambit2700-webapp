'use client';

import { useEffect, useRef, useState } from 'react';
import CountUp from 'react-countup';

interface StatItem {
  value: number;
  suffix?: string;
  label: string;
}

interface StatsSectionProps {
  stats?: StatItem[];
}

const defaultStats: StatItem[] = [
  {
    value: 100,
    suffix: '+',
    label: 'Chess Students Trained'
  },
  {
    value: 2000,
    suffix: '+',
    label: 'Hours Classes'
  },
  {
    value: 10,
    suffix: '+',
    label: 'Countries Served'
  },
  {
    value: 50,
    suffix: '+',
    label: 'FIDE Rated Players Trained'
  }
];

export default function StatsSection({ stats = defaultStats }: StatsSectionProps) {
  const [isVisible, setIsVisible] = useState(false);
  const sectionRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting && !isVisible) {
          setIsVisible(true);
        }
      },
      {
        threshold: 0.1,
        rootMargin: '0px 0px -100px 0px'
      }
    );

    if (sectionRef.current) {
      observer.observe(sectionRef.current);
    }

    return () => {
      if (sectionRef.current) {
        observer.unobserve(sectionRef.current);
      }
    };
  }, [isVisible]);

  return (
    <section ref={sectionRef} className="py-12 md:py-16 flex justify-center w-full bg-brand-cream border-y border-brand-border">
      <div className="flex flex-wrap justify-center items-center gap-y-8 max-w-5xl mx-auto px-6">
        {stats.map((stat, index) => (
          <div key={index} className="flex items-center">
            <div className="text-center px-6 md:px-10">
              <div className="font-serif text-[22px] text-brand-charcoal">
                {isVisible ? (
                  <CountUp end={stat.value} duration={2} separator="," suffix={stat.suffix} />
                ) : (
                  `0${stat.suffix || ''}`
                )}
              </div>
              <div className="text-stone-400 text-[11px] uppercase tracking-wider mt-1">
                {stat.label}
              </div>
            </div>
            {index < stats.length - 1 && (
              <div className="hidden md:block w-[0.5px] height-[32px] min-h-[32px] bg-brand-border"></div>
            )}
          </div>
        ))}
      </div>
    </section>
  );
}

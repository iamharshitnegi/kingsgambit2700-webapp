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
    value: 30000,
    suffix: '+',
    label: 'Chess Students Trained'
  },
  {
    value: 517000,
    suffix: '+',
    label: 'Chess Classes Delivered'
  },
  {
    value: 45,
    suffix: '+',
    label: 'Countries Served'
  },
  {
    value: 250,
    suffix: '+',
    label: 'Tournaments Conducted'
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
    <section 
      ref={sectionRef}
      className="py-16 md:py-24 relative z-20"
    >
      <div className="max-w-7xl mx-auto px-6 lg:px-8">
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-8">
          {stats.map((stat, index) => (
            <div 
              key={index}
              className="text-center group transform transition-all duration-300 hover:scale-105"
            >
              <div className="mb-3">
                {isVisible ? (
                  <div className="text-4xl md:text-5xl lg:text-6xl font-bold text-white drop-shadow-lg">
                    <CountUp
                      end={stat.value}
                      duration={2}
                      separator=","
                      suffix={stat.suffix}
                    />
                  </div>
                ) : (
                  <div className="text-4xl md:text-5xl lg:text-6xl font-bold text-white drop-shadow-lg">
                    0{stat.suffix}
                  </div>
                )}
              </div>
              <p className="text-sm md:text-base text-white font-medium leading-tight drop-shadow-md">
                {stat.label}
              </p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

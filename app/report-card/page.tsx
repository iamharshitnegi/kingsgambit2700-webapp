"use client";

import React, { useState } from "react";
import {
  TrendingUp,
  BrainCircuit,
  Target,
  Trophy,
  Loader2,
  AlertCircle
} from "lucide-react";

// Mock Input Data
const STUDENT_MOCK_DATA = {
  student_name: "Aarav",
  level: 3,
  games_played: 12,
  win_rate: "65%",
  avg_accuracy: 74.5,
  prev_week_accuracy: 68.2,
  puzzles_solved: 45,
  puzzle_avg_time: "42s",
  tactical_weakness: ["back_rank_mate", "unprotected_pieces"],
  tactical_strengths: ["forks", "opening_principles"]
};

// Response Type
type ReportCardResponse = {
  headline_summary: string;
  detailed_analysis: string;
  recommended_drill: string;
  achievement_badge: string;
};

export default function ReportCardPage() {
  const [report, setReport] = useState<ReportCardResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const calculateDelta = (current: number, prev: number) => {
    return (current - prev).toFixed(1);
  };

  const handleGenerateReport = async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch("/api/report-card", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(STUDENT_MOCK_DATA),
      });

      if (!response.ok) {
        throw new Error("Failed to generate report");
      }

      const data: ReportCardResponse = await response.json();
      setReport(data);
    } catch (err: any) {
      setError(err.message || "An unexpected error occurred.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-slate-900 text-slate-200 p-8 font-sans relative overflow-hidden flex flex-col items-center">
      {/* Watermark */}
      <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 text-[12vw] font-bold text-slate-800/20 whitespace-nowrap pointer-events-none select-none z-0">
        KingsGambit2700
      </div>

      <div className="z-10 w-full max-w-4xl relative">
        <header className="mb-10 text-center">
          <h1 className="text-4xl md:text-5xl font-extrabold text-transparent bg-clip-text bg-gradient-to-r from-cyan-400 to-emerald-400 mb-4">
            AI Parent Report Card
          </h1>
          <p className="text-slate-400 text-lg">
            Weekly Performance Review for {STUDENT_MOCK_DATA.student_name}
          </p>
        </header>

        {!report && !loading && (
          <div className="flex flex-col items-center justify-center py-20">
            <div className="bg-slate-800/50 p-8 rounded-2xl border border-slate-700/50 flex flex-col items-center text-center shadow-2xl backdrop-blur-sm max-w-md w-full">
              <BrainCircuit className="w-16 h-16 text-cyan-400 mb-6" />
              <h2 className="text-2xl font-bold text-slate-200 mb-4">Ready to Analyze</h2>
              <p className="text-slate-400 mb-8">
                Compile {STUDENT_MOCK_DATA.student_name}'s {STUDENT_MOCK_DATA.games_played} games and {STUDENT_MOCK_DATA.puzzles_solved} puzzles this week into an actionable AI report.
              </p>
              <button
                onClick={handleGenerateReport}
                className="bg-cyan-500 hover:bg-cyan-400 text-slate-900 font-bold py-3 px-8 rounded-xl transition-all hover:scale-105 active:scale-95 shadow-[0_0_20px_rgba(34,211,238,0.3)] hover:shadow-[0_0_30px_rgba(34,211,238,0.5)] flex items-center gap-2"
              >
                Generate Insights
              </button>
            </div>
          </div>
        )}

        {loading && (
          <div className="flex flex-col items-center justify-center py-32 space-y-4">
            <Loader2 className="w-12 h-12 text-cyan-400 animate-spin" />
            <p className="text-cyan-400 font-medium animate-pulse text-lg">
              KingsGambit Analyst is processing data...
            </p>
          </div>
        )}

        {error && (
          <div className="bg-red-900/20 border border-red-500/30 text-red-400 p-6 rounded-xl flex items-center gap-4 max-w-lg mx-auto">
            <AlertCircle className="w-6 h-6 shrink-0" />
            <p>{error}</p>
          </div>
        )}

        {report && !loading && (
          <div className="grid grid-cols-1 md:grid-cols-12 gap-6 animate-in fade-in slide-in-from-bottom-4 duration-700">
            {/* Top Bar: Headline & Badge */}
            <div className="col-span-1 md:col-span-12 bg-gradient-to-r from-slate-800 to-slate-800/50 border border-slate-700 p-6 rounded-2xl flex flex-col md:flex-row items-center justify-between shadow-lg">
              <div>
                <h2 className="text-2xl font-bold focus:outline-none focus-visible:ring text-slate-100">
                  {report.headline_summary}
                </h2>
              </div>
              <div className="mt-4 md:mt-0 flex items-center gap-3 bg-cyan-950/40 border border-cyan-800/50 py-2 px-5 rounded-full">
                <Trophy className="w-5 h-5 text-cyan-400" />
                <span className="text-cyan-300 font-semibold tracking-wide">
                  {report.achievement_badge}
                </span>
              </div>
            </div>

            {/* Progress Card */}
            <div className="col-span-1 md:col-span-4 bg-slate-800/80 border border-slate-700 p-6 rounded-2xl flex flex-col shadow-lg backdrop-blur-sm relative overflow-hidden">
              <div className="absolute top-0 right-0 w-32 h-32 bg-cyan-500/10 rounded-full blur-3xl -mr-10 -mt-10"></div>
              <div className="flex items-center gap-3 mb-6">
                <TrendingUp className="w-6 h-6 text-emerald-400" />
                <h3 className="text-lg font-semibold text-slate-300">Progress Delta</h3>
              </div>

              <div className="flex-1 flex flex-col justify-center items-center text-center">
                <div className="flex items-baseline gap-1">
                  <span className="text-5xl font-black text-emerald-400">
                    +{calculateDelta(STUDENT_MOCK_DATA.avg_accuracy, STUDENT_MOCK_DATA.prev_week_accuracy)}
                  </span>
                  <span className="text-xl font-bold text-emerald-500/70">%</span>
                </div>
                <p className="text-slate-400 mt-2 text-sm uppercase tracking-widest font-medium">
                  Accuracy Jump
                </p>
              </div>

              <div className="mt-6 pt-4 border-t border-slate-700/50 grid grid-cols-2 gap-4">
                <div className="text-center">
                  <p className="text-slate-500 text-xs uppercase mb-1">Win Rate</p>
                  <p className="text-slate-200 font-bold">{STUDENT_MOCK_DATA.win_rate}</p>
                </div>
                <div className="text-center">
                  <p className="text-slate-500 text-xs uppercase mb-1">Puzzles</p>
                  <p className="text-slate-200 font-bold">{STUDENT_MOCK_DATA.puzzles_solved}</p>
                </div>
              </div>
            </div>

            {/* Coach's Comment Box */}
            <div className="col-span-1 md:col-span-8 bg-slate-800/80 border border-slate-700 p-6 md:p-8 rounded-2xl shadow-lg backdrop-blur-sm flex flex-col relative overflow-hidden">
              <div className="absolute top-0 left-0 w-40 h-40 bg-indigo-500/10 rounded-full blur-3xl -ml-20 -mt-20"></div>
              <div className="flex items-center gap-3 mb-6">
                <BrainCircuit className="w-6 h-6 text-indigo-400" />
                <h3 className="text-lg font-semibold text-slate-300">Coach's Analysis</h3>
              </div>
              <div className="prose prose-invert max-w-none">
                <p className="text-slate-300 leading-relaxed text-lg font-light">
                  {report.detailed_analysis}
                </p>
              </div>
            </div>

            {/* Next Steps Section */}
            <div className="col-span-1 md:col-span-12 bg-gradient-to-br from-cyan-950/40 to-slate-900 border border-cyan-900/50 p-6 md:p-8 rounded-2xl shadow-xl flex items-start gap-5">
              <div className="bg-cyan-900/50 p-3 rounded-xl shrink-0 mt-1">
                <Target className="w-7 h-7 text-cyan-400" />
              </div>
              <div>
                <h3 className="text-xl font-bold text-cyan-300 mb-2">Recommended Next step</h3>
                <p className="text-slate-300 leading-relaxed text-lg">
                  {report.recommended_drill}
                </p>
              </div>
            </div>

            <div className="col-span-1 md:col-span-12 flex justify-center mt-6">
              <button
                onClick={() => setReport(null)}
                className="text-slate-500 hover:text-slate-300 text-sm font-medium transition-colors"
              >
                Reset Dashboard
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

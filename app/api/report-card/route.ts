import { NextResponse } from "next/server";
import { OpenRouter } from "@openrouter/sdk";
import { z } from "zod";

// Initialize OpenRouter with API key (using the env variable)
const openrouter = new OpenRouter({
  apiKey: process.env.OPENROUTER_API_KEY || "", // Ensure you add this to .env.local
});

// The Zod schema to enforce strict output
const ReportCardSchema = z.object({
  headline_summary: z.string().describe("A short, catchy line summarizing the student's week."),
  detailed_analysis: z.string().describe("Professional, encouraging, data-driven analysis."),
  recommended_drill: z.string().describe("Specific drill recommendation based on weaknesses."),
  achievement_badge: z.string().describe("A thematic badge title, e.g., 'Tactical Ninja'."),
});

export async function POST(request: Request) {
  try {
    const studentData = await request.json();

    const systemPrompt = `You are the KingsGambit2700 Lead Analyst. Your tone is professional, encouraging, and data-driven.
Task: Evaluate the student's weekly chess activity and provide a report card.
Logic: 
- Calculate the progress delta (Accuracy % increase).
- Translate technical weaknesses into "Actionable Learning Goals" for a parent.

Input Student Data:
${JSON.stringify(studentData, null, 2)}

Strict Output Requirement:
You MUST return valid JSON matching the following schema. NO markdown backticks, only pure JSON.
{
  "headline_summary": "string",
  "detailed_analysis": "string",
  "recommended_drill": "string",
  "achievement_badge": "string"
}`;

    // Stream the response to get reasoning tokens in usage
    const stream = await openrouter.chat.send({
      chatRequest: {
        model: "nvidia/nemotron-3-super-120b-a12b:free",
        messages: [
          {
            role: "user",
            content: systemPrompt
          }
        ],
        stream: true
      }
    });

    let responseString = "";
    for await (const chunk of stream) {
      const content = chunk.choices[0]?.delta?.content;
      if (content) {
        responseString += content;
      }
    }

    // Clean up potential markdown formatting like ```json ... ```
    let cleanJson = responseString.trim();
    if (cleanJson.startsWith('```json')) {
      cleanJson = cleanJson.replace(/^```json/, '').replace(/```$/, '').trim();
    } else if (cleanJson.startsWith('```')) {
      cleanJson = cleanJson.replace(/^```/, '').replace(/```$/, '').trim();
    }

    // Parse with Zod
    const parsedData = ReportCardSchema.parse(JSON.parse(cleanJson));

    return NextResponse.json(parsedData);
  } catch (error) {
    console.error("AI Report Card Generation Error:", error);
    return NextResponse.json(
      { error: "Failed to generate report card", details: error instanceof Error ? error.message : "Unknown error" },
      { status: 500 }
    );
  }
}

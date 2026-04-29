export type GreetingPeriod = "morning" | "afternoon" | "evening" | "night";

const periods: GreetingPeriod[] = ["morning", "afternoon", "evening", "night"];

export function greetingPeriodFor(date: Date): GreetingPeriod {
  const hour = date.getHours();

  if (hour >= 5 && hour < 12) {
    return "morning";
  }

  if (hour >= 12 && hour < 17) {
    return "afternoon";
  }

  if (hour >= 17 && hour < 21) {
    return "evening";
  }

  return "night";
}

export function parseGreetingMarkdown(markdown: string): Record<GreetingPeriod, string[]> {
  const greetings = emptyGreetingMap();
  let activePeriod: GreetingPeriod | null = null;

  for (const line of markdown.split(/\r?\n/)) {
    const heading = line.match(/^#{1,6}\s+(.+)$/);
    if (heading) {
      activePeriod = parsePeriodLabel(heading[1] ?? "");
      continue;
    }

    const listItem = line.match(/^\s*[-*]\s+(.+)$/);
    if (!listItem || !activePeriod) {
      continue;
    }

    const greeting = (listItem[1] ?? "").trim();
    if (greeting) {
      const periodGreetings = greetings[activePeriod];
      periodGreetings.push(greeting);
    }
  }

  return greetings;
}

export function selectGreeting(markdown: string, date = new Date()): string {
  const period = greetingPeriodFor(date);
  const greetings = parseGreetingMarkdown(markdown);
  const options = greetings[period];
  console.log("options", options);
  console.log("stableGreetingIndex", stableGreetingIndex(date, options.length));
  return options[stableGreetingIndex(date, options.length)] ?? fallbackGreeting(period);
}

function emptyGreetingMap(): Record<GreetingPeriod, string[]> {
  return {
    morning: [],
    afternoon: [],
    evening: [],
    night: [],
  };
}

function parsePeriodLabel(label: string): GreetingPeriod | null {
  const normalizedLabel = label.toLowerCase();
  return periods.find((period) => normalizedLabel.includes(period)) ?? null;
}

function stableGreetingIndex(date: Date, optionCount: number): number {
  if (optionCount <= 1) {
    return 0;
  }

  const hourlySeed = Math.floor(date.getTime() / 3_600_000);
  return hourlySeed % optionCount;
}

function fallbackGreeting(period: GreetingPeriod): string {
  switch (period) {
    case "morning":
      return "Good morning";
    case "afternoon":
      return "Good afternoon";
    case "evening":
      return "Good evening";
    case "night":
      return "Good night";
  }
}

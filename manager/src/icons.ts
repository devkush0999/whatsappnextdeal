const paths: Record<string, string> = {
  grid: "M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M14 14h7v7h-7z",
  phone:
    "M7 3H5a2 2 0 0 0-2 2c0 9 7 16 16 16a2 2 0 0 0 2-2v-2l-5-2-2 2a14 14 0 0 1-7-7l2-2z",
  send: "m3 3 19 9-19 9 4-9z M7 12h15",
  people:
    "M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2 M16 4a4 4 0 0 1 0 8 M22 21v-2a4 4 0 0 0-3-4 M13 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0",
  chat: "M21 15a4 4 0 0 1-4 4H7l-5 3V7a4 4 0 0 1 4-4h11a4 4 0 0 1 4 4z M7 8h10 M7 12h6",
  tag: "m20 13-7 7L2 9V2h7l11 11z M7 7h.01",
  radio:
    "M8 16a6 6 0 0 1 0-8 M16 8a6 6 0 0 1 0 8 M4 20a12 12 0 0 1 0-16 M20 4a12 12 0 0 1 0 16 M12 12h.01",
  settings:
    "M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8 M12 2v3 M12 19v3 M2 12h3 M19 12h3 M5 5l2 2 M17 17l2 2 M5 19l2-2 M17 7l2-2",
  activity: "M3 12h4l3-9 4 18 3-9h4",
  book: "M12 5C7 1 2 3 2 3v17s5-2 10 1c5-3 10-1 10-1V3s-5-2-10 2z M12 5v16",
  search: "M21 21l-5-5 M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0",
  arrow: "M5 12h14 M13 6l6 6-6 6",
  refresh:
    "M20 7v5h-5 M4 17v-5h5 M5 7a8 8 0 0 1 13-2l2 3 M4 16l2 3a8 8 0 0 0 13-2",
  lock: "M5 10h14v11H5z M8 10V6a4 4 0 0 1 8 0v4",
  check: "m5 12 4 4L19 6",
  code: "m8 5-7 7 7 7 M16 5l7 7-7 7",
  shield: "M12 2 3 6v6c0 6 9 10 9 10s9-4 9-10V6z",
  plus: "M12 5v14 M5 12h14",
  close: "m6 6 12 12 M18 6 6 18",
  moon: "M20 15A9 9 0 0 1 9 3a9 9 0 1 0 11 12",
};
export function icon(name: string): string {
  return `<svg aria-hidden="true" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="${paths[name] || paths.grid}"/></svg>`;
}

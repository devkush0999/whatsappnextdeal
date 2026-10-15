export interface Instance {
  id: string;
  name: string;
  token: string;
  connected: boolean;
  jid?: string;
}
export function instancesFrom(value: unknown): Instance[] {
  const rows = Array.isArray(value)
    ? value
    : (value as { data?: unknown })?.data;
  if (!Array.isArray(rows))
    throw new Error(
      "Unexpected instance response. Refresh or check server compatibility.",
    );
  return rows.map((v) => {
    if (
      !v ||
      typeof v.id !== "string" ||
      typeof v.token !== "string" ||
      typeof v.connected !== "boolean"
    )
      throw new Error("Invalid instance response");
    return {
      id: v.id,
      name: typeof v.name === "string" ? v.name : "Unnamed instance",
      token: v.token,
      connected: v.connected,
      jid: typeof v.jid === "string" ? v.jid : undefined,
    };
  });
}
export function escapeHtml(v: unknown): string {
  return String(v ?? "").replace(
    /[&<>"']/g,
    (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        c
      ]!,
  );
}
export function qrImage(value: unknown): string | null {
  if (typeof value !== "string") return null;
  const raw = value.replace(/^data:image\/png;base64,/, "");
  return raw.length > 32 && /^[A-Za-z0-9+/=]+$/.test(raw)
    ? "data:image/png;base64," + raw
    : null;
}
export function maskedNumber(jid?: string): string {
  const n = jid?.split("@")[0].split(":")[0];
  return n && /^\d+$/.test(n) ? "•••• " + n.slice(-4) : "Not linked";
}

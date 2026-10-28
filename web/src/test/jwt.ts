/** テスト用の ID トークン（署名なし。画面は表示用に email を読むだけ） */
export function idToken(claims: Record<string, unknown>): string {
  const b64 = (o: unknown) => btoa(String.fromCharCode(...new TextEncoder().encode(JSON.stringify(o)))).replace(/=+$/, "");
  return `${b64({ alg: "none" })}.${b64(claims)}.sig`;
}

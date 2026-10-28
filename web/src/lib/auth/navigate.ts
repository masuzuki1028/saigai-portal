/** 別のオリジン（Cognito のログイン画面）に移る。テストではモックに差し替える */
export function redirectTo(url: string): void {
  window.location.assign(url);
}

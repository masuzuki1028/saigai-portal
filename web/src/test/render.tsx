import { render } from "@testing-library/react";
import type { ReactElement } from "react";

import { AuthProvider } from "@/lib/auth/AuthProvider";
import { loadScenario } from "@/mocks/db";
import type { ScenarioName } from "@/mocks/fixtures";

/** シナリオのモックデータを読み込み、ログイン済みの状態で描画する */
export function renderLoggedIn(ui: ReactElement, scenario: ScenarioName = "normal") {
  loadScenario(scenario);
  window.sessionStorage.setItem("saigai.localToken", "local-alice");
  return render(<AuthProvider>{ui}</AuthProvider>);
}

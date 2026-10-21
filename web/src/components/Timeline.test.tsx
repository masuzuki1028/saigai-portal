import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { scenarios } from "@/mocks/fixtures";

import { Timeline } from "./Timeline";

const items = Object.values(scenarios["heavy-rain"].timelines)[0]!;

describe("Timeline", () => {
  it("公式情報に出典と公式のラベルを表示する", () => {
    render(<Timeline items={items} />);
    const list = screen.getByRole("list");
    const entries = within(list).getAllByRole("listitem");
    expect(entries).toHaveLength(4);
    expect(within(entries[0]!).getByText("【公式】")).toBeInTheDocument();
    expect(within(entries[0]!).getByText(/出典: 気象庁/)).toBeInTheDocument();
    expect(within(entries[0]!).getByRole("link", { name: "公式の情報を見る" })).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("SNS タブでは公式情報を表示しない", async () => {
    render(<Timeline items={items} />);
    await userEvent.click(screen.getByRole("tab", { name: "SNS" }));
    expect(screen.getByRole("tab", { name: "SNS" })).toHaveAttribute("aria-selected", "true");
    expect(screen.queryByRole("list")).not.toBeInTheDocument();
    expect(screen.getByText(/Phase 2/)).toBeInTheDocument();
  });

  it("情報がないときは空の状態を表示する", () => {
    render(<Timeline items={[]} />);
    expect(screen.getByText(/表示の対象になる情報はまだありません/)).toBeInTheDocument();
  });
});

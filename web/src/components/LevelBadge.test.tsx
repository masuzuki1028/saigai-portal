import { render, screen } from "@testing-library/react";

import { LevelBadge } from "./LevelBadge";

describe("LevelBadge", () => {
  it("色だけでなく文字でもレベルが分かる", () => {
    render(
      <>
        <LevelBadge severity={2} hazard="rain" />
        <LevelBadge severity={3} hazard="flood" />
        <LevelBadge severity={4} hazard="rain" />
        <LevelBadge severity={0} />
      </>,
    );
    expect(screen.getByText(/大雨\s*注意報/)).toBeInTheDocument();
    expect(screen.getByText(/洪水\s*警報/)).toBeInTheDocument();
    expect(screen.getByText(/大雨\s*特別警報/)).toBeInTheDocument();
    expect(screen.getByText(/平常/)).toBeInTheDocument();
  });

  it("地震は震度の目安で表示する", () => {
    render(<LevelBadge severity={4} hazard="earthquake" />);
    expect(screen.getByText(/地震\s*震度6弱以上/)).toBeInTheDocument();
  });
});

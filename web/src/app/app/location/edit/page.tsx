"use client";

import { WithId } from "@/components/WithId";
import { LocationEditScreen } from "@/screens/LocationEditScreen";

export default function Page() {
  return <WithId optional render={(id) => <LocationEditScreen key={id ?? "new"} id={id} />} />;
}

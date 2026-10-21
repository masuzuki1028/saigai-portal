"use client";

import { WithId } from "@/components/WithId";
import { MapScreen } from "@/screens/MapScreen";

export default function Page() {
  return <WithId render={(id) => <MapScreen key={id} id={id!} />} />;
}

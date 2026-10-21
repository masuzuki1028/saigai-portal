"use client";

import { WithId } from "@/components/WithId";
import { TimelineScreen } from "@/screens/TimelineScreen";

export default function Page() {
  return <WithId render={(id) => <TimelineScreen key={id} id={id!} />} />;
}

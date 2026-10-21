import { setupServer } from "msw/node";

import { handlers } from "./handlers";

/** Vitest 用のモックサーバー */
export const server = setupServer(...handlers);

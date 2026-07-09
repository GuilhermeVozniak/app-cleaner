import { cleanup } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { afterEach } from "vitest";

// RTL's automatic afterEach(cleanup) only self-registers when the test
// runner exposes global `afterEach` (Vitest's `test.globals` is off here,
// matching the rest of this repo's explicit-import style) — without this,
// DOM trees from earlier tests in the same file accumulate and later
// getByRole/getByTestId queries fail with "found multiple elements".
afterEach(() => {
  cleanup();
});

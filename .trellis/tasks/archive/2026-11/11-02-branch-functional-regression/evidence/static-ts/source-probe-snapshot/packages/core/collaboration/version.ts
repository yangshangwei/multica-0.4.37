import { z } from "zod";

const decimal = /^[1-9][0-9]{0,18}$/;
export const positiveVersion = z.string().regex(decimal).refine(
  (value) => decimal.test(value) && BigInt(value) <= 9223372036854775807n,
  "Version exceeds a signed 64-bit integer",
);

// A target has generation zero until its first root group is created.
export const nonnegativeVersion = z.union([z.literal("0"), positiveVersion]);

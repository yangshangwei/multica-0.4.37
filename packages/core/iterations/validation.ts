export function validateIterationName(
  value: string,
): "required" | "too_long" | null {
  const length = Array.from(value.replace(/\r\n/g, "\n").trim()).length;
  if (length === 0) return "required";
  return length > 200 ? "too_long" : null;
}

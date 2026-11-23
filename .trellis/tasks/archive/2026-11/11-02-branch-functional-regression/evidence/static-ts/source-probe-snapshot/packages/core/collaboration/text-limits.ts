const encoder = new TextEncoder();

const COLLABORATION_COMMENT_MAX_BYTES = 65_536;
export const COLLABORATION_REACTION_MAX_BYTES = 64;

/** Match Go's len(string) after JSON decoding, including supplementary text. */
export function collaborationTextByteLength(content: string): number {
  return encoder.encode(content).byteLength;
}

export function collaborationCommentWithinByteLimit(content: string): boolean {
  return collaborationTextByteLength(content) <= COLLABORATION_COMMENT_MAX_BYTES;
}

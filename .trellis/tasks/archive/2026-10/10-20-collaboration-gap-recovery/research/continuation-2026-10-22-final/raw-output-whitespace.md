# Immutable command-output whitespace

The scoped `.gitattributes` disables whitespace diagnostics only for raw `.stdout`
and `.stderr` command transcripts in this evidence directory. Turbo/Vitest/Knip
outputs include trailing padding and blank final lines. Those bytes and their
recorded SHA256 values are execution evidence; trimming them would falsify the
raw transcript. Product/test/spec/JSON files retain ordinary whitespace checks.
No raw transcript or hash was normalized to make staging pass.

package agent

// UsageNormalizerVersion names the version of the code that derives a
// TokenUsage from the output of the given provider: which fields of the
// provider's events are read, how they are combined into the four token
// counts per model, and how a provider-reported cost is converted to ticks.
// The daemon sends it with every usage report so the server can tell which
// derivation produced a number.
//
// It must be raised for a provider whenever that derivation changes for it,
// for example when an adapter starts reading a different usage field, counts
// cache tokens differently, or changes how it attributes usage to a model.
// A change that leaves the derived numbers the same for every provider
// output does not raise it.
func UsageNormalizerVersion(provider string) string {
	// Every provider is at version 1. When the derivation of one provider
	// changes, return its own value for that provider key here.
	return "1"
}

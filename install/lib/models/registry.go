package models

// replModels holds the plugin's models for the host's interactive REPL. The
// host merges them at startup through plugin.REPLNamespaces, which drops every
// plugin's models — not just this one's — as soon as a name collides with a
// host model, so the keys below are prefixed rather than left as the bare
// model names.
var replModels = map[string]any{}

func registerREPLModel(name string, model any) {
	if name == "" || model == nil {
		return
	}

	replModels[name] = model
}

// REPLModels returns the plugin's REPL models, keyed by REPL name.
func REPLModels() map[string]any {
	out := make(map[string]any, len(replModels))
	for name, model := range replModels {
		out[name] = model
	}

	return out
}

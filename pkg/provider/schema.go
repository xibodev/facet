package provider

// The kernel validator defaults additionalProperties to false, whereas JSON
// Schema defaults it to true. Make the canonical schema's implicit semantics
// explicit without modifying the shared registry or weakening explicit rules.
func kernelSchema(schema map[string]any) map[string]any {
	out := make(map[string]any, len(schema)+1)
	for key, value := range schema {
		switch v := value.(type) {
		case map[string]any:
			if key == "properties" || key == "$defs" || key == "definitions" {
				children := make(map[string]any, len(v))
				for name, child := range v {
					if s, ok := child.(map[string]any); ok {
						children[name] = kernelSchema(s)
					} else {
						children[name] = child
					}
				}
				out[key] = children
			} else {
				out[key] = kernelSchema(v)
			}
		case []any:
			items := make([]any, len(v))
			for i, item := range v {
				if s, ok := item.(map[string]any); ok {
					items[i] = kernelSchema(s)
				} else {
					items[i] = item
				}
			}
			out[key] = items
		default:
			out[key] = value
		}
	}
	if out["type"] == "object" {
		if _, explicit := out["additionalProperties"]; !explicit {
			out["additionalProperties"] = true
		}
	}
	return out
}

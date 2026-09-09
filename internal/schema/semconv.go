package schema

// semanticDefinition is intentionally a focused compatibility catalog, not a
// copy of the entire OpenTelemetry semantic-conventions registry. TelemetryForge
// uses it to recognize high-value stable attributes and common legacy names.
type semanticDefinition struct {
	Key         string
	Stability   string
	Requirement string
	Expected    ValueType
	Replacement string
	Legacy      bool
}

var semanticDefinitions = map[string]semanticDefinition{
	"service.name":                {Key: "service.name", Stability: "stable", Requirement: "required", Expected: TypeString},
	"service.namespace":           {Key: "service.namespace", Stability: "stable", Requirement: "recommended", Expected: TypeString},
	"service.instance.id":         {Key: "service.instance.id", Stability: "stable", Requirement: "recommended", Expected: TypeString},
	"service.version":             {Key: "service.version", Stability: "stable", Requirement: "recommended", Expected: TypeString},
	"deployment.environment.name": {Key: "deployment.environment.name", Stability: "stable", Requirement: "recommended", Expected: TypeString},
	"http.request.method":         {Key: "http.request.method", Stability: "stable", Requirement: "recommended", Expected: TypeString},
	"http.response.status_code":   {Key: "http.response.status_code", Stability: "stable", Requirement: "recommended", Expected: TypeNumber},
	"network.protocol.version":    {Key: "network.protocol.version", Stability: "stable", Requirement: "recommended", Expected: TypeString},
	"server.address":              {Key: "server.address", Stability: "stable", Requirement: "recommended", Expected: TypeString},
	"server.port":                 {Key: "server.port", Stability: "stable", Requirement: "opt-in", Expected: TypeNumber},
	"url.scheme":                  {Key: "url.scheme", Stability: "stable", Requirement: "opt-in", Expected: TypeString},

	// Common pre-stable HTTP / deployment names retained only to warn and point
	// users toward current semantic-convention attributes.
	"http.method":            {Key: "http.method", Stability: "legacy", Expected: TypeString, Replacement: "http.request.method", Legacy: true},
	"http.status_code":       {Key: "http.status_code", Stability: "legacy", Expected: TypeNumber, Replacement: "http.response.status_code", Legacy: true},
	"net.protocol.name":      {Key: "net.protocol.name", Stability: "legacy", Expected: TypeString, Replacement: "network.protocol.name", Legacy: true},
	"deployment.environment": {Key: "deployment.environment", Stability: "legacy", Expected: TypeString, Replacement: "deployment.environment.name", Legacy: true},
}

func semanticField(path, key string, observed ValueType) (Field, *SemanticFinding) {
	definition, ok := semanticDefinitions[key]
	if !ok {
		return Field{Path: path, Type: observed}, nil
	}

	field := Field{
		Path: path, Type: observed,
		SemanticKey:          definition.Key,
		SemanticStability:    definition.Stability,
		SemanticRequirement:  definition.Requirement,
		SemanticReplacement:  definition.Replacement,
		SemanticExpectedType: definition.Expected,
	}

	if definition.Legacy {
		return field, &SemanticFinding{
			Key: key, Path: path, Severity: "warning", Kind: "legacy_semantic_attribute",
			Message:     "legacy OpenTelemetry semantic-convention attribute is still being emitted",
			Replacement: definition.Replacement,
		}
	}

	// TelemetryForge tags are strings by envelope design, so type checking there
	// would incorrectly flag valid semantic attributes that have been encoded as
	// labels. Payload fields retain JSON types and can be checked safely.
	if definition.Expected != "" && observed != definition.Expected && path[:min(len(path), 8)] == "payload." {
		return field, &SemanticFinding{
			Key: key, Path: path, Severity: "warning", Kind: "semantic_type_mismatch",
			Message: "OpenTelemetry semantic attribute has an unexpected JSON value type",
		}
	}
	return field, nil
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}

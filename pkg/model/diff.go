package model

// ModelDiff represents the differences between two SystemModel versions.
// This is used to detect code changes that require test maintenance.
type ModelDiff struct {
	// Before and after metadata
	OldCommit string `json:"old_commit"`
	NewCommit string `json:"new_commit"`

	// Function changes
	AddedFunctions    []Function       `json:"added_functions"`
	RemovedFunctions  []Function       `json:"removed_functions"`
	ModifiedFunctions []FunctionChange `json:"modified_functions"`

	// Type changes
	AddedTypes    []TypeDef    `json:"added_types"`
	RemovedTypes  []TypeDef    `json:"removed_types"`
	ModifiedTypes []TypeChange `json:"modified_types"`

	// Endpoint changes
	AddedEndpoints    []Endpoint       `json:"added_endpoints"`
	RemovedEndpoints  []Endpoint       `json:"removed_endpoints"`
	ModifiedEndpoints []EndpointChange `json:"modified_endpoints"`

	// Summary statistics
	Stats DiffStats `json:"stats"`
}

// FunctionChange represents a modification to a function between versions
type FunctionChange struct {
	Before      Function `json:"before"`
	After       Function `json:"after"`
	ChangeTypes []string `json:"change_types"` // "signature", "body", "decorators"
}

// TypeChange represents a modification to a type between versions
type TypeChange struct {
	Before      TypeDef  `json:"before"`
	After       TypeDef  `json:"after"`
	ChangeTypes []string `json:"change_types"` // "fields", "methods", "extends"
}

// EndpointChange represents a modification to an endpoint between versions
type EndpointChange struct {
	Before      Endpoint `json:"before"`
	After       Endpoint `json:"after"`
	ChangeTypes []string `json:"change_types"` // "path", "method", "handler", "params"
}

// DiffStats provides summary statistics about the diff
type DiffStats struct {
	FunctionsAdded    int `json:"functions_added"`
	FunctionsRemoved  int `json:"functions_removed"`
	FunctionsModified int `json:"functions_modified"`

	TypesAdded    int `json:"types_added"`
	TypesRemoved  int `json:"types_removed"`
	TypesModified int `json:"types_modified"`

	EndpointsAdded    int `json:"endpoints_added"`
	EndpointsRemoved  int `json:"endpoints_removed"`
	EndpointsModified int `json:"endpoints_modified"`

	TotalChanges int `json:"total_changes"`
}

// HasChanges returns true if there are any changes in the diff
func (d *ModelDiff) HasChanges() bool {
	return d.Stats.TotalChanges > 0
}

// GetAffectedFunctionIDs returns all function IDs that were added, removed, or modified
func (d *ModelDiff) GetAffectedFunctionIDs() []string {
	var ids []string

	for _, fn := range d.AddedFunctions {
		ids = append(ids, fn.ID)
	}
	for _, fn := range d.RemovedFunctions {
		ids = append(ids, fn.ID)
	}
	for _, change := range d.ModifiedFunctions {
		ids = append(ids, change.After.ID)
	}

	return ids
}

// GetAffectedEndpointIDs returns all endpoint IDs that were added, removed, or modified
func (d *ModelDiff) GetAffectedEndpointIDs() []string {
	var ids []string

	for _, ep := range d.AddedEndpoints {
		ids = append(ids, ep.ID)
	}
	for _, ep := range d.RemovedEndpoints {
		ids = append(ids, ep.ID)
	}
	for _, change := range d.ModifiedEndpoints {
		ids = append(ids, change.After.ID)
	}

	return ids
}

// HasSignatureChanges returns true if any function signatures were modified
func (d *ModelDiff) HasSignatureChanges() bool {
	for _, change := range d.ModifiedFunctions {
		for _, ct := range change.ChangeTypes {
			if ct == "signature" {
				return true
			}
		}
	}
	return false
}

// HasEndpointChanges returns true if any endpoints were changed
func (d *ModelDiff) HasEndpointChanges() bool {
	return len(d.AddedEndpoints) > 0 || len(d.RemovedEndpoints) > 0 || len(d.ModifiedEndpoints) > 0
}

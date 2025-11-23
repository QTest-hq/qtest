// Package differ provides functionality to compare SystemModel versions
// and detect changes that may require test maintenance.
package differ

import (
	"reflect"

	"github.com/QTest-hq/qtest/pkg/model"
)

// Differ compares SystemModel versions
type Differ struct{}

// New creates a new Differ instance
func New() *Differ {
	return &Differ{}
}

// DiffSystemModels compares two SystemModel versions and returns the differences
func (d *Differ) DiffSystemModels(old, new *model.SystemModel) *model.ModelDiff {
	diff := &model.ModelDiff{
		OldCommit: old.CommitSHA,
		NewCommit: new.CommitSHA,
	}

	// Compare functions
	d.diffFunctions(old.Functions, new.Functions, diff)

	// Compare types
	d.diffTypes(old.Types, new.Types, diff)

	// Compare endpoints
	d.diffEndpoints(old.Endpoints, new.Endpoints, diff)

	// Calculate stats
	diff.Stats = model.DiffStats{
		FunctionsAdded:    len(diff.AddedFunctions),
		FunctionsRemoved:  len(diff.RemovedFunctions),
		FunctionsModified: len(diff.ModifiedFunctions),
		TypesAdded:        len(diff.AddedTypes),
		TypesRemoved:      len(diff.RemovedTypes),
		TypesModified:     len(diff.ModifiedTypes),
		EndpointsAdded:    len(diff.AddedEndpoints),
		EndpointsRemoved:  len(diff.RemovedEndpoints),
		EndpointsModified: len(diff.ModifiedEndpoints),
	}
	diff.Stats.TotalChanges = diff.Stats.FunctionsAdded + diff.Stats.FunctionsRemoved + diff.Stats.FunctionsModified +
		diff.Stats.TypesAdded + diff.Stats.TypesRemoved + diff.Stats.TypesModified +
		diff.Stats.EndpointsAdded + diff.Stats.EndpointsRemoved + diff.Stats.EndpointsModified

	return diff
}

// diffFunctions compares function lists and populates diff
func (d *Differ) diffFunctions(oldFuncs, newFuncs []model.Function, diff *model.ModelDiff) {
	oldMap := make(map[string]model.Function)
	newMap := make(map[string]model.Function)

	for _, fn := range oldFuncs {
		oldMap[fn.ID] = fn
	}
	for _, fn := range newFuncs {
		newMap[fn.ID] = fn
	}

	// Find added and modified functions
	for id, newFn := range newMap {
		if oldFn, exists := oldMap[id]; exists {
			// Check for modifications
			if changes := d.detectFunctionChanges(oldFn, newFn); len(changes) > 0 {
				diff.ModifiedFunctions = append(diff.ModifiedFunctions, model.FunctionChange{
					Before:      oldFn,
					After:       newFn,
					ChangeTypes: changes,
				})
			}
		} else {
			// Added
			diff.AddedFunctions = append(diff.AddedFunctions, newFn)
		}
	}

	// Find removed functions
	for id, oldFn := range oldMap {
		if _, exists := newMap[id]; !exists {
			diff.RemovedFunctions = append(diff.RemovedFunctions, oldFn)
		}
	}
}

// detectFunctionChanges detects what changed between two function versions
func (d *Differ) detectFunctionChanges(old, new model.Function) []string {
	var changes []string

	// Check signature changes (parameters and returns)
	if !d.parametersEqual(old.Parameters, new.Parameters) ||
		!d.parametersEqual(old.Returns, new.Returns) {
		changes = append(changes, "signature")
	}

	// Check body changes
	if old.Body != new.Body {
		changes = append(changes, "body")
	}

	// Check decorator changes
	if !reflect.DeepEqual(old.Decorators, new.Decorators) {
		changes = append(changes, "decorators")
	}

	return changes
}

// parametersEqual compares two parameter lists
func (d *Differ) parametersEqual(a, b []model.Parameter) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name ||
			a[i].Type != b[i].Type ||
			a[i].Optional != b[i].Optional {
			return false
		}
	}
	return true
}

// diffTypes compares type lists and populates diff
func (d *Differ) diffTypes(oldTypes, newTypes []model.TypeDef, diff *model.ModelDiff) {
	oldMap := make(map[string]model.TypeDef)
	newMap := make(map[string]model.TypeDef)

	for _, t := range oldTypes {
		oldMap[t.ID] = t
	}
	for _, t := range newTypes {
		newMap[t.ID] = t
	}

	// Find added and modified types
	for id, newType := range newMap {
		if oldType, exists := oldMap[id]; exists {
			// Check for modifications
			if changes := d.detectTypeChanges(oldType, newType); len(changes) > 0 {
				diff.ModifiedTypes = append(diff.ModifiedTypes, model.TypeChange{
					Before:      oldType,
					After:       newType,
					ChangeTypes: changes,
				})
			}
		} else {
			// Added
			diff.AddedTypes = append(diff.AddedTypes, newType)
		}
	}

	// Find removed types
	for id, oldType := range oldMap {
		if _, exists := newMap[id]; !exists {
			diff.RemovedTypes = append(diff.RemovedTypes, oldType)
		}
	}
}

// detectTypeChanges detects what changed between two type versions
func (d *Differ) detectTypeChanges(old, new model.TypeDef) []string {
	var changes []string

	// Check field changes
	if !d.fieldsEqual(old.Fields, new.Fields) {
		changes = append(changes, "fields")
	}

	// Check method changes
	if !reflect.DeepEqual(old.Methods, new.Methods) {
		changes = append(changes, "methods")
	}

	// Check extends/implements changes
	if old.Extends != new.Extends || !reflect.DeepEqual(old.Implements, new.Implements) {
		changes = append(changes, "extends")
	}

	return changes
}

// fieldsEqual compares two field lists
func (d *Differ) fieldsEqual(a, b []model.Field) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name ||
			a[i].Type != b[i].Type ||
			a[i].Exported != b[i].Exported {
			return false
		}
	}
	return true
}

// diffEndpoints compares endpoint lists and populates diff
func (d *Differ) diffEndpoints(oldEndpoints, newEndpoints []model.Endpoint, diff *model.ModelDiff) {
	oldMap := make(map[string]model.Endpoint)
	newMap := make(map[string]model.Endpoint)

	for _, ep := range oldEndpoints {
		oldMap[ep.ID] = ep
	}
	for _, ep := range newEndpoints {
		newMap[ep.ID] = ep
	}

	// Find added and modified endpoints
	for id, newEp := range newMap {
		if oldEp, exists := oldMap[id]; exists {
			// Check for modifications
			if changes := d.detectEndpointChanges(oldEp, newEp); len(changes) > 0 {
				diff.ModifiedEndpoints = append(diff.ModifiedEndpoints, model.EndpointChange{
					Before:      oldEp,
					After:       newEp,
					ChangeTypes: changes,
				})
			}
		} else {
			// Added
			diff.AddedEndpoints = append(diff.AddedEndpoints, newEp)
		}
	}

	// Find removed endpoints
	for id, oldEp := range oldMap {
		if _, exists := newMap[id]; !exists {
			diff.RemovedEndpoints = append(diff.RemovedEndpoints, oldEp)
		}
	}
}

// detectEndpointChanges detects what changed between two endpoint versions
func (d *Differ) detectEndpointChanges(old, new model.Endpoint) []string {
	var changes []string

	// Check path changes
	if old.Path != new.Path {
		changes = append(changes, "path")
	}

	// Check method changes
	if old.Method != new.Method {
		changes = append(changes, "method")
	}

	// Check handler changes
	if old.Handler != new.Handler {
		changes = append(changes, "handler")
	}

	// Check params changes
	if !reflect.DeepEqual(old.PathParams, new.PathParams) ||
		!reflect.DeepEqual(old.QueryParams, new.QueryParams) {
		changes = append(changes, "params")
	}

	return changes
}

// GetImpactedTests identifies tests that may need updates based on diff
func (d *Differ) GetImpactedTests(diff *model.ModelDiff, testMapping map[string][]string) []string {
	impactedTestIDs := make(map[string]bool)

	// Functions with signature changes directly impact their tests
	for _, change := range diff.ModifiedFunctions {
		for _, ct := range change.ChangeTypes {
			if ct == "signature" {
				if tests, ok := testMapping[change.After.ID]; ok {
					for _, testID := range tests {
						impactedTestIDs[testID] = true
					}
				}
			}
		}
	}

	// Removed functions mean tests are now orphaned
	for _, fn := range diff.RemovedFunctions {
		if tests, ok := testMapping[fn.ID]; ok {
			for _, testID := range tests {
				impactedTestIDs[testID] = true
			}
		}
	}

	// Modified endpoints impact API tests
	for _, change := range diff.ModifiedEndpoints {
		if tests, ok := testMapping[change.After.ID]; ok {
			for _, testID := range tests {
				impactedTestIDs[testID] = true
			}
		}
	}

	// Removed endpoints mean tests are now orphaned
	for _, ep := range diff.RemovedEndpoints {
		if tests, ok := testMapping[ep.ID]; ok {
			for _, testID := range tests {
				impactedTestIDs[testID] = true
			}
		}
	}

	// Convert to slice
	result := make([]string, 0, len(impactedTestIDs))
	for id := range impactedTestIDs {
		result = append(result, id)
	}
	return result
}

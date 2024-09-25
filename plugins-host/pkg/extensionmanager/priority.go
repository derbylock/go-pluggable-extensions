package extensionmanager

import (
	"fmt"
	"slices"
)

type extensionRuntimeInfoWithDependenciesInfo struct {
	info                    extensionRuntimeInfo
	dependenciesIDs         *Set[string]
	unsatisfiedDependencies *Set[string]
	dependants              *Set[*extensionRuntimeInfoWithDependenciesInfo]
	processed               bool
}

func newExtensionRuntimeInfoWithDependenciesInfo(
	info extensionRuntimeInfo,
) *extensionRuntimeInfoWithDependenciesInfo {
	return &extensionRuntimeInfoWithDependenciesInfo{
		info:                    info,
		dependenciesIDs:         NewSet[string](),
		unsatisfiedDependencies: NewSet[string](),
		dependants:              NewSet[*extensionRuntimeInfoWithDependenciesInfo](),
		processed:               false,
	}
}

func OrderExtensionRuntimeInfo(orig []extensionRuntimeInfo) ([]extensionRuntimeInfo, error) {
	recursiveDependenciesByName, err := createRecursiveDependenciesByExtensionID(orig)
	if err != nil {
		return nil, err
	}

	return prioritizeExtensionRuntimeInfos(orig, recursiveDependenciesByName)
}

func prioritizeExtensionRuntimeInfos(orig []extensionRuntimeInfo, recursiveDependenciesByName map[string]*Set[string]) ([]extensionRuntimeInfo, error) {
	extensionRuntimeInfoByExtensionID := make(map[string]*extensionRuntimeInfoWithDependenciesInfo)
	extensionRuntimeInfoWithDependenciesInfos := make([]*extensionRuntimeInfoWithDependenciesInfo, 0, len(orig))
	for _, info := range orig {
		if _, ok := extensionRuntimeInfoByExtensionID[info.cfg.ID]; ok {
			return nil, fmt.Errorf(`extensionID duplication found for id "%s"`, info.cfg.ID)
		}
		extensionRuntimeInfoWithDependenciesInfo := newExtensionRuntimeInfoWithDependenciesInfo(info)
		extensionRuntimeInfoByExtensionID[info.cfg.ID] = extensionRuntimeInfoWithDependenciesInfo
		extensionRuntimeInfoWithDependenciesInfos = append(
			extensionRuntimeInfoWithDependenciesInfos,
			extensionRuntimeInfoWithDependenciesInfo,
		)
	}

	// there could be sort by custom specified priority

	// process dependency declared priority
	for _, info := range extensionRuntimeInfoWithDependenciesInfos {
		if deps, ok := recursiveDependenciesByName[info.info.cfg.ID]; ok {
			for _, depExtensionID := range deps.Values() {
				if dep, ok := extensionRuntimeInfoByExtensionID[depExtensionID]; ok {
					dep.dependants.Add(info)
					info.dependenciesIDs.Add(dep.info.cfg.ID)
				}
			}
		}
		// init unsatisfied dependencies
		info.unsatisfiedDependencies.AddAll(info.dependenciesIDs)
	}

	// calculate real dependencies
	var sortedInfos []extensionRuntimeInfo
	var err error
	sortedInfos, err = addWithOrderPrevention(extensionRuntimeInfoWithDependenciesInfos, sortedInfos)
	if err != nil {
		return nil, err
	}
	return sortedInfos, nil
}

func addWithOrderPrevention(
	extensionRuntimeInfoWithDependenciesInfos []*extensionRuntimeInfoWithDependenciesInfo,
	sortedInfos []extensionRuntimeInfo,
) ([]extensionRuntimeInfo, error) {
	var lastlyAdded []*extensionRuntimeInfoWithDependenciesInfo
	first := true
	for first || len(lastlyAdded) > 0 {
		first = false
		for _, info := range lastlyAdded {
			for _, dependenciesInfo := range info.dependants.Values() {
				dependenciesInfo.unsatisfiedDependencies.Remove(info.info.cfg.ID)
			}
		}
		lastlyAdded = nil

		// reverse extensionRuntimeInfos so that we'll add them in reversed order according to dependencies
		extensionRuntimeInfoWithDependenciesInfosReversed := make([]*extensionRuntimeInfoWithDependenciesInfo, 0, len(extensionRuntimeInfoWithDependenciesInfos))
		copy(extensionRuntimeInfoWithDependenciesInfosReversed, extensionRuntimeInfoWithDependenciesInfos)
		slices.Reverse(extensionRuntimeInfoWithDependenciesInfosReversed)

		for _, info := range extensionRuntimeInfoWithDependenciesInfosReversed {
			if !info.processed && info.unsatisfiedDependencies.Len() == 0 {
				sortedInfos = append(sortedInfos, info.info)
				info.processed = true
				lastlyAdded = append(lastlyAdded, info)
			}
		}
	}

	// here could be check that all non-optional dependencies are satisfied
	return sortedInfos, nil
}

func createRecursiveDependenciesByExtensionID(orig []extensionRuntimeInfo) (map[string]*Set[string], error) {
	recursiveDependenciesByName := make(map[string]*Set[string])
	// process AfterExtensionIDs
	for _, info := range orig {
		if _, ok := recursiveDependenciesByName[info.cfg.ID]; ok {
			return nil, fmt.Errorf("extension duplication found with extension ID %s", info.cfg.ID)
		}
		afterSet := NewSetFromSlice[string](info.cfg.AfterExtensionIDs)
		recursiveDependenciesByName[info.cfg.ID] = afterSet
	}
	// process BeforeExtensionIDs
	for _, info := range orig {
		beforeSet := NewSetFromSlice[string](info.cfg.BeforeExtensionIDs)
		for _, extensionID := range beforeSet.Values() {
			if dependencies, ok := recursiveDependenciesByName[info.cfg.ID]; ok {
				dependencies.Add(extensionID)
			}
		}
	}

	// process transitive dependencies
	for extensionID, dependenciesIDs := range recursiveDependenciesByName {
		additionalTransitiveDependencies, err := getTransitiveDependencyIDs(
			extensionID,
			recursiveDependenciesByName,
			NewSetFromSlice[string](dependenciesIDs.Values()),
			NewSet[string](),
		)
		if err != nil {
			return nil, err
		}
		dependenciesIDs.AddAll(additionalTransitiveDependencies)
	}
	return recursiveDependenciesByName, nil
}

func getTransitiveDependencyIDs(
	extensionID string,
	recursiveDependenciesByName map[string]*Set[string],
	unprocessedExtensionIDs *Set[string],
	processingExtensionIDs *Set[string],
) (*Set[string], error) {
	additionalDependencies := NewSet[string]()
	unprocessedExtensionIDs.Remove(extensionID)
	processingExtensionIDs.Add(extensionID)
	if dependencyIDs, ok := recursiveDependenciesByName[extensionID]; ok {
		additionalDependencies.AddAll(dependencyIDs)
		for _, dependencyID := range dependencyIDs.Values() {
			if processingExtensionIDs.Contains(dependencyID) {
				return nil, fmt.Errorf(
					`circular transitive dependency found during plugins extensions`+
						` priority resolution for extensionID "%s". Circular dependency on the extensionID="%s"`,
					extensionID,
					dependencyID)
			}
			if unprocessedExtensionIDs.Contains(dependencyID) {
				recursiveDependencies, err := getTransitiveDependencyIDs(dependencyID, recursiveDependenciesByName, unprocessedExtensionIDs, processingExtensionIDs)
				if err != nil {
					return nil, err
				}
				additionalDependencies.AddAll(recursiveDependencies)
			}
		}
	}
	processingExtensionIDs.Remove(extensionID)
	return additionalDependencies, nil
}

package extensionmanager

import "fmt"

type extensionRuntimeInfoWithDependenciesInfo struct {
	info                    extensionRuntimeInfo
	extensionInfoIndex      int
	dependenciesNames       *Set[string]
	unsatisfiedDependencies *Set[string]
	dependants              *Set[extensionRuntimeInfoWithDependenciesInfo]
	processed               bool
}

func newExtensionRuntimeInfoWithDependenciesInfo(
	info extensionRuntimeInfo,
	extensionInfoIndex int,
) *extensionRuntimeInfoWithDependenciesInfo {
	return &extensionRuntimeInfoWithDependenciesInfo{
		info:                    info,
		extensionInfoIndex:      extensionInfoIndex,
		dependenciesNames:       NewSet[string](),
		unsatisfiedDependencies: NewSet[string](),
		dependants:              NewSet[extensionRuntimeInfoWithDependenciesInfo](),
		processed:               false,
	}
}

func OrderExtensionRuntimeInfo(orig []extensionRuntimeInfo) ([]extensionRuntimeInfo, error) {
	recursiveDependenciesByName := make(map[string]*Set[string])
	for _, info := range orig {
		if _, ok := recursiveDependenciesByName[info.cfg.ID]; ok {
			return nil, fmt.Errorf("extension duplication found with extension ID %s", info.cfg.ID)
		}
		afterSet := NewSetFromSlice[string](info.cfg.AfterExtensions)
		recursiveDependenciesByName.put(filterName(filterType), afterSet)
	}
}

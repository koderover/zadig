package featuregates

import "strings"

type Feature string

const (
	ModernWorkflow             Feature = "ModernWorkflow"
	CommunityProjectRepository Feature = "CommunityProjectRepository"
)

var Features FeatureGates = map[Feature]bool{
	ModernWorkflow:             false,
	CommunityProjectRepository: false,
}

type FeatureGates map[Feature]bool

func (fg FeatureGates) EnabledFeatures() []Feature {
	var res []Feature

	for k, v := range fg {
		if v {
			res = append(res, k)
		}
	}

	return res
}

func (fg FeatureGates) FeatureEnabled(f Feature) bool {
	return fg[f]
}

func (fg FeatureGates) MergeFeatureGates(fs FeatureGates) {
	for k, v := range fs {
		fg[k] = v
	}
}

func ToFeatureGates(s string) FeatureGates {
	res := make(FeatureGates)

	fs := strings.Split(s, ",")
	for _, f := range fs {
		kv := strings.Split(f, "=")
		if len(kv) != 2 {
			continue
		}

		res[Feature(kv[0])] = kv[1] == "true"
	}

	return res
}

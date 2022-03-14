package client

import (
	"k8s.io/apimachinery/pkg/util/version"
	k8sversion "k8s.io/apimachinery/pkg/version"
)

var KubernetesVersion122 *version.Version

func init() {
	// as of zadig v1.11.0. Only kubernetes version 1.17+ is supported
	// There is only 1 major api version change from v1.17+
	// More versions of kubernetes should be added if there are more API changes
	// in future kubernetes version
	v122, _ := version.ParseGeneric("v1.22.0")
	KubernetesVersion122 = v122
}

func VersionLessThan122(ver *k8sversion.Info) bool {
	currVersion, _ := version.ParseGeneric(ver.String())
	return currVersion.LessThan(KubernetesVersion122)
}

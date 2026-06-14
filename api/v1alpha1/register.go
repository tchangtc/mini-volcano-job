package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupName is the API group for mini-volcano CRDs.
const GroupName = "mini-volcano.sh"

// SchemeGroupVersion is the identifier for this API group + version.
var SchemeGroupVersion = schema.GroupVersion{
	Group:   GroupName,
	Version: "v1alpha1",
}

var (
	// SchemeBuilder collects functions that register our types with the scheme.
	SchemeBuilder      = runtime.NewSchemeBuilder(addKnownTypes)
	localSchemeBuilder = &SchemeBuilder

	// AddToScheme is a shortcut to register everything.
	AddToScheme = localSchemeBuilder.AddToScheme
)

// Resource takes an unqualified resource name and returns a GroupResource.
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&Job{},
		&JobList{},
		&PodGroup{},
		&PodGroupList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

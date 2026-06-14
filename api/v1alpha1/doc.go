// Package v1alpha1 is the API schema for the mini-volcano-job project.
//
// It defines two CRDs under the mini-volcano.sh/v1alpha1 API group:
//   - Job: a batch job with task templates and gang scheduling
//   - PodGroup: a group of pods scheduled atomically (owned by a Job)
//
// +k8s:deepcopy-gen=package
// +groupName=mini-volcano.sh
package v1alpha1

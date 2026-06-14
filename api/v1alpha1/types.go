package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=jobs,shortName=vj;mvj
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.state.phase`
// +kubebuilder:printcolumn:name="Queue",type=string,priority=1,JSONPath=`.spec.queue`
// +kubebuilder:printcolumn:name="Running",type=integer,JSONPath=`.status.running`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Job is the core workload type of mini-volcano. It describes a batch job composed
// of one or more task groups, each with a pod template and replica count.
type Job struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JobSpec   `json:"spec,omitempty"`
	Status JobStatus `json:"status,omitempty"`
}

// JobSpec describes the desired state of a Job.
type JobSpec struct {
	// SchedulerName is the scheduler that will place pods; defaults to "mini-volcano".
	// +optional
	SchedulerName string `json:"schedulerName,omitempty"`

	// MinAvailable is the minimum number of pods that must be Running for the Job
	// to enter the Running phase. Defaults to the sum of all task replicas.
	// +kubebuilder:validation:Minimum=0
	// +optional
	MinAvailable int32 `json:"minAvailable,omitempty"`

	// Tasks is a list of task specifications. Each task defines a pod template
	// and how many replicas of that template to run.
	// +kubebuilder:validation:MinItems=1
	Tasks []TaskSpec `json:"tasks"`

	// Queue is the name of the queue this job belongs to. Defaults to "default".
	// +kubebuilder:default:="default"
	// +optional
	Queue string `json:"queue,omitempty"`

	// MaxRetry is the maximum number of times the whole Job may be retried
	// before being marked as Failed. Defaults to 3.
	// +kubebuilder:default:=3
	// +kubebuilder:validation:Minimum=0
	// +optional
	MaxRetry int32 `json:"maxRetry,omitempty"`

	// TTLSecondsAfterFinished limits the lifetime of a finished Job. If set,
	// the Job is automatically deleted after this many seconds.
	// +optional
	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`
}

// TaskSpec describes a task within a Job. Each task has a pod template and a
// number of replicas.
type TaskSpec struct {
	// Name is the unique name of this task within the Job.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Name string `json:"name"`

	// Replicas is the number of pods to create from this task's template.
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// Template is the pod template. Every pod of this task is created from
	// this template.
	// +optional
	Template corev1.PodTemplateSpec `json:"template,omitempty"`

	// MinAvailable overrides the Job-level MinAvailable for this specific task.
	// +optional
	MinAvailable *int32 `json:"minAvailable,omitempty"`

	// DependsOn specifies task-level dependencies — this task will not start
	// until all listed tasks reach the Running phase.
	// +optional
	DependsOn []string `json:"dependsOn,omitempty"`
}

// ---------------------------------------------------------------------------
// Job phases and state machine
// ---------------------------------------------------------------------------

// JobPhase is a label for the condition of a job at the current time.
// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed;Terminating
type JobPhase string

const (
	// JobPending means the job has been accepted but its pods have not all been scheduled.
	JobPending JobPhase = "Pending"

	// JobRunning means at least MinAvailable pods are running.
	JobRunning JobPhase = "Running"

	// JobCompleted means all pods finished successfully.
	JobCompleted JobPhase = "Completed"

	// JobFailed means the job has exhausted retries or met a permanent error.
	JobFailed JobPhase = "Failed"

	// JobTerminating means the job is being gracefully stopped.
	JobTerminating JobPhase = "Terminating"
)

// JobState describes the current phase and transition reason of a Job.
type JobState struct {
	// Phase is the current phase of the Job.
	// +optional
	Phase JobPhase `json:"phase,omitempty"`

	// Reason is a brief CamelCase string describing why the phase changed.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable explanation.
	// +optional
	Message string `json:"message,omitempty"`

	// LastTransitionTime is the last time the phase changed.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

// JobStatus represents the observed state of a Job.
type JobStatus struct {
	// State holds the current phase and transition information.
	// +optional
	State JobState `json:"state,omitempty"`

	// Pending is the number of pods currently in Pending phase.
	// +optional
	Pending int32 `json:"pending,omitempty"`

	// Running is the number of pods currently in Running phase.
	// +optional
	Running int32 `json:"running,omitempty"`

	// Succeeded is the number of pods that reached Succeeded phase.
	// +optional
	Succeeded int32 `json:"succeeded,omitempty"`

	// Failed is the number of pods that reached Failed phase.
	// +optional
	Failed int32 `json:"failed,omitempty"`

	// RetryCount is the number of times the job has been retried.
	// +optional
	RetryCount int32 `json:"retryCount,omitempty"`

	// Version is incremented on every spec change.
	// +optional
	Version int32 `json:"version,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// JobList is a list of Job resources.
type JobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Job `json:"items"`
}

// ---------------------------------------------------------------------------
// PodGroup — gang-scheduling primitive
// ---------------------------------------------------------------------------

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=podgroups,shortName=pg;mpg
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="MinMember",type=integer,JSONPath=`.spec.minMember`
// +kubebuilder:printcolumn:name="Running",type=integer,JSONPath=`.status.running`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// PodGroup represents a group of pods that should be scheduled atomically.
// Every mini-volcano Job owns exactly one PodGroup.
type PodGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PodGroupSpec   `json:"spec,omitempty"`
	Status PodGroupStatus `json:"status,omitempty"`
}

// PodGroupSpec is the desired state of a PodGroup.
type PodGroupSpec struct {
	// MinMember is the minimum number of pods that must be assigned and running
	// before this group is considered running.
	// +kubebuilder:validation:Minimum=0
	// +optional
	MinMember int32 `json:"minMember,omitempty"`

	// Queue is the scheduling queue this group belongs to.
	// +optional
	Queue string `json:"queue,omitempty"`
}

// PodGroupPhase is the current phase of a PodGroup.
// +kubebuilder:validation:Enum=Pending;Running;Finished;Failed
type PodGroupPhase string

const (
	// PodGroupPending means fewer than MinMember pods are running.
	PodGroupPending PodGroupPhase = "Pending"

	// PodGroupRunning means at least MinMember pods are running.
	PodGroupRunning PodGroupPhase = "Running"

	// PodGroupFinished means all pods have finished (Succeeded or Failed).
	PodGroupFinished PodGroupPhase = "Finished"

	// PodGroupFailed means the group is permanently unable to make progress.
	PodGroupFailed PodGroupPhase = "Failed"
)

// PodGroupStatus is the observed state of a PodGroup.
type PodGroupStatus struct {
	// Phase is the current phase of the PodGroup.
	// +optional
	Phase PodGroupPhase `json:"phase,omitempty"`

	// Running is the number of pods in the Running phase.
	// +optional
	Running int32 `json:"running,omitempty"`

	// Succeeded is the number of pods in the Succeeded phase.
	// +optional
	Succeeded int32 `json:"succeeded,omitempty"`

	// Failed is the number of pods in the Failed phase.
	// +optional
	Failed int32 `json:"failed,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// PodGroupList is a list of PodGroup resources.
type PodGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodGroup `json:"items"`
}

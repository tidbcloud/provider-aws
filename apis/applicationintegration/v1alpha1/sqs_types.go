/*
Copyright 2019 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
)

// Enum values for Queue attribute names
const (
	AttributeAll                                   string = "All"
	AttributePolicy                                string = "Policy"
	AttributeVisibilityTimeout                     string = "VisibilityTimeout"
	AttributeMaximumMessageSize                    string = "MaximumMessageSize"
	AttributeMessageRetentionPeriod                string = "MessageRetentionPeriod"
	AttributeApproximateNumberOfMessages           string = "ApproximateNumberOfMessages"
	AttributeApproximateNumberOfMessagesNotVisible string = "ApproximateNumberOfMessagesNotVisible"
	AttributeCreatedTimestamp                      string = "CreatedTimestamp"
	AttributeLastModifiedTimestamp                 string = "LastModifiedTimestamp"
	AttributeQueueArn                              string = "QueueArn"
	AttributeApproximateNumberOfMessagesDelayed    string = "ApproximateNumberOfMessagesDelayed"
	AttributeDelaySeconds                          string = "DelaySeconds"
	AttributeReceiveMessageWaitTimeSeconds         string = "ReceiveMessageWaitTimeSeconds"
	AttributeRedrivePolicy                         string = "RedrivePolicy"
	AttributeDeadLetterQueueARN                    string = "DeadLetterQueueARN"
	AttributeMaxReceiveCount                       string = "MaxReceiveCount"
	AttributeFifoQueue                             string = "FifoQueue"
	AttributeContentBasedDeduplication             string = "ContentBasedDeduplication"
	AttributeKmsMasterKeyID                        string = "KmsMasterKeyId"
	AttributeKmsDataKeyReusePeriodSeconds          string = "KmsDataKeyReusePeriodSeconds"
)

// Tag add cost allocation tags to the specified Amazon SQS queue.
// For an overview, see Tagging Your Amazon SQS Queues
// (https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-queue-tags.html)
// in the Amazon Simple Queue Service Developer Guide.
type Tag struct {

	// The key name that can be used to look up or retrieve the associated value.
	// For example, Department or Cost Center are common choices.
	Key string `json:"key"`

	// When you use queue tags, keep the following guidelines in mind:
	//
	//    * Adding more than 50 tags to a queue isn't recommended.
	//
	//    * Tags don't have any semantic meaning. Amazon SQS interprets tags as
	//    character strings.
	//
	//    * Tags are case-sensitive.
	//
	//    * A new tag with a key identical to that of an existing tag overwrites
	//    the existing tag.
	//
	// For a full list of tag restrictions, see Limits Related to Queues
	// (https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-limits.html#limits-queues)
	// in the Amazon Simple Queue Service Developer Guide.
	//
	// To be able to tag a queue on creation, you must have the sqs:CreateQueue
	// and sqs:TagQueue permissions.
	//
	// Cross-account permissions don't apply to this action. For more information,
	// see Grant Cross-Account Permissions to a Role and a User Name
	// (https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-customer-managed-policy-examples.html#grant-cross-account-permissions-to-role-and-user-name)
	// in the Amazon Simple Queue Service Developer Guide.
	// +optional
	Value string `json:"value,omitempty"`
}

// RedrivePolicy includes the parameters for the dead-letter
// queue functionality of the source queue.
type RedrivePolicy struct {
	DeadLetterQueueARN string `json:"deadLetterQueueARN,omitempty"`
	MaxReceiveCount    int    `json:"maxReceiveCount,omitempty"`
}

// Attributes for a Queue with their corresponding values.
type Attributes struct {
	// The length of time, in seconds, for which the delivery
	// of all messages in the queue is delayed. Valid values: An integer from
	// 0 to 900 seconds (15 minutes)
	DelaySeconds int `json:"delaySeconds,omitempty"`

	// The limit of how many bytes a message can contain
	// before Amazon SQS rejects it. Valid values: An integer from 1,024 bytes
	// (1 KiB) to 262,144 bytes (256 KiB). Default: 262,144 (256 KiB).
	MaximumMessageSize int `json:"maximumMessageSize,omitempty"`

	// The length of time, in seconds, for which Amazon
	// SQS retains a message. Valid values: An integer from 60 seconds (1 minute)
	// to 1,209,600 seconds (14 days). Default: 345,600 (4 days).
	MessageRetentionPeriod int `json:"messageRetentionPeriod,omitempty"`

	// The length of time, in seconds, for
	// which a ReceiveMessage action waits for a message to arrive. Valid values:
	// An integer from 0 to 20 (seconds). Default: 0.
	ReceiveMessageWaitTimeSeconds int `json:"receiveMessageWaitTimeSeconds,omitempty"`

	// RedrivePolicy includes the parameters for the dead-letter
	// queue functionality of the source queue.
	RedrivePolicy RedrivePolicy `json:"redrivePolicy,omitempty"`

	// The visibility timeout for the queue, in seconds.
	// Valid values: An integer from 0 to 43,200 (12 hours). Default: 30. For
	// more information about the visibility timeout, see Visibility Timeout
	// (https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-visibility-timeout.html)
	// in the Amazon Simple Queue Service Developer Guide.
	VisibilityTimeout int `json:"visibilityTimeout,omitempty"`

	// The ID of an AWS-managed customer master key (CMK)
	// for Amazon SQS or a custom CMK. For more information, see Key Terms
	// (https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-server-side-encryption.html#sqs-sse-key-terms).
	// While the alias of the AWS-managed CMK for Amazon SQS is always alias/aws/sqs,
	// the alias of a custom CMK can, for example, be alias/MyAlias . For more
	// examples, see KeyId (https://docs.aws.amazon.com/kms/latest/APIReference/API_DescribeKey.html#API_DescribeKey_RequestParameters)
	// in the AWS Key Management Service API Reference.
	KmsMasterKeyID string `json:"kmsMasterKeyID,omitempty"`

	// The length of time, in seconds, for which
	// Amazon SQS can reuse a data key (https://docs.aws.amazon.com/kms/latest/developerguide/concepts.html#data-keys)
	// to encrypt or decrypt messages before calling AWS KMS again. An integer
	// representing seconds, between 60 seconds (1 minute) and 86,400 seconds
	// (24 hours). Default: 300 (5 minutes).
	KmsDataKeyReusePeriodSeconds int `json:"kmsDataKeyReusePeriodSeconds,omitempty"`
}

// QueueParameters define the desired state of an AWS Queue
type QueueParameters struct {
	// Name of the queue. A queue name can have up to 80 characters, a combination of
	// alphanumeric characters, hyphens(-), and underscores(_).
	// A FIFO queue name must end with the .fifo suffix.
	// +kubebuilder:validation:Required
	// +immutable
	Name string `json:"name,omitempty"`

	// Queue attributes with their corresponding values.
	// +optional
	Attributes Attributes `json:"attributes,omitempty"`

	// Designates a queue as FIFO. Valid values: true, false. If
	// you don't specify the FifoQueue attribute, Amazon SQS creates a standard
	// queue. You can provide this attribute only during queue creation. You
	// can't change it for an existing queue.
	// +immutable
	// +optional
	FifoQueue bool `json:"fifoQueue,omitempty"`

	// Tags add cost allocation tags to the specified Amazon SQS queue.
	// For an overview, see Tagging Your Amazon SQS Queues
	// (https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-queue-tags.html)
	// in the Amazon Simple Queue Service Developer Guide.
	// +optional
	Tags []Tag `json:"tags,omitempty"`
}

// QueueSpec defines the desired state of a Queue.
type QueueSpec struct {
	runtimev1alpha1.ResourceSpec `json:",inline"`
	ForProvider                  QueueParameters `json:"forProvider"`
}

// QueueObservation is the representation of the current state that is observed
type QueueObservation struct {
	// Queue attributes with their corresponding values.
	Attributes Attributes `json:"attributes,omitempty"`

	// Designates a queue as FIFO. If user doesn't specify the FifoQueue attribute,
	// Amazon SQS creates a standard queue. You can provide this attribute only during queue creation.
	// You can't change it for an existing queue.
	FifoQueue bool `json:"fifoQueue,omitempty"`
}

// QueueStatus represents the observed state of a Queue.
type QueueStatus struct {
	runtimev1alpha1.ResourceStatus `json:",inline"`
	AtProvider                     QueueObservation `json:"atProvider"`
}

// +kubebuilder:object:root=true

// A Queue is a managed resource that represents a AWS Simple Queue
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
type Queue struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QueueSpec   `json:"spec"`
	Status QueueStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// QueueList contains a list of Queue
type QueueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Queue `json:"items"`
}

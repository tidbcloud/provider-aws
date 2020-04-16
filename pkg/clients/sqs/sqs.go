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

package sqs

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/crossplane/provider-aws/apis/applicationintegration/v1alpha1"
	aws "github.com/crossplane/provider-aws/pkg/clients"
	awsclients "github.com/crossplane/provider-aws/pkg/clients"
)

// Client defines Queue client operations
type Client interface {
	CreateQueueRequest(input *sqs.CreateQueueInput) sqs.CreateQueueRequest
	DeleteQueueRequest(input *sqs.DeleteQueueInput) sqs.DeleteQueueRequest
	TagQueueRequest(input *sqs.TagQueueInput) sqs.TagQueueRequest
	ListQueueTagsRequest(*sqs.ListQueueTagsInput) sqs.ListQueueTagsRequest
	GetQueueAttributesRequest(*sqs.GetQueueAttributesInput) sqs.GetQueueAttributesRequest
	SetQueueAttributesRequest(input *sqs.SetQueueAttributesInput) sqs.SetQueueAttributesRequest
}

// NewClient creates new Queue Client with provided AWS Configurations/Credentials
func NewClient(ctx context.Context, credentials []byte, region string, auth awsclients.AuthMethod) (Client, error) {
	cfg, err := auth(ctx, credentials, awsclients.DefaultSection, region)
	if cfg == nil {
		return nil, err
	}
	return sqs.New(*cfg), err
}

// GenerateCreateQueueInput from QueueSpec
func GenerateCreateQueueInput(name string, p *v1alpha1.QueueParameters) *sqs.CreateQueueInput {
	m := &sqs.CreateQueueInput{
		QueueName:  &p.Name,
		Attributes: GenerateQueueAttributes(p),
		Tags:       GenerateQueueTags(p),
	}
	return m
}

// GenerateQueueAttributes returns a map of queue attributes
func GenerateQueueAttributes(p *v1alpha1.QueueParameters) map[string]string { // nolint:gocyclo
	m := map[string]string{}
	if p.Attributes.DelaySeconds > 0 {
		m[v1alpha1.AttributeDelaySeconds] = strconv.Itoa(p.Attributes.DelaySeconds)
	}
	if p.Attributes.MaximumMessageSize > 0 {
		m[v1alpha1.AttributeDelaySeconds] = strconv.Itoa(p.Attributes.MaximumMessageSize)
	}
	if p.Attributes.MessageRetentionPeriod > 0 {
		m[v1alpha1.AttributeMessageRetentionPeriod] = strconv.Itoa(p.Attributes.MessageRetentionPeriod)
	}
	if p.Attributes.ReceiveMessageWaitTimeSeconds > 0 {
		m[v1alpha1.AttributeReceiveMessageWaitTimeSeconds] = strconv.Itoa(p.Attributes.ReceiveMessageWaitTimeSeconds)
	}
	if p.Attributes.VisibilityTimeout > 0 {
		m[v1alpha1.AttributeVisibilityTimeout] = strconv.Itoa(p.Attributes.VisibilityTimeout)
	}
	if p.Attributes.KmsMasterKeyID != "" {
		m[v1alpha1.AttributeKmsMasterKeyID] = p.Attributes.KmsMasterKeyID
	}
	if p.Attributes.KmsDataKeyReusePeriodSeconds > 0 {
		m[v1alpha1.AttributeKmsDataKeyReusePeriodSeconds] = strconv.Itoa(p.Attributes.KmsDataKeyReusePeriodSeconds)
	}
	if p.Attributes.ReceiveMessageWaitTimeSeconds > 0 {
		m[v1alpha1.AttributeReceiveMessageWaitTimeSeconds] = strconv.Itoa(p.Attributes.ReceiveMessageWaitTimeSeconds)
	}
	if p.FifoQueue {
		m[v1alpha1.AttributeFifoQueue] = strconv.FormatBool(p.FifoQueue)
	}

	// RedrivePolicy example:
	// {"maxReceiveCount": "5","deadLetterTargetArn":"arn:aws:sqs:us-east-2:123456789012:MyDeadLetterQueue"}
	if p.Attributes.RedrivePolicy.DeadLetterQueueARN != "" {
		val, err := json.Marshal(p.Attributes.RedrivePolicy)
		if err == nil {
			m[v1alpha1.AttributeRedrivePolicy] = string(val)
		}
	}
	return m
}

// GenerateQueueTags returns a map of queue tags
func GenerateQueueTags(p *v1alpha1.QueueParameters) map[string]string {
	m := map[string]string{}
	if len(p.Tags) != 0 {
		for _, val := range p.Tags {
			m[val.Key] = val.Value
		}
	}
	return m
}

// IsErrorNotFound helper function to test for ErrCodeTableNotFoundException error
func IsErrorNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "InvalidAddress")
}

// LateInitialize fills the empty fields in *v1alpha1.QueueParameters with
// the values seen in queue.Attributes
func LateInitialize(in *v1alpha1.QueueParameters, attributes map[string]string) {
	if len(attributes) == 0 {
		return
	}
	aws.LateInitializeIntPtr(&in.Attributes.DelaySeconds, aws.Int64(intValue(attributes[v1alpha1.AttributeDelaySeconds])))
	aws.LateInitializeIntPtr(&in.Attributes.KmsDataKeyReusePeriodSeconds, aws.Int64(intValue(attributes[v1alpha1.AttributeKmsDataKeyReusePeriodSeconds])))
	aws.LateInitializeIntPtr(&in.Attributes.MaximumMessageSize, aws.Int64(intValue(attributes[v1alpha1.AttributeMaximumMessageSize])))
	aws.LateInitializeIntPtr(&in.Attributes.MessageRetentionPeriod, aws.Int64(intValue(attributes[v1alpha1.AttributeMessageRetentionPeriod])))
	aws.LateInitializeIntPtr(&in.Attributes.ReceiveMessageWaitTimeSeconds, aws.Int64(intValue(attributes[v1alpha1.AttributeReceiveMessageWaitTimeSeconds])))
	aws.LateInitializeIntPtr(&in.Attributes.VisibilityTimeout, aws.Int64(intValue(attributes[v1alpha1.AttributeVisibilityTimeout])))
	aws.LateInitializeStringPtr(&in.Attributes.KmsMasterKeyID, aws.String(attributes[v1alpha1.AttributeKmsMasterKeyID]))
	aws.LateInitializeStringPtr(&in.Attributes.RedrivePolicy.DeadLetterQueueARN, aws.String(attributes[v1alpha1.AttributeDeadLetterQueueARN]))
	aws.LateInitializeIntPtr(&in.Attributes.RedrivePolicy.MaxReceiveCount, aws.Int64(intValue(attributes[v1alpha1.AttributeMaxReceiveCount])))
}

func intValue(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}

// GenerateObservation is used to produce v1alpha1.QueueObservation from queue.Attributes
func GenerateObservation(attributes map[string]string) v1alpha1.QueueObservation {
	o := v1alpha1.QueueObservation{
		Attributes: v1alpha1.Attributes{
			DelaySeconds:                  intValue(attributes[v1alpha1.AttributeDelaySeconds]),
			KmsDataKeyReusePeriodSeconds:  intValue(attributes[v1alpha1.AttributeKmsDataKeyReusePeriodSeconds]),
			KmsMasterKeyID:                attributes[v1alpha1.AttributeKmsMasterKeyID],
			MaximumMessageSize:            intValue(attributes[v1alpha1.AttributeMaximumMessageSize]),
			MessageRetentionPeriod:        intValue(attributes[v1alpha1.AttributeMessageRetentionPeriod]),
			ReceiveMessageWaitTimeSeconds: intValue(attributes[v1alpha1.AttributeReceiveMessageWaitTimeSeconds]),
			VisibilityTimeout:             intValue(attributes[v1alpha1.AttributeVisibilityTimeout]),
			RedrivePolicy: v1alpha1.RedrivePolicy{
				DeadLetterQueueARN: attributes[v1alpha1.AttributeDeadLetterQueueARN],
				MaxReceiveCount:    intValue(attributes[v1alpha1.AttributeMaxReceiveCount]),
			},
		},
	}
	return o
}

// IsUpToDate checks whether there is a change in any of the modifiable fields.
func IsUpToDate(p v1alpha1.QueueParameters, attributes map[string]string, tags map[string]string) bool { // nolint:gocyclo
	if len(p.Tags) != len(tags) {
		return false
	}
	pTags := make(map[string]string, len(p.Tags))
	for _, tag := range p.Tags {
		pTags[tag.Key] = tag.Value
	}
	for key, val := range tags {
		pVal, ok := pTags[key]
		if !ok || !strings.EqualFold(pVal, val) {
			return false
		}
	}

	if p.Attributes.DelaySeconds != intValue(attributes[v1alpha1.AttributeDelaySeconds]) {
		return false
	}
	if p.Attributes.KmsDataKeyReusePeriodSeconds != intValue(attributes[v1alpha1.AttributeKmsDataKeyReusePeriodSeconds]) {
		return false
	}
	if p.Attributes.MaximumMessageSize != intValue(attributes[v1alpha1.AttributeMaximumMessageSize]) {
		return false
	}
	if p.Attributes.MessageRetentionPeriod != intValue(attributes[v1alpha1.AttributeMessageRetentionPeriod]) {
		return false
	}
	if p.Attributes.ReceiveMessageWaitTimeSeconds != intValue(attributes[v1alpha1.AttributeReceiveMessageWaitTimeSeconds]) {
		return false
	}
	if p.Attributes.VisibilityTimeout != intValue(attributes[v1alpha1.AttributeVisibilityTimeout]) {
		return false
	}
	if !strings.EqualFold(p.Attributes.KmsMasterKeyID, attributes[v1alpha1.AttributeDelaySeconds]) {
		return false
	}
	if !strings.EqualFold(p.Attributes.RedrivePolicy.DeadLetterQueueARN, attributes[v1alpha1.AttributeDeadLetterQueueARN]) {
		return false
	}
	if p.Attributes.RedrivePolicy.MaxReceiveCount != intValue(attributes[v1alpha1.AttributeMaxReceiveCount]) {
		return false
	}

	return true
}

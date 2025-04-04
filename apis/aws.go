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

// Package apis contains Kubernetes API groups for AWS cloud provider.
package apis

import (
	"k8s.io/apimachinery/pkg/runtime"

	ec2v1alpha1 "github.com/crossplane/provider-aws/apis/ec2/v1alpha1"
	ec2v1beta1 "github.com/crossplane/provider-aws/apis/ec2/v1beta1"
	eksmanualv1alpha1 "github.com/crossplane/provider-aws/apis/eks/manualv1alpha1"
	eksv1alpha1 "github.com/crossplane/provider-aws/apis/eks/v1alpha1"
	eksv1beta1 "github.com/crossplane/provider-aws/apis/eks/v1beta1"
	elasticloadbalancingv1alpha1 "github.com/crossplane/provider-aws/apis/elasticloadbalancing/v1alpha1"
	elbv2v1alpha1 "github.com/crossplane/provider-aws/apis/elbv2/v1alpha1"
	iamv1beta1 "github.com/crossplane/provider-aws/apis/iam/v1beta1"
	kmsv1alpha1 "github.com/crossplane/provider-aws/apis/kms/v1alpha1"
	lambdav1alpha1 "github.com/crossplane/provider-aws/apis/lambda/v1alpha1"
	route53v1alpha1 "github.com/crossplane/provider-aws/apis/route53/v1alpha1"
	s3v1alpha2 "github.com/crossplane/provider-aws/apis/s3/v1alpha3"
	s3v1beta1 "github.com/crossplane/provider-aws/apis/s3/v1beta1"
	sqsv1beta1 "github.com/crossplane/provider-aws/apis/sqs/v1beta1"
	awsv1alpha3 "github.com/crossplane/provider-aws/apis/v1alpha3"
	awsv1beta1 "github.com/crossplane/provider-aws/apis/v1beta1"
	peeringv1alpha1 "github.com/crossplane/provider-aws/apis/vpcpeering/v1alpha1"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes,
		elasticloadbalancingv1alpha1.SchemeBuilder.AddToScheme,
		iamv1beta1.SchemeBuilder.AddToScheme,
		elbv2v1alpha1.SchemeBuilder.AddToScheme,
		route53v1alpha1.SchemeBuilder.AddToScheme,
		ec2v1beta1.SchemeBuilder.AddToScheme,
		awsv1alpha3.SchemeBuilder.AddToScheme,
		awsv1beta1.SchemeBuilder.AddToScheme,
		s3v1alpha2.SchemeBuilder.AddToScheme,
		s3v1beta1.SchemeBuilder.AddToScheme,
		sqsv1beta1.SchemeBuilder.AddToScheme,
		kmsv1alpha1.SchemeBuilder.AddToScheme,
		ec2v1alpha1.SchemeBuilder.AddToScheme,
		lambdav1alpha1.SchemeBuilder.AddToScheme,
		peeringv1alpha1.SchemeBuilder.AddToScheme,
		eksv1beta1.SchemeBuilder.AddToScheme,
		eksv1alpha1.SchemeBuilder.AddToScheme,
		eksmanualv1alpha1.SchemeBuilder.AddToScheme,
	)
}

// AddToSchemes may be used to add all resources defined in the project to a Scheme
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}

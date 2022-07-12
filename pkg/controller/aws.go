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

package controller

import (
	"time"

	"github.com/crossplane/provider-aws/pkg/controller/iam/policy"
	"github.com/crossplane/provider-aws/pkg/controller/iam/role"
	"github.com/crossplane/provider-aws/pkg/controller/iam/rolepolicyattachment"
	"github.com/crossplane/provider-aws/pkg/controller/vpcpeering"

	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/provider-aws/pkg/controller/config"
	"github.com/crossplane/provider-aws/pkg/controller/ec2/address"
	"github.com/crossplane/provider-aws/pkg/controller/ec2/internetgateway"
	"github.com/crossplane/provider-aws/pkg/controller/ec2/natgateway"
	"github.com/crossplane/provider-aws/pkg/controller/ec2/routetable"
	"github.com/crossplane/provider-aws/pkg/controller/ec2/securitygroup"
	"github.com/crossplane/provider-aws/pkg/controller/ec2/subnet"
	"github.com/crossplane/provider-aws/pkg/controller/ec2/vpc"
	"github.com/crossplane/provider-aws/pkg/controller/ec2/vpcendpoint"
	"github.com/crossplane/provider-aws/pkg/controller/ec2/vpcendpointserviceconfiguration"
	"github.com/crossplane/provider-aws/pkg/controller/eks"
	"github.com/crossplane/provider-aws/pkg/controller/eks/nodegroup"
	"github.com/crossplane/provider-aws/pkg/controller/elasticloadbalancing/elb"
	"github.com/crossplane/provider-aws/pkg/controller/elasticloadbalancing/elbattachment"
	"github.com/crossplane/provider-aws/pkg/controller/kms/key"
	"github.com/crossplane/provider-aws/pkg/controller/lambda/function"
	"github.com/crossplane/provider-aws/pkg/controller/route53/hostedzone"
	"github.com/crossplane/provider-aws/pkg/controller/route53/resourcerecordset"
	"github.com/crossplane/provider-aws/pkg/controller/s3"
	"github.com/crossplane/provider-aws/pkg/controller/s3/bucketpolicy"
	"github.com/crossplane/provider-aws/pkg/controller/sqs/queue"
)

// Setup creates all AWS controllers with the supplied logger and adds them to
// the supplied manager.
func Setup(mgr ctrl.Manager, l logging.Logger, rl workqueue.RateLimiter, poll time.Duration) error {
	for _, setup := range []func(ctrl.Manager, logging.Logger, workqueue.RateLimiter, time.Duration) error{
		elb.SetupELB,
		elbattachment.SetupELBAttachment,
		s3.SetupBucket,
		bucketpolicy.SetupBucketPolicy,
		vpc.SetupVPC,
		resourcerecordset.SetupResourceRecordSet,
		hostedzone.SetupHostedZone,
		queue.SetupQueue,
		key.SetupKey,
		eks.SetupCluster,
		nodegroup.SetupNodeGroup,
		policy.SetupPolicy,
		role.SetupRole,
		rolepolicyattachment.SetupRolePolicyAttachment,
		function.SetupFunction,
		vpcendpoint.SetupVPCEndpoint,
		vpcendpointserviceconfiguration.SetupVPCEndpointServiceConfiguration,
		securitygroup.SetupSecurityGroup,
		subnet.SetupSubnet,
		internetgateway.SetupInternetGateway,
		natgateway.SetupNatGateway,
		routetable.SetupRouteTable,
		address.SetupAddress,
	} {
		if err := setup(mgr, l, rl, poll); err != nil {
			return err
		}
	}

	for _, setup := range []func(ctrl.Manager, logging.Logger, workqueue.RateLimiter) error{
		config.Setup,
		vpcpeering.SetupVPCPeeringConnection,
	} {
		if err := setup(mgr, l, rl); err != nil {
			return err
		}
	}

	return config.Setup(mgr, l, rl)
}

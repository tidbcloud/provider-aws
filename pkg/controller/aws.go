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
	"github.com/crossplane/provider-aws/pkg/controller/vpcpeering"

	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/provider-aws/pkg/controller/config"
	"github.com/crossplane/provider-aws/pkg/controller/ec2/vpc"
	"github.com/crossplane/provider-aws/pkg/controller/eks"
	"github.com/crossplane/provider-aws/pkg/controller/eks/nodegroup"
	"github.com/crossplane/provider-aws/pkg/controller/elasticloadbalancing/elb"
	"github.com/crossplane/provider-aws/pkg/controller/elasticloadbalancing/elbattachment"
	"github.com/crossplane/provider-aws/pkg/controller/identity/iampolicy"
	"github.com/crossplane/provider-aws/pkg/controller/identity/iamrole"
	"github.com/crossplane/provider-aws/pkg/controller/identity/iamrolepolicyattachment"
	"github.com/crossplane/provider-aws/pkg/controller/kms/key"
	"github.com/crossplane/provider-aws/pkg/controller/route53/hostedzone"
	"github.com/crossplane/provider-aws/pkg/controller/route53/resourcerecordset"
	"github.com/crossplane/provider-aws/pkg/controller/route53resolver/resolverendpoint"
	"github.com/crossplane/provider-aws/pkg/controller/route53resolver/resolverrule"
	"github.com/crossplane/provider-aws/pkg/controller/s3"
	"github.com/crossplane/provider-aws/pkg/controller/s3/bucketpolicy"
	"github.com/crossplane/provider-aws/pkg/controller/sqs/queue"
	transferserver "github.com/crossplane/provider-aws/pkg/controller/transfer/server"
	transferuser "github.com/crossplane/provider-aws/pkg/controller/transfer/user"
)

// Setup creates all AWS controllers with the supplied logger and adds them to
// the supplied manager.
func Setup(mgr ctrl.Manager, l logging.Logger, rl workqueue.RateLimiter) error {
	for _, setup := range []func(ctrl.Manager, logging.Logger, workqueue.RateLimiter) error{
		config.Setup,
		elb.SetupELB,
		elbattachment.SetupELBAttachment,
		s3.SetupBucket,
		bucketpolicy.SetupBucketPolicy,
		iampolicy.SetupIAMPolicy,
		iamrole.SetupIAMRole,
		iamrolepolicyattachment.SetupIAMRolePolicyAttachment,
		vpc.SetupVPC,
		resourcerecordset.SetupResourceRecordSet,
		hostedzone.SetupHostedZone,
		queue.SetupQueue,
		key.SetupKey,
		vpcpeering.SetupVPCPeeringConnection,
		eks.SetupCluster,
		nodegroup.SetupNodeGroup,
	} {
		if err := setup(mgr, l, rl, poll); err != nil {
			return err
		}
	}

	return config.Setup(mgr, l, rl)
}

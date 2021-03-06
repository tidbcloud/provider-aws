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

package dbsubnetgroup

import (
	"context"

	"reflect"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsrds "github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/pkg/errors"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/provider-aws/apis/database/v1beta1"
	awsclient "github.com/crossplane/provider-aws/pkg/clients"
	dbsg "github.com/crossplane/provider-aws/pkg/clients/dbsubnetgroup"
)

const (
	errLateInit         = "cannot late initialize DBSubnetGroup"
	errUnexpectedObject = "the managed resource is not an DBSubnetGroup"
	errDescribe         = "cannot describe DBSubnetGroup"
	errCreate           = "cannot create the DBSubnetGroup"
	errDelete           = "cannot delete the DBSubnetGroup"
	errUpdate           = "cannot update the DBSubnetGroup"
	errAddTagsFailed    = "cannot add tags to DBSubnetGroup"
	errListTagsFailed   = "cannot list tags for DBSubnetGroup"
	errNotOne           = "expected exactly one DBSubnetGroup"
)

// SetupDBSubnetGroup adds a controller that reconciles DBSubnetGroups.
func SetupDBSubnetGroup(mgr ctrl.Manager, l logging.Logger, rl workqueue.RateLimiter) error {
	name := managed.ControllerName(v1beta1.DBSubnetGroupGroupKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(controller.Options{
			RateLimiter: ratelimiter.NewDefaultManagedRateLimiter(rl),
		}).
		For(&v1beta1.DBSubnetGroup{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1beta1.DBSubnetGroupGroupVersionKind),
			managed.WithExternalConnecter(&connector{kube: mgr.GetClient(), newClientFn: dbsg.NewClient}),
			managed.WithReferenceResolver(managed.NewAPISimpleReferenceResolver(mgr.GetClient())),
			managed.WithConnectionPublishers(),
			managed.WithLogger(l.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name)))))
}

type connector struct {
	kube        client.Client
	newClientFn func(config aws.Config) dbsg.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1beta1.DBSubnetGroup)
	if !ok {
		return nil, errors.New(errUnexpectedObject)
	}
	cfg, err := awsclient.GetConfig(ctx, c.kube, mg, aws.StringValue(cr.Spec.ForProvider.Region))
	if err != nil {
		return nil, err
	}
	return &external{c.newClientFn(*cfg), c.kube}, nil
}

type external struct {
	client dbsg.Client
	kube   client.Client
}

func (e *external) Observe(ctx context.Context, mgd resource.Managed) (managed.ExternalObservation, error) { // nolint:gocyclo
	cr, ok := mgd.(*v1beta1.DBSubnetGroup)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errUnexpectedObject)
	}

	req := e.client.DescribeDBSubnetGroupsRequest(&awsrds.DescribeDBSubnetGroupsInput{
		DBSubnetGroupName: aws.String(meta.GetExternalName(cr)),
	})
	res, err := req.Send(ctx)
	if err != nil {
		return managed.ExternalObservation{}, awsclient.Wrap(resource.Ignore(dbsg.IsErrorNotFound, err), errDescribe)
	}

	// in a successful response, there should be one and only one object
	if len(res.DBSubnetGroups) != 1 {
		return managed.ExternalObservation{}, errors.New(errNotOne)
	}

	observed := res.DBSubnetGroups[0]
	current := cr.Spec.ForProvider.DeepCopy()
	dbsg.LateInitialize(&cr.Spec.ForProvider, &observed)
	if !reflect.DeepEqual(current, &cr.Spec.ForProvider) {
		if err := e.kube.Update(ctx, cr); err != nil {
			return managed.ExternalObservation{}, errors.Wrap(err, errLateInit)
		}
	}
	cr.Status.AtProvider = dbsg.GenerateObservation(observed)

	if strings.EqualFold(cr.Status.AtProvider.State, v1beta1.DBSubnetGroupStateAvailable) {
		cr.Status.SetConditions(xpv1.Available())
	} else {
		cr.Status.SetConditions(xpv1.Unavailable())
	}

	tags, err := e.client.ListTagsForResourceRequest(&awsrds.ListTagsForResourceInput{
		ResourceName: aws.String(cr.Status.AtProvider.ARN),
	}).Send(ctx)
	if err != nil {
		return managed.ExternalObservation{}, awsclient.Wrap(err, errListTagsFailed)
	}

	return managed.ExternalObservation{
		ResourceUpToDate: dbsg.IsDBSubnetGroupUpToDate(cr.Spec.ForProvider, observed, tags.TagList),
		ResourceExists:   true,
	}, nil
}

func (e *external) Create(ctx context.Context, mgd resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mgd.(*v1beta1.DBSubnetGroup)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errUnexpectedObject)
	}

	cr.SetConditions(xpv1.Creating())
	input := &awsrds.CreateDBSubnetGroupInput{
		DBSubnetGroupDescription: aws.String(cr.Spec.ForProvider.Description),
		DBSubnetGroupName:        aws.String(meta.GetExternalName(cr)),
		SubnetIds:                cr.Spec.ForProvider.SubnetIDs,
	}

	if len(cr.Spec.ForProvider.Tags) != 0 {
		input.Tags = make([]awsrds.Tag, len(cr.Spec.ForProvider.Tags))
		for i, val := range cr.Spec.ForProvider.Tags {
			input.Tags[i] = awsrds.Tag{Key: aws.String(val.Key), Value: aws.String(val.Value)}
		}
	}

	_, err := e.client.CreateDBSubnetGroupRequest(input).Send(ctx)
	return managed.ExternalCreation{}, awsclient.Wrap(err, errCreate)
}

func (e *external) Update(ctx context.Context, mgd resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mgd.(*v1beta1.DBSubnetGroup)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errUnexpectedObject)
	}

	_, err := e.client.ModifyDBSubnetGroupRequest(&awsrds.ModifyDBSubnetGroupInput{
		DBSubnetGroupName:        aws.String(meta.GetExternalName(cr)),
		DBSubnetGroupDescription: aws.String(cr.Spec.ForProvider.Description),
		SubnetIds:                cr.Spec.ForProvider.SubnetIDs,
	}).Send(ctx)

	if err != nil {
		return managed.ExternalUpdate{}, awsclient.Wrap(err, errUpdate)
	}

	if len(cr.Spec.ForProvider.Tags) > 0 {
		tags := make([]awsrds.Tag, len(cr.Spec.ForProvider.Tags))
		for i, t := range cr.Spec.ForProvider.Tags {
			tags[i] = awsrds.Tag{Key: aws.String(t.Key), Value: aws.String(t.Value)}
		}
		_, err = e.client.AddTagsToResourceRequest(&awsrds.AddTagsToResourceInput{
			ResourceName: aws.String(cr.Status.AtProvider.ARN),
			Tags:         tags,
		}).Send(ctx)
		if err != nil {
			return managed.ExternalUpdate{}, awsclient.Wrap(err, errAddTagsFailed)
		}
	}
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mgd resource.Managed) error {
	cr, ok := mgd.(*v1beta1.DBSubnetGroup)
	if !ok {
		return errors.New(errUnexpectedObject)
	}

	cr.SetConditions(xpv1.Deleting())
	_, err := e.client.DeleteDBSubnetGroupRequest(&awsrds.DeleteDBSubnetGroupInput{
		DBSubnetGroupName: aws.String(meta.GetExternalName(cr)),
	}).Send(ctx)
	return awsclient.Wrap(resource.Ignore(dbsg.IsDBSubnetGroupNotFoundErr, err), errDelete)
}

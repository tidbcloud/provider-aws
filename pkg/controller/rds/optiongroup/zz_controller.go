/*
Copyright 2021 The Crossplane Authors.

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

// Code generated by ack-generate. DO NOT EDIT.

package optiongroup

import (
	"context"

	svcapi "github.com/aws/aws-sdk-go/service/rds"
	svcsdk "github.com/aws/aws-sdk-go/service/rds"
	svcsdkapi "github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	cpresource "github.com/crossplane/crossplane-runtime/pkg/resource"

	svcapitypes "github.com/crossplane-contrib/provider-aws/apis/rds/v1alpha1"
	connectaws "github.com/crossplane-contrib/provider-aws/pkg/utils/connect/aws"
	errorutils "github.com/crossplane-contrib/provider-aws/pkg/utils/errors"
)

const (
	errUnexpectedObject = "managed resource is not an OptionGroup resource"

	errCreateSession = "cannot create a new session"
	errCreate        = "cannot create OptionGroup in AWS"
	errUpdate        = "cannot update OptionGroup in AWS"
	errDescribe      = "failed to describe OptionGroup"
	errDelete        = "failed to delete OptionGroup"
)

type connector struct {
	kube client.Client
	opts []option
}

func (c *connector) Connect(ctx context.Context, cr *svcapitypes.OptionGroup) (managed.TypedExternalClient[*svcapitypes.OptionGroup], error) {
	sess, err := connectaws.GetConfigV1(ctx, c.kube, cr, cr.Spec.ForProvider.Region)
	if err != nil {
		return nil, errors.Wrap(err, errCreateSession)
	}
	return newExternal(c.kube, svcapi.New(sess), c.opts), nil
}

func (e *external) Observe(ctx context.Context, cr *svcapitypes.OptionGroup) (managed.ExternalObservation, error) {
	if meta.GetExternalName(cr) == "" {
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}
	input := GenerateDescribeOptionGroupsInput(cr)
	if err := e.preObserve(ctx, cr, input); err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, "pre-observe failed")
	}
	resp, err := e.client.DescribeOptionGroupsWithContext(ctx, input)
	if err != nil {
		return managed.ExternalObservation{ResourceExists: false}, errorutils.Wrap(cpresource.Ignore(IsNotFound, err), errDescribe)
	}
	resp = e.filterList(cr, resp)
	if len(resp.OptionGroupsList) == 0 {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	currentSpec := cr.Spec.ForProvider.DeepCopy()
	if err := e.lateInitialize(&cr.Spec.ForProvider, resp); err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, "late-init failed")
	}
	GenerateOptionGroup(resp).Status.AtProvider.DeepCopyInto(&cr.Status.AtProvider)
	upToDate := true
	diff := ""
	if !meta.WasDeleted(cr) { // There is no need to run isUpToDate if the resource is deleted
		upToDate, diff, err = e.isUpToDate(ctx, cr, resp)
		if err != nil {
			return managed.ExternalObservation{}, errors.Wrap(err, "isUpToDate check failed")
		}
	}
	return e.postObserve(ctx, cr, resp, managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        upToDate,
		Diff:                    diff,
		ResourceLateInitialized: !cmp.Equal(&cr.Spec.ForProvider, currentSpec),
	}, nil)
}

func (e *external) Create(ctx context.Context, cr *svcapitypes.OptionGroup) (managed.ExternalCreation, error) {
	cr.Status.SetConditions(xpv1.Creating())
	input := GenerateCreateOptionGroupInput(cr)
	if err := e.preCreate(ctx, cr, input); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "pre-create failed")
	}
	resp, err := e.client.CreateOptionGroupWithContext(ctx, input)
	if err != nil {
		return managed.ExternalCreation{}, errorutils.Wrap(err, errCreate)
	}

	if resp.OptionGroup.AllowsVpcAndNonVpcInstanceMemberships != nil {
		cr.Status.AtProvider.AllowsVPCAndNonVPCInstanceMemberships = resp.OptionGroup.AllowsVpcAndNonVpcInstanceMemberships
	} else {
		cr.Status.AtProvider.AllowsVPCAndNonVPCInstanceMemberships = nil
	}
	if resp.OptionGroup.CopyTimestamp != nil {
		cr.Status.AtProvider.CopyTimestamp = &metav1.Time{*resp.OptionGroup.CopyTimestamp}
	} else {
		cr.Status.AtProvider.CopyTimestamp = nil
	}
	if resp.OptionGroup.EngineName != nil {
		cr.Spec.ForProvider.EngineName = resp.OptionGroup.EngineName
	} else {
		cr.Spec.ForProvider.EngineName = nil
	}
	if resp.OptionGroup.MajorEngineVersion != nil {
		cr.Spec.ForProvider.MajorEngineVersion = resp.OptionGroup.MajorEngineVersion
	} else {
		cr.Spec.ForProvider.MajorEngineVersion = nil
	}
	if resp.OptionGroup.OptionGroupArn != nil {
		cr.Status.AtProvider.OptionGroupARN = resp.OptionGroup.OptionGroupArn
	} else {
		cr.Status.AtProvider.OptionGroupARN = nil
	}
	if resp.OptionGroup.OptionGroupDescription != nil {
		cr.Spec.ForProvider.OptionGroupDescription = resp.OptionGroup.OptionGroupDescription
	} else {
		cr.Spec.ForProvider.OptionGroupDescription = nil
	}
	if resp.OptionGroup.OptionGroupName != nil {
		cr.Status.AtProvider.OptionGroupName = resp.OptionGroup.OptionGroupName
	} else {
		cr.Status.AtProvider.OptionGroupName = nil
	}
	if resp.OptionGroup.Options != nil {
		f7 := []*svcapitypes.Option{}
		for _, f7iter := range resp.OptionGroup.Options {
			f7elem := &svcapitypes.Option{}
			if f7iter.DBSecurityGroupMemberships != nil {
				f7elemf0 := []*svcapitypes.DBSecurityGroupMembership{}
				for _, f7elemf0iter := range f7iter.DBSecurityGroupMemberships {
					f7elemf0elem := &svcapitypes.DBSecurityGroupMembership{}
					if f7elemf0iter.DBSecurityGroupName != nil {
						f7elemf0elem.DBSecurityGroupName = f7elemf0iter.DBSecurityGroupName
					}
					if f7elemf0iter.Status != nil {
						f7elemf0elem.Status = f7elemf0iter.Status
					}
					f7elemf0 = append(f7elemf0, f7elemf0elem)
				}
				f7elem.DBSecurityGroupMemberships = f7elemf0
			}
			if f7iter.OptionDescription != nil {
				f7elem.OptionDescription = f7iter.OptionDescription
			}
			if f7iter.OptionName != nil {
				f7elem.OptionName = f7iter.OptionName
			}
			if f7iter.OptionSettings != nil {
				f7elemf3 := []*svcapitypes.OptionSetting{}
				for _, f7elemf3iter := range f7iter.OptionSettings {
					f7elemf3elem := &svcapitypes.OptionSetting{}
					if f7elemf3iter.AllowedValues != nil {
						f7elemf3elem.AllowedValues = f7elemf3iter.AllowedValues
					}
					if f7elemf3iter.ApplyType != nil {
						f7elemf3elem.ApplyType = f7elemf3iter.ApplyType
					}
					if f7elemf3iter.DataType != nil {
						f7elemf3elem.DataType = f7elemf3iter.DataType
					}
					if f7elemf3iter.DefaultValue != nil {
						f7elemf3elem.DefaultValue = f7elemf3iter.DefaultValue
					}
					if f7elemf3iter.Description != nil {
						f7elemf3elem.Description = f7elemf3iter.Description
					}
					if f7elemf3iter.IsCollection != nil {
						f7elemf3elem.IsCollection = f7elemf3iter.IsCollection
					}
					if f7elemf3iter.IsModifiable != nil {
						f7elemf3elem.IsModifiable = f7elemf3iter.IsModifiable
					}
					if f7elemf3iter.Name != nil {
						f7elemf3elem.Name = f7elemf3iter.Name
					}
					if f7elemf3iter.Value != nil {
						f7elemf3elem.Value = f7elemf3iter.Value
					}
					f7elemf3 = append(f7elemf3, f7elemf3elem)
				}
				f7elem.OptionSettings = f7elemf3
			}
			if f7iter.OptionVersion != nil {
				f7elem.OptionVersion = f7iter.OptionVersion
			}
			if f7iter.Permanent != nil {
				f7elem.Permanent = f7iter.Permanent
			}
			if f7iter.Persistent != nil {
				f7elem.Persistent = f7iter.Persistent
			}
			if f7iter.Port != nil {
				f7elem.Port = f7iter.Port
			}
			if f7iter.VpcSecurityGroupMemberships != nil {
				f7elemf8 := []*svcapitypes.VPCSecurityGroupMembership{}
				for _, f7elemf8iter := range f7iter.VpcSecurityGroupMemberships {
					f7elemf8elem := &svcapitypes.VPCSecurityGroupMembership{}
					if f7elemf8iter.Status != nil {
						f7elemf8elem.Status = f7elemf8iter.Status
					}
					if f7elemf8iter.VpcSecurityGroupId != nil {
						f7elemf8elem.VPCSecurityGroupID = f7elemf8iter.VpcSecurityGroupId
					}
					f7elemf8 = append(f7elemf8, f7elemf8elem)
				}
				f7elem.VPCSecurityGroupMemberships = f7elemf8
			}
			f7 = append(f7, f7elem)
		}
		cr.Status.AtProvider.Options = f7
	} else {
		cr.Status.AtProvider.Options = nil
	}
	if resp.OptionGroup.SourceAccountId != nil {
		cr.Status.AtProvider.SourceAccountID = resp.OptionGroup.SourceAccountId
	} else {
		cr.Status.AtProvider.SourceAccountID = nil
	}
	if resp.OptionGroup.SourceOptionGroup != nil {
		cr.Status.AtProvider.SourceOptionGroup = resp.OptionGroup.SourceOptionGroup
	} else {
		cr.Status.AtProvider.SourceOptionGroup = nil
	}
	if resp.OptionGroup.VpcId != nil {
		cr.Status.AtProvider.VPCID = resp.OptionGroup.VpcId
	} else {
		cr.Status.AtProvider.VPCID = nil
	}

	return e.postCreate(ctx, cr, resp, managed.ExternalCreation{}, err)
}

func (e *external) Update(ctx context.Context, cr *svcapitypes.OptionGroup) (managed.ExternalUpdate, error) {
	input := GenerateModifyOptionGroupInput(cr)
	if err := e.preUpdate(ctx, cr, input); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "pre-update failed")
	}
	resp, err := e.client.ModifyOptionGroupWithContext(ctx, input)
	return e.postUpdate(ctx, cr, resp, managed.ExternalUpdate{}, errorutils.Wrap(err, errUpdate))
}

func (e *external) Delete(ctx context.Context, cr *svcapitypes.OptionGroup) (managed.ExternalDelete, error) {
	cr.Status.SetConditions(xpv1.Deleting())
	input := GenerateDeleteOptionGroupInput(cr)
	ignore, err := e.preDelete(ctx, cr, input)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "pre-delete failed")
	}
	if ignore {
		return managed.ExternalDelete{}, nil
	}
	resp, err := e.client.DeleteOptionGroupWithContext(ctx, input)
	return e.postDelete(ctx, cr, resp, errorutils.Wrap(cpresource.Ignore(IsNotFound, err), errDelete))
}

func (e *external) Disconnect(ctx context.Context) error {
	// Unimplemented, required by newer versions of crossplane-runtime
	return nil
}

type option func(*external)

func newExternal(kube client.Client, client svcsdkapi.RDSAPI, opts []option) *external {
	e := &external{
		kube:           kube,
		client:         client,
		preObserve:     nopPreObserve,
		postObserve:    nopPostObserve,
		lateInitialize: nopLateInitialize,
		isUpToDate:     alwaysUpToDate,
		filterList:     nopFilterList,
		preCreate:      nopPreCreate,
		postCreate:     nopPostCreate,
		preDelete:      nopPreDelete,
		postDelete:     nopPostDelete,
		preUpdate:      nopPreUpdate,
		postUpdate:     nopPostUpdate,
	}
	for _, f := range opts {
		f(e)
	}
	return e
}

type external struct {
	kube           client.Client
	client         svcsdkapi.RDSAPI
	preObserve     func(context.Context, *svcapitypes.OptionGroup, *svcsdk.DescribeOptionGroupsInput) error
	postObserve    func(context.Context, *svcapitypes.OptionGroup, *svcsdk.DescribeOptionGroupsOutput, managed.ExternalObservation, error) (managed.ExternalObservation, error)
	filterList     func(*svcapitypes.OptionGroup, *svcsdk.DescribeOptionGroupsOutput) *svcsdk.DescribeOptionGroupsOutput
	lateInitialize func(*svcapitypes.OptionGroupParameters, *svcsdk.DescribeOptionGroupsOutput) error
	isUpToDate     func(context.Context, *svcapitypes.OptionGroup, *svcsdk.DescribeOptionGroupsOutput) (bool, string, error)
	preCreate      func(context.Context, *svcapitypes.OptionGroup, *svcsdk.CreateOptionGroupInput) error
	postCreate     func(context.Context, *svcapitypes.OptionGroup, *svcsdk.CreateOptionGroupOutput, managed.ExternalCreation, error) (managed.ExternalCreation, error)
	preDelete      func(context.Context, *svcapitypes.OptionGroup, *svcsdk.DeleteOptionGroupInput) (bool, error)
	postDelete     func(context.Context, *svcapitypes.OptionGroup, *svcsdk.DeleteOptionGroupOutput, error) (managed.ExternalDelete, error)
	preUpdate      func(context.Context, *svcapitypes.OptionGroup, *svcsdk.ModifyOptionGroupInput) error
	postUpdate     func(context.Context, *svcapitypes.OptionGroup, *svcsdk.ModifyOptionGroupOutput, managed.ExternalUpdate, error) (managed.ExternalUpdate, error)
}

func nopPreObserve(context.Context, *svcapitypes.OptionGroup, *svcsdk.DescribeOptionGroupsInput) error {
	return nil
}
func nopPostObserve(_ context.Context, _ *svcapitypes.OptionGroup, _ *svcsdk.DescribeOptionGroupsOutput, obs managed.ExternalObservation, err error) (managed.ExternalObservation, error) {
	return obs, err
}
func nopFilterList(_ *svcapitypes.OptionGroup, list *svcsdk.DescribeOptionGroupsOutput) *svcsdk.DescribeOptionGroupsOutput {
	return list
}

func nopLateInitialize(*svcapitypes.OptionGroupParameters, *svcsdk.DescribeOptionGroupsOutput) error {
	return nil
}
func alwaysUpToDate(context.Context, *svcapitypes.OptionGroup, *svcsdk.DescribeOptionGroupsOutput) (bool, string, error) {
	return true, "", nil
}

func nopPreCreate(context.Context, *svcapitypes.OptionGroup, *svcsdk.CreateOptionGroupInput) error {
	return nil
}
func nopPostCreate(_ context.Context, _ *svcapitypes.OptionGroup, _ *svcsdk.CreateOptionGroupOutput, cre managed.ExternalCreation, err error) (managed.ExternalCreation, error) {
	return cre, err
}
func nopPreDelete(context.Context, *svcapitypes.OptionGroup, *svcsdk.DeleteOptionGroupInput) (bool, error) {
	return false, nil
}
func nopPostDelete(_ context.Context, _ *svcapitypes.OptionGroup, _ *svcsdk.DeleteOptionGroupOutput, err error) (managed.ExternalDelete, error) {
	return managed.ExternalDelete{}, err
}
func nopPreUpdate(context.Context, *svcapitypes.OptionGroup, *svcsdk.ModifyOptionGroupInput) error {
	return nil
}
func nopPostUpdate(_ context.Context, _ *svcapitypes.OptionGroup, _ *svcsdk.ModifyOptionGroupOutput, upd managed.ExternalUpdate, err error) (managed.ExternalUpdate, error) {
	return upd, err
}

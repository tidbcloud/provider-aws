package vpcpeering

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/route53"

	"github.com/crossplane/provider-aws/pkg/clients/peering"

	"github.com/aws/aws-sdk-go/aws"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	corev1 "k8s.io/api/core/v1"

	svcapitypes "github.com/crossplane/provider-aws/apis/vpcpeering/v1alpha1"

	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cpresource "github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/aws/aws-sdk-go/aws/awserr"

	awsclient "github.com/crossplane/provider-aws/pkg/clients"
)

// ConnectionStateReasonCode vpc connection state code
type ConnectionStateReasonCode string

const (
	errUnexpectedObject = "managed resource is not an VPCPeeringConnection resource"

	errCreate              = "cannot create VPCPeeringConnection in AWS"
	errCreateHostzone      = "cannot create HostedZoneAssosciation in AWS"
	errDescribe            = "failed to describe VPCPeeringConnection"
	errDescribeRouteTable  = "failed to describe RouteTable"
	errModifyVpcPeering    = "failed to modify VPCPeeringConnection"
	errUpdateManagedStatus = "cannot update managed resource status"

	routeTableEnsured = "tidbcloud.com/route-table-ensured"
	hostedZoneEnsured = "tidbcloud.com/hosted-zone-ensured"
	attributeModified = "tidbcloud.com/attribute-modified"
)

// SetupVPCPeeringConnection adds a controller that reconciles VPCPeeringConnection.
func SetupVPCPeeringConnection(mgr ctrl.Manager, l logging.Logger, rl workqueue.RateLimiter) error {
	name := managed.ControllerName(svcapitypes.VPCPeeringConnectionGroupKind)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(controller.Options{
			RateLimiter: ratelimiter.NewDefaultManagedRateLimiter(rl),
		}).
		For(&svcapitypes.VPCPeeringConnection{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(svcapitypes.VPCPeeringConnectionGroupVersionKind),
			managed.WithExternalConnecter(&connector{kube: mgr.GetClient(), log: l}),
			managed.WithLogger(l.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name)))))
}

const (
	// ApprovedCondition resources are believed to be approved.
	ApprovedCondition xpv1.ConditionType = "Approved"
	// ApprovedConditionReason customer approved the vpc request.
	ApprovedConditionReason xpv1.ConditionReason = "CreateRouteInfo"
)

// Approved return approve condition
func Approved() xpv1.Condition {
	return xpv1.Condition{
		Type:               ApprovedCondition,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ApprovedConditionReason,
	}
}

type connector struct {
	kube client.Client
	log  logging.Logger
}

func (c *connector) Connect(ctx context.Context, mg cpresource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*svcapitypes.VPCPeeringConnection)
	if !ok {
		return nil, errors.New(errUnexpectedObject)
	}
	cfg, err := awsclient.GetConfig(ctx, c.kube, mg, cr.Spec.ForProvider.Region)
	if err != nil {
		return nil, err
	}

	return &external{
		kube:          c.kube,
		route53Client: peering.NewRoute53Client(*cfg),
		client:        peering.NewEc2Client(*cfg),
		log:           c.log,
	}, nil
}

func isUPToDate(conditions []xpv1.Condition) bool {
	if len(conditions) == 0 {
		return false
	}

	for _, c := range conditions {
		if c.Type == xpv1.TypeReady {
			if c.Status == corev1.ConditionTrue {
				return true
			}
		}
	}

	return false
}

type external struct {
	kube          client.Client
	client        peering.EC2Client
	route53Client peering.Route53Client
	log           logging.Logger
}

func (e *external) Observe(ctx context.Context, mg cpresource.Managed) (managed.ExternalObservation, error) { // nolint:gocyclo
	cr, ok := mg.(*svcapitypes.VPCPeeringConnection)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errUnexpectedObject)
	}
	if meta.GetExternalName(cr) == "" {
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	input := peering.GenerateDescribeVpcPeeringConnectionsInput(cr)
	resp, err := e.client.DescribeVpcPeeringConnectionsRequest(input).Send(ctx)
	if err != nil {
		return managed.ExternalObservation{ResourceExists: false}, awsclient.Wrap(err, errDescribe)
	}

	if len(resp.VpcPeeringConnections) == 0 {
		return managed.ExternalObservation{ResourceExists: true}, nil
	}

	existedPeer := resp.VpcPeeringConnections[0]

	if !(existedPeer.Status.Code == ec2.VpcPeeringConnectionStateReasonCodeInitiatingRequest || existedPeer.Status.Code == ec2.VpcPeeringConnectionStateReasonCodePendingAcceptance || existedPeer.Status.Code == ec2.VpcPeeringConnectionStateReasonCodeActive || existedPeer.Status.Code == ec2.VpcPeeringConnectionStateReasonCodeExpired || existedPeer.Status.Code == ec2.VpcPeeringConnectionStateReasonCodeRejected) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	_, routeTableReady := cr.GetAnnotations()[routeTableEnsured]
	_, hostZoneReady := cr.GetAnnotations()[hostedZoneEnsured]
	_, attributeReady := cr.GetAnnotations()[attributeModified]
	if !routeTableReady || !hostZoneReady || !attributeReady {
		return managed.ExternalObservation{
			ResourceExists: true,
			// vpc peering post processing not complete, forward to Update()
			ResourceUpToDate: false,
		}, errors.Wrap(e.kube.Status().Update(ctx, cr), errUpdateManagedStatus)
	}

	peering.BuildPeering(resp).Status.AtProvider.DeepCopyInto(&cr.Status.AtProvider)
	if existedPeer.Status.Code == ec2.VpcPeeringConnectionStateReasonCodeActive && cr.GetCondition(ApprovedCondition).Status == corev1.ConditionTrue {
		cr.Status.SetConditions(xpv1.Available())
	}

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        isUPToDate(cr.Status.Conditions),
		ResourceLateInitialized: true,
	}, errors.Wrap(e.kube.Status().Update(ctx, cr), errUpdateManagedStatus)
}

func (e *external) Create(ctx context.Context, mg cpresource.Managed) (managed.ExternalCreation, error) { // nolint:gocyclo
	cr, ok := mg.(*svcapitypes.VPCPeeringConnection)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errUnexpectedObject)
	}
	cr.Status.SetConditions(xpv1.Creating())
	input := peering.GenerateCreateVpcPeeringConnectionInput(cr)

	resp, err := e.client.CreateVpcPeeringConnectionRequest(input).Send(ctx)
	if err != nil {
		return managed.ExternalCreation{}, awsclient.Wrap(err, errCreate)
	}

	tags := make([]ec2.Tag, 0)
	tags = append(tags, ec2.Tag{
		Key:   aws.String("Name"),
		Value: aws.String(cr.ObjectMeta.Name),
	})

	for _, tag := range cr.Spec.ForProvider.Tags {
		tags = append(tags, ec2.Tag{
			Key:   tag.Key,
			Value: tag.Value,
		})
	}

	_, err = e.client.CreateTagsRequest(&ec2.CreateTagsInput{
		Resources: []string{
			*resp.VpcPeeringConnection.VpcPeeringConnectionId,
		},
		Tags: tags,
	}).Send(ctx)
	if err != nil {
		return managed.ExternalCreation{}, awsclient.Wrap(err, errCreate)
	}

	meta.SetExternalName(cr, aws.StringValue(resp.VpcPeeringConnection.VpcPeeringConnectionId))

	if resp.VpcPeeringConnection.AccepterVpcInfo != nil {
		f0 := &svcapitypes.VPCPeeringConnectionVPCInfo{}
		if resp.VpcPeeringConnection.AccepterVpcInfo.CidrBlock != nil {
			f0.CIDRBlock = resp.VpcPeeringConnection.AccepterVpcInfo.CidrBlock
		}
		if resp.VpcPeeringConnection.AccepterVpcInfo.CidrBlockSet != nil {
			f0f1 := []*svcapitypes.CIDRBlock{}
			for _, f0f1iter := range resp.VpcPeeringConnection.AccepterVpcInfo.CidrBlockSet {
				f0f1elem := &svcapitypes.CIDRBlock{}
				if f0f1iter.CidrBlock != nil {
					f0f1elem.CIDRBlock = f0f1iter.CidrBlock
				}
				f0f1 = append(f0f1, f0f1elem)
			}
			f0.CIDRBlockSet = f0f1
		}
		if resp.VpcPeeringConnection.AccepterVpcInfo.Ipv6CidrBlockSet != nil {
			f0f2 := []*svcapitypes.IPv6CIDRBlock{}
			for _, f0f2iter := range resp.VpcPeeringConnection.AccepterVpcInfo.Ipv6CidrBlockSet {
				f0f2elem := &svcapitypes.IPv6CIDRBlock{}
				if f0f2iter.Ipv6CidrBlock != nil {
					f0f2elem.IPv6CIDRBlock = f0f2iter.Ipv6CidrBlock
				}
				f0f2 = append(f0f2, f0f2elem)
			}
			f0.IPv6CIDRBlockSet = f0f2
		}
		if resp.VpcPeeringConnection.AccepterVpcInfo.OwnerId != nil {
			f0.OwnerID = resp.VpcPeeringConnection.AccepterVpcInfo.OwnerId
		}
		if resp.VpcPeeringConnection.AccepterVpcInfo.PeeringOptions != nil {
			f0f4 := &svcapitypes.VPCPeeringConnectionOptionsDescription{}
			if resp.VpcPeeringConnection.AccepterVpcInfo.PeeringOptions.AllowDnsResolutionFromRemoteVpc != nil {
				f0f4.AllowDNSResolutionFromRemoteVPC = resp.VpcPeeringConnection.AccepterVpcInfo.PeeringOptions.AllowDnsResolutionFromRemoteVpc
			}
			if resp.VpcPeeringConnection.AccepterVpcInfo.PeeringOptions.AllowEgressFromLocalClassicLinkToRemoteVpc != nil {
				f0f4.AllowEgressFromLocalClassicLinkToRemoteVPC = resp.VpcPeeringConnection.AccepterVpcInfo.PeeringOptions.AllowEgressFromLocalClassicLinkToRemoteVpc
			}
			if resp.VpcPeeringConnection.AccepterVpcInfo.PeeringOptions.AllowEgressFromLocalVpcToRemoteClassicLink != nil {
				f0f4.AllowEgressFromLocalVPCToRemoteClassicLink = resp.VpcPeeringConnection.AccepterVpcInfo.PeeringOptions.AllowEgressFromLocalVpcToRemoteClassicLink
			}
			f0.PeeringOptions = f0f4
		}
		if resp.VpcPeeringConnection.AccepterVpcInfo.Region != nil {
			f0.Region = resp.VpcPeeringConnection.AccepterVpcInfo.Region
		}
		if resp.VpcPeeringConnection.AccepterVpcInfo.VpcId != nil {
			f0.VPCID = resp.VpcPeeringConnection.AccepterVpcInfo.VpcId
		}
		cr.Status.AtProvider.AccepterVPCInfo = f0
	} else {
		cr.Status.AtProvider.AccepterVPCInfo = nil
	}
	if resp.VpcPeeringConnection.ExpirationTime != nil {
		cr.Status.AtProvider.ExpirationTime = &metav1.Time{
			Time: *resp.VpcPeeringConnection.ExpirationTime,
		}
	} else {
		cr.Status.AtProvider.ExpirationTime = nil
	}
	if resp.VpcPeeringConnection.RequesterVpcInfo != nil {
		f2 := &svcapitypes.VPCPeeringConnectionVPCInfo{}
		if resp.VpcPeeringConnection.RequesterVpcInfo.CidrBlock != nil {
			f2.CIDRBlock = resp.VpcPeeringConnection.RequesterVpcInfo.CidrBlock
		}
		if resp.VpcPeeringConnection.RequesterVpcInfo.CidrBlockSet != nil {
			f2f1 := []*svcapitypes.CIDRBlock{}
			for _, f2f1iter := range resp.VpcPeeringConnection.RequesterVpcInfo.CidrBlockSet {
				f2f1elem := &svcapitypes.CIDRBlock{}
				if f2f1iter.CidrBlock != nil {
					f2f1elem.CIDRBlock = f2f1iter.CidrBlock
				}
				f2f1 = append(f2f1, f2f1elem)
			}
			f2.CIDRBlockSet = f2f1
		}
		if resp.VpcPeeringConnection.RequesterVpcInfo.Ipv6CidrBlockSet != nil {
			f2f2 := []*svcapitypes.IPv6CIDRBlock{}
			for _, f2f2iter := range resp.VpcPeeringConnection.RequesterVpcInfo.Ipv6CidrBlockSet {
				f2f2elem := &svcapitypes.IPv6CIDRBlock{}
				if f2f2iter.Ipv6CidrBlock != nil {
					f2f2elem.IPv6CIDRBlock = f2f2iter.Ipv6CidrBlock
				}
				f2f2 = append(f2f2, f2f2elem)
			}
			f2.IPv6CIDRBlockSet = f2f2
		}
		if resp.VpcPeeringConnection.RequesterVpcInfo.OwnerId != nil {
			f2.OwnerID = resp.VpcPeeringConnection.RequesterVpcInfo.OwnerId
		}
		if resp.VpcPeeringConnection.RequesterVpcInfo.PeeringOptions != nil {
			f2f4 := &svcapitypes.VPCPeeringConnectionOptionsDescription{}
			if resp.VpcPeeringConnection.RequesterVpcInfo.PeeringOptions.AllowDnsResolutionFromRemoteVpc != nil {
				f2f4.AllowDNSResolutionFromRemoteVPC = resp.VpcPeeringConnection.RequesterVpcInfo.PeeringOptions.AllowDnsResolutionFromRemoteVpc
			}
			if resp.VpcPeeringConnection.RequesterVpcInfo.PeeringOptions.AllowEgressFromLocalClassicLinkToRemoteVpc != nil {
				f2f4.AllowEgressFromLocalClassicLinkToRemoteVPC = resp.VpcPeeringConnection.RequesterVpcInfo.PeeringOptions.AllowEgressFromLocalClassicLinkToRemoteVpc
			}
			if resp.VpcPeeringConnection.RequesterVpcInfo.PeeringOptions.AllowEgressFromLocalVpcToRemoteClassicLink != nil {
				f2f4.AllowEgressFromLocalVPCToRemoteClassicLink = resp.VpcPeeringConnection.RequesterVpcInfo.PeeringOptions.AllowEgressFromLocalVpcToRemoteClassicLink
			}
			f2.PeeringOptions = f2f4
		}
		if resp.VpcPeeringConnection.RequesterVpcInfo.Region != nil {
			f2.Region = resp.VpcPeeringConnection.RequesterVpcInfo.Region
		}
		if resp.VpcPeeringConnection.RequesterVpcInfo.VpcId != nil {
			f2.VPCID = resp.VpcPeeringConnection.RequesterVpcInfo.VpcId
		}
		cr.Status.AtProvider.RequesterVPCInfo = f2
	} else {
		cr.Status.AtProvider.RequesterVPCInfo = nil
	}
	if resp.VpcPeeringConnection.Status != nil {
		f3 := &svcapitypes.VPCPeeringConnectionStateReason{}
		f3.Code = aws.String(string(resp.VpcPeeringConnection.Status.Code))
		if resp.VpcPeeringConnection.Status.Message != nil {
			f3.Message = resp.VpcPeeringConnection.Status.Message
		}
		cr.Status.AtProvider.Status = f3
	} else {
		cr.Status.AtProvider.Status = nil
	}
	if resp.VpcPeeringConnection.Tags != nil {
		f4 := []*svcapitypes.Tag{}
		for _, f4iter := range resp.VpcPeeringConnection.Tags {
			f4elem := &svcapitypes.Tag{}
			if f4iter.Key != nil {
				f4elem.Key = f4iter.Key
			}
			if f4iter.Value != nil {
				f4elem.Value = f4iter.Value
			}
			f4 = append(f4, f4elem)
		}
		cr.Status.AtProvider.Tags = f4
	} else {
		cr.Status.AtProvider.Tags = nil
	}
	if resp.VpcPeeringConnection.VpcPeeringConnectionId != nil {
		cr.Status.AtProvider.VPCPeeringConnectionID = resp.VpcPeeringConnection.VpcPeeringConnectionId
	} else {
		cr.Status.AtProvider.VPCPeeringConnectionID = nil
	}

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg cpresource.Managed) (managed.ExternalUpdate, error) { // nolint:gocyclo
	cr, ok := mg.(*svcapitypes.VPCPeeringConnection)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errUnexpectedObject)
	}
	_, routeTableReady := cr.GetAnnotations()[routeTableEnsured]
	_, hostZoneReady := cr.GetAnnotations()[hostedZoneEnsured]
	_, attributeReady := cr.GetAnnotations()[attributeModified]

	if !attributeReady {
		modifyVpcPeeringConnectionOptionsInput := &ec2.ModifyVpcPeeringConnectionOptionsInput{
			VpcPeeringConnectionId: cr.Status.AtProvider.VPCPeeringConnectionID,
			RequesterPeeringConnectionOptions: &ec2.PeeringConnectionOptionsRequest{
				AllowDnsResolutionFromRemoteVpc: aws.Bool(true),
			},
		}
		_, err := e.client.ModifyVpcPeeringConnectionOptionsRequest(modifyVpcPeeringConnectionOptionsInput).Send(ctx)
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, errModifyVpcPeering)
		}
		if cr.Annotations == nil {
			cr.Annotations = map[string]string{}
		}
		cr.Annotations[attributeModified] = "true"
		err = e.kube.Update(ctx, cr)
		if err != nil {
			return managed.ExternalUpdate{}, awsclient.Wrap(err, "error update peering annotations")
		}
	}

	if !routeTableReady {
		filter := ec2.Filter{
			Name: aws.String("vpc-id"),
			Values: []string{
				*cr.Spec.ForProvider.VPCID,
			},
		}
		describeRouteTablesInput := &ec2.DescribeRouteTablesInput{
			Filters:    []ec2.Filter{filter},
			MaxResults: aws.Int64(10),
		}
		routeTablesRes, err := e.client.DescribeRouteTablesRequest(describeRouteTablesInput).Send(ctx)
		if err != nil {
			return managed.ExternalUpdate{}, awsclient.Wrap(err, errDescribeRouteTable)
		}

		for _, rt := range routeTablesRes.RouteTables {
			createRouteInput := &ec2.CreateRouteInput{
				RouteTableId:           rt.RouteTableId,
				DestinationCidrBlock:   cr.Spec.ForProvider.PeerCIDR,
				VpcPeeringConnectionId: cr.Status.AtProvider.VPCPeeringConnectionID,
			}
			createRouteRes, err := e.client.CreateRouteRequest(createRouteInput).Send(ctx)
			if err != nil {
				if aerr, ok := err.(awserr.Error); ok {
					if aerr.Code() != "RouteAlreadyExists" {
						return managed.ExternalUpdate{}, awsclient.Wrap(err, errCreate)
					}
				}
			} else {
				e.log.Info("Create route for route table", "RouteTableID", *rt.RouteTableId, "return", *createRouteRes.Return)
			}
		}
		if cr.Annotations == nil {
			cr.Annotations = map[string]string{}
		}
		cr.Annotations[routeTableEnsured] = "true"
		err = e.kube.Update(ctx, cr)
		if err != nil {
			return managed.ExternalUpdate{}, awsclient.Wrap(err, "error update peering annotations")
		}
	}

	if !hostZoneReady {
		vpcAssociationAuthorizationInput := &route53.CreateVPCAssociationAuthorizationInput{
			HostedZoneId: cr.Spec.ForProvider.HostZoneID,
			VPC: &route53.VPC{
				VPCId:     cr.Spec.ForProvider.PeerVPCID,
				VPCRegion: route53.VPCRegion(*cr.Spec.ForProvider.PeerRegion),
			},
		}
		_, err := e.route53Client.CreateVPCAssociationAuthorizationRequest(vpcAssociationAuthorizationInput).Send(ctx)
		if err != nil && !isAlreadyCreated(err) {
			return managed.ExternalUpdate{}, errors.Wrap(err, errCreateHostzone)
		}
		if cr.Annotations == nil {
			cr.Annotations = map[string]string{}
		}
		cr.Annotations[hostedZoneEnsured] = "true"
		err = e.kube.Update(ctx, cr)

		if err != nil {
			return managed.ExternalUpdate{}, awsclient.Wrap(err, "error update peering annotations")
		}
	}

	cr.Status.SetConditions(Approved())
	return managed.ExternalUpdate{}, errors.Wrap(e.kube.Status().Update(ctx, cr), errUpdateManagedStatus)
}

func isAlreadyCreated(err error) bool {
	if err == nil {
		return false
	}
	if aerr, ok := err.(awserr.Error); ok {
		return strings.Contains(strings.ToLower(aerr.Code()), "already")
	}
	return false
}

func (e *external) Delete(ctx context.Context, mg cpresource.Managed) error { // nolint:gocyclo
	cr, ok := mg.(*svcapitypes.VPCPeeringConnection)
	if !ok {
		return errors.New(errUnexpectedObject)
	}
	cr.Status.SetConditions(xpv1.Deleting())

	_, err := e.route53Client.DeleteVPCAssociationAuthorizationRequest(&route53.DeleteVPCAssociationAuthorizationInput{
		HostedZoneId: cr.Spec.ForProvider.HostZoneID,
		VPC: &route53.VPC{
			VPCId:     cr.Spec.ForProvider.PeerVPCID,
			VPCRegion: route53.VPCRegion(*cr.Spec.ForProvider.PeerRegion),
		},
	}).Send(ctx)
	if err != nil {
		e.log.Info("delete VPCAssociationAuthorization failed", "error", err)
	}

	filter := ec2.Filter{
		Name: aws.String("vpc-id"),
		Values: []string{
			*cr.Spec.ForProvider.VPCID,
		},
	}
	describeRouteTablesInput := &ec2.DescribeRouteTablesInput{
		Filters:    []ec2.Filter{filter},
		MaxResults: aws.Int64(10),
	}
	routeTablesRes, err := e.client.DescribeRouteTablesRequest(describeRouteTablesInput).Send(ctx)
	if err != nil {
		return err
	}

	for _, rt := range routeTablesRes.RouteTables {
		for _, r := range rt.Routes {
			if r.VpcPeeringConnectionId != nil && *r.VpcPeeringConnectionId == *cr.Status.AtProvider.VPCPeeringConnectionID {
				_, err := e.client.DeleteRouteRequest(&ec2.DeleteRouteInput{
					DestinationCidrBlock: cr.Spec.ForProvider.PeerCIDR,

					RouteTableId: rt.RouteTableId,
				}).Send(ctx)
				if err != nil {
					return err
				}
			}
		}
	}

	err = e.deleteVPCPeeringConnection(ctx, cr)

	return err
}

func isAWSErr(err error, code string, message string) bool {
	if err, ok := err.(awserr.Error); ok {
		return err.Code() == code && strings.Contains(err.Message(), message)
	}
	return false
}

func (e *external) deleteVPCPeeringConnection(ctx context.Context, cr *svcapitypes.VPCPeeringConnection) error {
	input := peering.GenerateDescribeVpcPeeringConnectionsInput(cr)
	resp, err := e.client.DescribeVpcPeeringConnectionsRequest(input).Send(ctx)
	if err != nil {
		return err
	}

	for _, peering := range resp.VpcPeeringConnections {
		if peering.Status.Code == ec2.VpcPeeringConnectionStateReasonCodeInitiatingRequest || peering.Status.Code == ec2.VpcPeeringConnectionStateReasonCodePendingAcceptance || peering.Status.Code == ec2.VpcPeeringConnectionStateReasonCodeActive || peering.Status.Code == ec2.VpcPeeringConnectionStateReasonCodeExpired || peering.Status.Code == ec2.VpcPeeringConnectionStateReasonCodeRejected {
			_, err := e.client.DeleteVpcPeeringConnectionRequest(&ec2.DeleteVpcPeeringConnectionInput{
				VpcPeeringConnectionId: peering.VpcPeeringConnectionId,
			}).Send(ctx)

			if err != nil && !isAWSErr(err, "InvalidVpcPeeringConnectionID.NotFound", "") {
				return awsclient.Wrap(err, "errDelete")
			}
		}
	}

	return nil
}

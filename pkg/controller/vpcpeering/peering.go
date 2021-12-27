package vpcpeering

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/route53"

	"github.com/crossplane/provider-aws/pkg/clients/peering"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
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
	awsclient "github.com/crossplane/provider-aws/pkg/clients"
)

// ConnectionStateReasonCode vpc connection state code
type ConnectionStateReasonCode string

const (
	errUnexpectedObject = "managed resource is not an VPCPeeringConnection resource"

	errCreate                         = "cannot create VPCPeeringConnection in AWS"
	errCreateHostzone                 = "cannot create HostedZoneAssosciation in AWS"
	errDescribe                       = "failed to describe VPCPeeringConnection"
	errDescribeRouteTable             = "failed to describe RouteTable"
	errModifyVpcPeering               = "failed to modify VPCPeeringConnection"
	errUpdateManagedStatus            = "cannot update managed resource status"
	errWaitVpcPeeringConnectionAccept = "waiting for the user to accept the vpc peering connection"

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
			RateLimiter:             ratelimiter.NewDefaultManagedRateLimiter(rl),
			MaxConcurrentReconciles: 1,
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
		return managed.ExternalObservation{ResourceExists: false}, errors.Wrap(err, errDescribe)
	}

	e.log.WithValues("VpcPeering", cr.Name).Debug("Describe VpcPeeringConnections")
	// TODO: if user delete vpc peering in aws cloud, how ensure subresource deleted
	if len(resp.VpcPeeringConnections) == 0 {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	existedPeer := resp.VpcPeeringConnections[0]

	if !(existedPeer.Status.Code == ec2.VpcPeeringConnectionStateReasonCodeInitiatingRequest || existedPeer.Status.Code == ec2.VpcPeeringConnectionStateReasonCodePendingAcceptance || existedPeer.Status.Code == ec2.VpcPeeringConnectionStateReasonCodeActive || existedPeer.Status.Code == ec2.VpcPeeringConnectionStateReasonCodeExpired || existedPeer.Status.Code == ec2.VpcPeeringConnectionStateReasonCodeRejected) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	currentPeeringStatus := peering.BuildPeering(resp).Status.AtProvider

	e.log.WithValues("VpcPeering", cr.Name).Debug("Build current peering status")

	// update current peering status to status.atProvider
	if !reflect.DeepEqual(currentPeeringStatus, cr.Status.AtProvider) {
		currentPeeringStatus.DeepCopyInto(&cr.Status.AtProvider)
		if err := e.kube.Status().Update(ctx, cr); err != nil {
			return managed.ExternalObservation{ResourceExists: true}, err
		}
	}

	// If vpc peering connection status is pending acceptance, modify vpc peering attributes request will failed.
	// In order to reduce the API request to AWS, return errors early to avoid unnecessary API requests.
	if existedPeer.Status.Code == ec2.VpcPeeringConnectionStateReasonCodePendingAcceptance && !meta.WasDeleted(cr) {
		return managed.ExternalObservation{ResourceExists: true}, fmt.Errorf(errWaitVpcPeeringConnectionAccept)
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
		return managed.ExternalCreation{}, errors.Wrap(err, "create VpcPeeringConnection")
	}

	e.log.WithValues("VpcPeering", cr.Name).Debug("Create VpcPeeringConnectio successful")

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
		return managed.ExternalCreation{}, errors.Wrap(err, "create tag for vpc peering")
	}

	e.log.WithValues("VpcPeering", cr.Name).Debug("Create tag for vpc peering successful")

	meta.SetExternalName(cr, aws.StringValue(resp.VpcPeeringConnection.VpcPeeringConnectionId))

	return managed.ExternalCreation{ExternalNameAssigned: true}, nil
}

func (e *external) Update(ctx context.Context, mg cpresource.Managed) (managed.ExternalUpdate, error) { // nolint:gocyclo
	cr, ok := mg.(*svcapitypes.VPCPeeringConnection)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errUnexpectedObject)
	}

	_, routeTableReady := cr.GetAnnotations()[routeTableEnsured]
	_, hostZoneReady := cr.GetAnnotations()[hostedZoneEnsured]
	_, attributeReady := cr.GetAnnotations()[attributeModified]

	if !attributeReady && cr.Status.AtProvider.VPCPeeringConnectionID != nil {
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
			return managed.ExternalUpdate{}, errors.Wrap(err, "error update peering annotations")
		}

		e.log.WithValues("VpcPeering", cr.Name).Debug("Modify VpcPeeringConnection successful")
	}

	if !routeTableReady && cr.Status.AtProvider.VPCPeeringConnectionID != nil {
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
			return managed.ExternalUpdate{}, errors.Wrap(err, errDescribeRouteTable)
		}

		e.log.WithValues("VpcPeering", cr.Name).Debug("Describe RouteTables for creating")

		for _, rt := range routeTablesRes.RouteTables {
			createRouteInput := &ec2.CreateRouteInput{
				RouteTableId:           rt.RouteTableId,
				DestinationCidrBlock:   cr.Spec.ForProvider.PeerCIDR,
				VpcPeeringConnectionId: cr.Status.AtProvider.VPCPeeringConnectionID,
			}
			_, err := e.client.CreateRouteRequest(createRouteInput).Send(ctx)
			if err != nil {
				// FIXME: The error is not aws.Err type?
				if !strings.Contains(err.Error(), "RouteAlreadyExists") {
					return managed.ExternalUpdate{}, errors.Wrap(err, "create route for vpc peering")
				} else {

					for _, route := range rt.Routes {
						// The route identified by DestinationCidrBlock, if route table already have DestinationCidrBlock point to other vpc peering connetion ID, should be return error
						if route.DestinationCidrBlock != nil && *route.DestinationCidrBlock == *cr.Spec.ForProvider.PeerCIDR {
							if route.VpcPeeringConnectionId != nil && *route.VpcPeeringConnectionId == *cr.Status.AtProvider.VPCPeeringConnectionID {
								e.log.WithValues("VpcPeering", cr.Name).Debug("Route already exist, no need to recreate", "RouteTableId", rt.RouteTableId, "DestinationCidrBlock", *route.DestinationCidrBlock)
								continue
							} else {
								return managed.ExternalUpdate{}, errors.Wrap(err, fmt.Sprintf("failed add route for vpc peering connection: %s, routeID: %s", *cr.Status.AtProvider.VPCPeeringConnectionID, *rt.RouteTableId))
							}
						}
					}
				}
			} else {
				e.log.WithValues("VpcPeering", cr.Name).Debug("Create Route successful", "RouteTableId", rt.RouteTableId)
			}
		}
		if cr.Annotations == nil {
			cr.Annotations = map[string]string{}
		}
		cr.Annotations[routeTableEnsured] = "true"
		err = e.kube.Update(ctx, cr)
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, "error update peering annotations")
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

		e.log.WithValues("VpcPeering", cr.Name).Debug("Create VPCAssociationAuthorization successful")

		if cr.Annotations == nil {
			cr.Annotations = map[string]string{}
		}
		cr.Annotations[hostedZoneEnsured] = "true"
		err = e.kube.Update(ctx, cr)

		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, "error update peering annotations")
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
	if err != nil && !strings.Contains(err.Error(), "VPCAssociationAuthorizationNotFound") {
		return errors.Wrap(err, "delete VPCAssociationAuthorization")
	}
	e.log.WithValues("VpcPeering", cr.Name).Debug("Delete VPCAssociationAuthorization successful")

	filter := ec2.Filter{
		Name: aws.String("vpc-id"),
		Values: []string{
			*cr.Spec.ForProvider.VPCID,
		},
	}
	describeRouteTablesInput := &ec2.DescribeRouteTablesInput{
		Filters: []ec2.Filter{filter},
	}
	routeTablesRes, err := e.client.DescribeRouteTablesRequest(describeRouteTablesInput).Send(ctx)
	if err != nil {
		return errors.Wrap(err, "describe RouteTables")
	}

	e.log.WithValues("VpcPeering", cr.Name).Debug("Describe RouteTables for deleting", "result", routeTablesRes.String())
	for _, rt := range routeTablesRes.RouteTables {
		for _, r := range rt.Routes {
			if r.VpcPeeringConnectionId != nil && cr.Status.AtProvider.VPCPeeringConnectionID != nil && *r.VpcPeeringConnectionId == *cr.Status.AtProvider.VPCPeeringConnectionID {
				_, err := e.client.DeleteRouteRequest(&ec2.DeleteRouteInput{
					DestinationCidrBlock: cr.Spec.ForProvider.PeerCIDR,

					RouteTableId: rt.RouteTableId,
				}).Send(ctx)
				if err != nil {
					return errors.Wrap(err, "delete Route")
				}
				e.log.WithValues("VpcPeering", cr.Name).Debug("Delete route successful", "RouteTableId", rt.RouteTableId)
			}
		}
	}

	err = e.deleteVPCPeeringConnection(ctx, cr)

	return errors.Wrap(err, "delete VPCPeeringConnection")
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
				return errors.Wrap(err, "delete vpc peering connection")
			}
			e.log.WithValues("VpcPeering", cr.Name).Debug("Delete VpcPeeringConnection successful")
		}
	}

	return nil
}

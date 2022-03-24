package vpcpeering

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	"github.com/crossplane/provider-aws/pkg/clients/peering"

	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
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

	// The maximum number of results to return with a single call
	masResults = 100
)

// SetupVPCPeeringConnection adds a controller that reconciles VPCPeeringConnection.
func SetupVPCPeeringConnection(mgr ctrl.Manager, l logging.Logger, rl workqueue.RateLimiter) error {
	name := managed.ControllerName(svcapitypes.VPCPeeringConnectionGroupKind)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewMaxOfRateLimiter(
				workqueue.NewItemExponentialFailureRateLimiter(10*time.Second, 120*time.Second),
				rl,
			),
			MaxConcurrentReconciles: 1,
		}).
		For(&svcapitypes.VPCPeeringConnection{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(svcapitypes.VPCPeeringConnectionGroupVersionKind),
			managed.WithExternalConnecter(&connector{kube: mgr.GetClient(), log: l}),
			// resync interval default is 1 minute, may cause rate limit
			managed.WithPollInterval(5*time.Minute),
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

	external := &external{
		kube:          c.kube,
		route53Client: peering.NewRoute53Client(*cfg),
		client:        peering.NewEc2Client(*cfg),
		stsClient:     peering.NewStsClient(*cfg),
		log:           c.log,
	}
	isInternal, err := external.isInternalVpcPeering(ctx, cr)
	if err != nil {
		return nil, err
	}
	if isInternal {
		external.isInternal = true
		peerCfg, err := awsclient.GetConfig(ctx, c.kube, mg, *cr.Spec.ForProvider.PeerRegion)
		if err != nil {
			return nil, err
		}
		external.peerClient = peering.NewEc2Client(*peerCfg)
	}
	return external, nil
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
	stsClient     peering.StsClient
	peerClient    peering.EC2Client
	isInternal    bool
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
	resp, err := e.client.DescribeVpcPeeringConnections(ctx, input)
	if err != nil {
		return managed.ExternalObservation{ResourceExists: false}, errors.Wrap(err, errDescribe)
	}

	e.log.WithValues("VpcPeering", cr.Name).Debug("Describe VpcPeeringConnections")
	// TODO: if user delete vpc peering in aws cloud, how ensure subresource deleted
	if len(resp.VpcPeeringConnections) == 0 {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	existedPeer := resp.VpcPeeringConnections[0]

	if !(existedPeer.Status.Code == ec2types.VpcPeeringConnectionStateReasonCodeInitiatingRequest ||
		existedPeer.Status.Code == ec2types.VpcPeeringConnectionStateReasonCodePendingAcceptance ||
		existedPeer.Status.Code == ec2types.VpcPeeringConnectionStateReasonCodeActive ||
		existedPeer.Status.Code == ec2types.VpcPeeringConnectionStateReasonCodeExpired ||
		existedPeer.Status.Code == ec2types.VpcPeeringConnectionStateReasonCodeRejected ||
		existedPeer.Status.Code == ec2types.VpcPeeringConnectionStateReasonCodeFailed) {
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

	if existedPeer.Status.Code == ec2types.VpcPeeringConnectionStateReasonCodeRejected || existedPeer.Status.Code == ec2types.VpcPeeringConnectionStateReasonCodeFailed {
		cr.Status.SetConditions(xpv1.Unavailable())
		err := e.kube.Status().Update(ctx, cr)
		if err != nil {
			return managed.ExternalObservation{
				ResourceExists: true,
			}, err
		}
		// TODO: Actually we don't need to reconcile this object, but the crossplane runtime cannot forgot it from queue.
		// Fortunately AWS has an expiration time，it will eventually be removed
		return managed.ExternalObservation{
			ResourceExists: true,
			// Peering options can be added only to active peerings. so we need not call Update function
		}, fmt.Errorf("Peering %s is not active", *existedPeer.VpcPeeringConnectionId)
	}

	if existedPeer.Status.Code == ec2types.VpcPeeringConnectionStateReasonCodePendingAcceptance && !meta.WasDeleted(cr) {
		// if requester peering connection acountID same with accepter vpc peering connection, auto-accept it
		if e.isInternal {
			// accept it when vpc peering connection created
			if cr.Status.AtProvider.VPCPeeringConnectionID != nil {
				_, err := e.peerClient.AcceptVpcPeeringConnection(ctx, &ec2.AcceptVpcPeeringConnectionInput{VpcPeeringConnectionId: aws.String(*cr.Status.AtProvider.VPCPeeringConnectionID)})
				if err != nil {
					return managed.ExternalObservation{ResourceExists: true}, err
				}
			}
		} else {
			// let peering status change from available to unavailable, sometimes user can delete peering in aws provider
			cr.Status.SetConditions(xpv1.Unavailable())
			// If vpc peering connection status is pending acceptance, modify vpc peering attributes request will failed.
			// In order to reduce the API request to AWS, return errors early to avoid unnecessary API requests.
			return managed.ExternalObservation{ResourceExists: true}, fmt.Errorf(errWaitVpcPeeringConnectionAccept)
		}
	}

	_, routeTableReady := cr.GetAnnotations()[routeTableEnsured]
	_, hostZoneReady := cr.GetAnnotations()[hostedZoneEnsured]
	_, attributeReady := cr.GetAnnotations()[attributeModified]
	if !routeTableReady || !attributeReady {
		return managed.ExternalObservation{
			ResourceExists: true,
			// vpc peering post processing not complete, forward to Update()
			ResourceUpToDate: false,
		}, errors.Wrap(e.kube.Status().Update(ctx, cr), errUpdateManagedStatus)
	}
	if !hostZoneReady {
		e.log.WithValues("VpcPeering", cr.Name).Debug("Skip setting optional hosted zone")
	}

	if existedPeer.Status.Code == ec2types.VpcPeeringConnectionStateReasonCodeActive && cr.GetCondition(ApprovedCondition).Status == corev1.ConditionTrue {
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

	resp, err := e.client.CreateVpcPeeringConnection(ctx, input)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "create VpcPeeringConnection")
	}

	e.log.WithValues("VpcPeering", cr.Name).Debug("Create VpcPeeringConnection successful")

	tags := make([]ec2types.Tag, 0)
	tags = append(tags, ec2types.Tag{
		Key:   aws.String("Name"),
		Value: aws.String(cr.ObjectMeta.Name),
	})

	for _, tag := range cr.Spec.ForProvider.Tags {
		tags = append(tags, ec2types.Tag{
			Key:   tag.Key,
			Value: tag.Value,
		})
	}

	_, err = e.client.CreateTags(ctx, &ec2.CreateTagsInput{
		Resources: []string{
			*resp.VpcPeeringConnection.VpcPeeringConnectionId,
		},
		Tags: tags,
	})
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
		_, err := e.client.ModifyVpcPeeringConnectionOptions(ctx, &ec2.ModifyVpcPeeringConnectionOptionsInput{
			VpcPeeringConnectionId: cr.Status.AtProvider.VPCPeeringConnectionID,
			RequesterPeeringConnectionOptions: &ec2types.PeeringConnectionOptionsRequest{
				AllowDnsResolutionFromRemoteVpc: aws.Bool(true),
			},
		})
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, errModifyVpcPeering)
		}

		if e.isInternal {
			_, err := e.peerClient.ModifyVpcPeeringConnectionOptions(ctx, &ec2.ModifyVpcPeeringConnectionOptionsInput{
				VpcPeeringConnectionId: cr.Status.AtProvider.VPCPeeringConnectionID,
				AccepterPeeringConnectionOptions: &ec2types.PeeringConnectionOptionsRequest{
					AllowDnsResolutionFromRemoteVpc: aws.Bool(true),
				},
			})
			if err != nil {
				return managed.ExternalUpdate{}, errors.Wrap(err, errModifyVpcPeering)
			}
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
		err := e.addRoute(ctx, e.client, cr.Name, []string{*cr.Spec.ForProvider.VPCID}, *cr.Spec.ForProvider.PeerCIDR, cr.Status.AtProvider.VPCPeeringConnectionID)
		if err != nil {
			return managed.ExternalUpdate{}, err
		}
		if e.isInternal {
			if cr.Status.AtProvider.RequesterVPCInfo != nil && cr.Status.AtProvider.RequesterVPCInfo.CIDRBlock != nil {
				err := e.addRoute(ctx, e.peerClient, cr.Name, []string{*cr.Spec.ForProvider.PeerVPCID}, *cr.Status.AtProvider.RequesterVPCInfo.CIDRBlock, cr.Status.AtProvider.VPCPeeringConnectionID)
				if err != nil {
					return managed.ExternalUpdate{}, err
				}
			} else {
				return managed.ExternalUpdate{}, fmt.Errorf("requester vpc cidr is null")
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

	// // hostZoneID is optional
	if !hostZoneReady && cr.Spec.ForProvider.HostZoneID != nil {
		vpcAssociationAuthorizationInput := &route53.CreateVPCAssociationAuthorizationInput{
			HostedZoneId: cr.Spec.ForProvider.HostZoneID,
			VPC: &route53types.VPC{
				VPCId:     cr.Spec.ForProvider.PeerVPCID,
				VPCRegion: route53types.VPCRegion(*cr.Spec.ForProvider.PeerRegion),
			},
		}
		_, err := e.route53Client.CreateVPCAssociationAuthorization(ctx, vpcAssociationAuthorizationInput)
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

	// hostZoneID is optional
	if cr.Spec.ForProvider.HostZoneID != nil {
		_, err := e.route53Client.DeleteVPCAssociationAuthorization(ctx, &route53.DeleteVPCAssociationAuthorizationInput{
			HostedZoneId: cr.Spec.ForProvider.HostZoneID,
			VPC: &route53types.VPC{
				VPCId:     cr.Spec.ForProvider.PeerVPCID,
				VPCRegion: route53types.VPCRegion(*cr.Spec.ForProvider.PeerRegion),
			},
		})
		if err != nil && !strings.Contains(err.Error(), "VPCAssociationAuthorizationNotFound") {
			return errors.Wrap(err, "delete VPCAssociationAuthorization")
		}
		e.log.WithValues("VpcPeering", cr.Name).Debug("Delete VPCAssociationAuthorization successful")
	}

	if cr.Status.AtProvider.VPCPeeringConnectionID != nil {
		err := deleteRoute(ctx, e.log, e.client, cr.Name, []string{*cr.Spec.ForProvider.VPCID}, *cr.Spec.ForProvider.PeerCIDR, cr.Status.AtProvider.VPCPeeringConnectionID)
		if err != nil {
			return err
		}
		if e.isInternal {
			if cr.Status.AtProvider.RequesterVPCInfo != nil && cr.Status.AtProvider.RequesterVPCInfo.CIDRBlock != nil {
				err := deleteRoute(ctx, e.log, e.peerClient, cr.Name, []string{*cr.Spec.ForProvider.PeerVPCID}, *cr.Status.AtProvider.RequesterVPCInfo.CIDRBlock, cr.Status.AtProvider.VPCPeeringConnectionID)
				if err != nil {
					return err
				}
			} else {
				return fmt.Errorf("requester vpc cidr is null")
			}
		}
	}

	err := e.deleteVPCPeeringConnection(ctx, cr)

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
	resp, err := e.client.DescribeVpcPeeringConnections(ctx, input)
	if err != nil {
		return err
	}

	for _, peering := range resp.VpcPeeringConnections {
		if peering.Status.Code == ec2types.VpcPeeringConnectionStateReasonCodeInitiatingRequest || peering.Status.Code == ec2types.VpcPeeringConnectionStateReasonCodePendingAcceptance || peering.Status.Code == ec2types.VpcPeeringConnectionStateReasonCodeActive || peering.Status.Code == ec2types.VpcPeeringConnectionStateReasonCodeExpired || peering.Status.Code == ec2types.VpcPeeringConnectionStateReasonCodeRejected {
			_, err := e.client.DeleteVpcPeeringConnection(ctx, &ec2.DeleteVpcPeeringConnectionInput{
				VpcPeeringConnectionId: peering.VpcPeeringConnectionId,
			})

			if err != nil && !isAWSErr(err, "InvalidVpcPeeringConnectionID.NotFound", "") {
				return errors.Wrap(err, "delete vpc peering connection")
			}
			e.log.WithValues("VpcPeering", cr.Name).Debug("Delete VpcPeeringConnection successful")
		}
	}

	return nil
}

func (e *external) isInternalVpcPeering(ctx context.Context, cr *svcapitypes.VPCPeeringConnection) (bool, error) {
	input := &sts.GetCallerIdentityInput{}
	res, err := e.stsClient.GetCallerIdentity(ctx, input)
	if err != nil {
		return false, err
	}
	// if requester peering connection acountID same with accepter vpc peering connection, auto-accept it
	if cr.Spec.ForProvider.PeerOwnerID != nil && res.Account != nil && *res.Account == *cr.Spec.ForProvider.PeerOwnerID {
		return true, nil
	}

	return false, nil
}

func (e *external) addRoute(ctx context.Context, client peering.EC2Client, name string, vpcIDs []string, peerCIDR string, pcx *string) error {
	filter := ec2types.Filter{
		Name:   aws.String("vpc-id"),
		Values: vpcIDs,
	}
	describeRouteTablesInput := &ec2.DescribeRouteTablesInput{
		Filters:    []ec2types.Filter{filter},
		MaxResults: aws.Int32(masResults),
	}
	routeTablesRes, err := client.DescribeRouteTables(ctx, describeRouteTablesInput)
	if err != nil {
		return errors.Wrap(err, errDescribeRouteTable)
	}

	e.log.WithValues("VpcPeering", name).Debug("Describe RouteTables for creating")

	for _, rt := range routeTablesRes.RouteTables {
		createRouteInput := &ec2.CreateRouteInput{
			RouteTableId:           rt.RouteTableId,
			DestinationCidrBlock:   aws.String(peerCIDR),
			VpcPeeringConnectionId: pcx,
		}

		// If the route table exists, and the state is blackhole, remove it.
		for _, route := range rt.Routes {
			if route.DestinationCidrBlock != nil && *route.DestinationCidrBlock == peerCIDR && route.State == ec2types.RouteStateBlackhole {
				_, err := client.DeleteRoute(ctx, &ec2.DeleteRouteInput{
					DestinationCidrBlock: aws.String(peerCIDR),
					RouteTableId:         rt.RouteTableId,
				})
				if err != nil {
					if !strings.Contains(err.Error(), "InvalidRoute.NotFound") {
						return errors.Wrap(err, "delete Route")
					}
				}
				e.log.WithValues("VpcPeering", name).Debug("Delete the occupied route whose route is in blackhole successful", "RouteTableId", rt.RouteTableId)
			}
		}

		_, err := client.CreateRoute(ctx, createRouteInput)
		if err != nil {
			// FIXME: The error is not aws.Err type?
			if !strings.Contains(err.Error(), "RouteAlreadyExists") {
				return errors.Wrap(err, "create route for vpc peering")
			} else {
				for _, route := range rt.Routes {
					// The route identified by DestinationCidrBlock, if route table already have DestinationCidrBlock point to other vpc peering connetion ID, should be return error
					if route.DestinationCidrBlock != nil && *route.DestinationCidrBlock == peerCIDR {
						if route.VpcPeeringConnectionId != nil && *route.VpcPeeringConnectionId == *pcx {
							e.log.WithValues("VpcPeering", name).Debug("Route already exist, no need to recreate", "RouteTableId", rt.RouteTableId, "DestinationCidrBlock", *route.DestinationCidrBlock)
							continue
						} else {
							return errors.Wrap(err, fmt.Sprintf("failed add route for vpc peering connection: %s, routeID: %s", *pcx, *rt.RouteTableId))
						}
					}
				}
			}
		} else {
			e.log.WithValues("VpcPeering", name).Debug("Create Route successful", "RouteTableId", rt.RouteTableId)
		}
	}
	return nil
}

func deleteRoute(ctx context.Context, log logging.Logger, client peering.EC2Client, name string, vpcIDs []string, peerCIDR string, pcx *string) error {
	filter := ec2types.Filter{
		Name:   aws.String("vpc-id"),
		Values: vpcIDs,
	}
	describeRouteTablesInput := &ec2.DescribeRouteTablesInput{
		Filters:    []ec2types.Filter{filter},
		MaxResults: aws.Int32(masResults),
	}
	routeTablesRes, err := client.DescribeRouteTables(ctx, describeRouteTablesInput)
	if err != nil {
		return errors.Wrap(err, "describe RouteTables")
	}

	for _, rt := range routeTablesRes.RouteTables {
		for _, r := range rt.Routes {
			if r.VpcPeeringConnectionId != nil && pcx != nil && *r.VpcPeeringConnectionId == *pcx {
				_, err := client.DeleteRoute(ctx, &ec2.DeleteRouteInput{
					DestinationCidrBlock: aws.String(peerCIDR),
					RouteTableId:         rt.RouteTableId,
				})
				if err != nil {
					if !strings.Contains(err.Error(), "InvalidRoute.NotFound") {
						return errors.Wrap(err, "delete Route")
					}
				}
				log.WithValues("VpcPeering", name).Debug("Delete route successful", "RouteTableId", rt.RouteTableId)
			}
		}
	}
	return nil
}

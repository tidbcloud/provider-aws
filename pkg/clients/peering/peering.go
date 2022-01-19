package peering

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	aws "github.com/crossplane/provider-aws/pkg/clients"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	svcapitypes "github.com/crossplane/provider-aws/apis/vpcpeering/v1alpha1"
)

// NOTE(muvaf): We return pointers in case the function needs to start with an
// empty object, hence need to return a new pointer.

// GenerateDescribeVpcPeeringConnectionsInput returns input for read
// operation.
func GenerateDescribeVpcPeeringConnectionsInput(cr *svcapitypes.VPCPeeringConnection) *ec2.DescribeVpcPeeringConnectionsInput {
	res := &ec2.DescribeVpcPeeringConnectionsInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: []string{cr.ObjectMeta.Name},
			},
			{
				Name: aws.String("status-code"),
				Values: []string{
					string(ec2types.VpcPeeringConnectionStateReasonCodeInitiatingRequest),
					string(ec2types.VpcPeeringConnectionStateReasonCodeActive),
					string(ec2types.VpcPeeringConnectionStateReasonCodePendingAcceptance),
					string(ec2types.VpcPeeringConnectionStateReasonCodeProvisioning),
					// If the vpc id does not exist, the status of the vpc peering connection will become failed.
					// Describe result should contains failed vpc peering connection, otherwise the controller will always create peering and eventually api throttling
					string(ec2types.VpcPeeringConnectionStateReasonCodeFailed),
					// If peer vpc reject connection, we should not create it again.
					string(ec2types.VpcPeeringConnectionStateReasonCodeRejected),
				},
			},
		},
	}

	return res
}

// BuildPeering returns the current state in the form of *svcapitypes.VPCPeeringConnection.
func BuildPeering(resp *ec2.DescribeVpcPeeringConnectionsOutput) *svcapitypes.VPCPeeringConnection { // nolint:gocyclo
	cr := &svcapitypes.VPCPeeringConnection{}

	output := resp
	if output == nil {
		return cr
	}

	for _, elem := range output.VpcPeeringConnections {
		if elem.AccepterVpcInfo != nil {
			f0 := &svcapitypes.VPCPeeringConnectionVPCInfo{}
			if elem.AccepterVpcInfo.CidrBlock != nil {
				f0.CIDRBlock = elem.AccepterVpcInfo.CidrBlock
			}
			if elem.AccepterVpcInfo.CidrBlockSet != nil {
				f0f1 := []*svcapitypes.CIDRBlock{}
				for _, f0f1iter := range elem.AccepterVpcInfo.CidrBlockSet {
					f0f1elem := &svcapitypes.CIDRBlock{}
					if f0f1iter.CidrBlock != nil {
						f0f1elem.CIDRBlock = f0f1iter.CidrBlock
					}
					f0f1 = append(f0f1, f0f1elem)
				}
				f0.CIDRBlockSet = f0f1
			}
			if elem.AccepterVpcInfo.Ipv6CidrBlockSet != nil {
				f0f2 := []*svcapitypes.IPv6CIDRBlock{}
				for _, f0f2iter := range elem.AccepterVpcInfo.Ipv6CidrBlockSet {
					f0f2elem := &svcapitypes.IPv6CIDRBlock{}
					if f0f2iter.Ipv6CidrBlock != nil {
						f0f2elem.IPv6CIDRBlock = f0f2iter.Ipv6CidrBlock
					}
					f0f2 = append(f0f2, f0f2elem)
				}
				f0.IPv6CIDRBlockSet = f0f2
			}
			if elem.AccepterVpcInfo.OwnerId != nil {
				f0.OwnerID = elem.AccepterVpcInfo.OwnerId
			}
			if elem.AccepterVpcInfo.PeeringOptions != nil {
				f0f4 := &svcapitypes.VPCPeeringConnectionOptionsDescription{}
				if elem.AccepterVpcInfo.PeeringOptions.AllowDnsResolutionFromRemoteVpc != nil {
					f0f4.AllowDNSResolutionFromRemoteVPC = elem.AccepterVpcInfo.PeeringOptions.AllowDnsResolutionFromRemoteVpc
				}
				if elem.AccepterVpcInfo.PeeringOptions.AllowEgressFromLocalClassicLinkToRemoteVpc != nil {
					f0f4.AllowEgressFromLocalClassicLinkToRemoteVPC = elem.AccepterVpcInfo.PeeringOptions.AllowEgressFromLocalClassicLinkToRemoteVpc
				}
				if elem.AccepterVpcInfo.PeeringOptions.AllowEgressFromLocalVpcToRemoteClassicLink != nil {
					f0f4.AllowEgressFromLocalVPCToRemoteClassicLink = elem.AccepterVpcInfo.PeeringOptions.AllowEgressFromLocalVpcToRemoteClassicLink
				}
				f0.PeeringOptions = f0f4
			}
			if elem.AccepterVpcInfo.Region != nil {
				f0.Region = elem.AccepterVpcInfo.Region
			}
			if elem.AccepterVpcInfo.VpcId != nil {
				f0.VPCID = elem.AccepterVpcInfo.VpcId
			}
			cr.Status.AtProvider.AccepterVPCInfo = f0
		} else {
			cr.Status.AtProvider.AccepterVPCInfo = nil
		}
		if elem.ExpirationTime != nil {
			cr.Status.AtProvider.ExpirationTime = &metav1.Time{
				Time: *elem.ExpirationTime,
			}
		} else {
			cr.Status.AtProvider.ExpirationTime = nil
		}
		if elem.RequesterVpcInfo != nil {
			f2 := &svcapitypes.VPCPeeringConnectionVPCInfo{}
			if elem.RequesterVpcInfo.CidrBlock != nil {
				f2.CIDRBlock = elem.RequesterVpcInfo.CidrBlock
			}
			if elem.RequesterVpcInfo.CidrBlockSet != nil {
				f2f1 := []*svcapitypes.CIDRBlock{}
				for _, f2f1iter := range elem.RequesterVpcInfo.CidrBlockSet {
					f2f1elem := &svcapitypes.CIDRBlock{}
					if f2f1iter.CidrBlock != nil {
						f2f1elem.CIDRBlock = f2f1iter.CidrBlock
					}
					f2f1 = append(f2f1, f2f1elem)
				}
				f2.CIDRBlockSet = f2f1
			}
			if elem.RequesterVpcInfo.Ipv6CidrBlockSet != nil {
				f2f2 := []*svcapitypes.IPv6CIDRBlock{}
				for _, f2f2iter := range elem.RequesterVpcInfo.Ipv6CidrBlockSet {
					f2f2elem := &svcapitypes.IPv6CIDRBlock{}
					if f2f2iter.Ipv6CidrBlock != nil {
						f2f2elem.IPv6CIDRBlock = f2f2iter.Ipv6CidrBlock
					}
					f2f2 = append(f2f2, f2f2elem)
				}
				f2.IPv6CIDRBlockSet = f2f2
			}
			if elem.RequesterVpcInfo.OwnerId != nil {
				f2.OwnerID = elem.RequesterVpcInfo.OwnerId
			}
			if elem.RequesterVpcInfo.PeeringOptions != nil {
				f2f4 := &svcapitypes.VPCPeeringConnectionOptionsDescription{}
				if elem.RequesterVpcInfo.PeeringOptions.AllowDnsResolutionFromRemoteVpc != nil {
					f2f4.AllowDNSResolutionFromRemoteVPC = elem.RequesterVpcInfo.PeeringOptions.AllowDnsResolutionFromRemoteVpc
				}
				if elem.RequesterVpcInfo.PeeringOptions.AllowEgressFromLocalClassicLinkToRemoteVpc != nil {
					f2f4.AllowEgressFromLocalClassicLinkToRemoteVPC = elem.RequesterVpcInfo.PeeringOptions.AllowEgressFromLocalClassicLinkToRemoteVpc
				}
				if elem.RequesterVpcInfo.PeeringOptions.AllowEgressFromLocalVpcToRemoteClassicLink != nil {
					f2f4.AllowEgressFromLocalVPCToRemoteClassicLink = elem.RequesterVpcInfo.PeeringOptions.AllowEgressFromLocalVpcToRemoteClassicLink
				}
				f2.PeeringOptions = f2f4
			}
			if elem.RequesterVpcInfo.Region != nil {
				f2.Region = elem.RequesterVpcInfo.Region
			}
			if elem.RequesterVpcInfo.VpcId != nil {
				f2.VPCID = elem.RequesterVpcInfo.VpcId
			}
			cr.Status.AtProvider.RequesterVPCInfo = f2
		} else {
			cr.Status.AtProvider.RequesterVPCInfo = nil
		}
		if elem.Status != nil {
			f3 := &svcapitypes.VPCPeeringConnectionStateReason{}
			f3.Code = aws.String(string(elem.Status.Code))
			if elem.Status.Message != nil {
				f3.Message = elem.Status.Message
			}
			cr.Status.AtProvider.Status = f3
		} else {
			cr.Status.AtProvider.Status = nil
		}
		if elem.Tags != nil {
			f4 := []*svcapitypes.Tag{}
			for _, f4iter := range elem.Tags {
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
		if elem.VpcPeeringConnectionId != nil {
			cr.Status.AtProvider.VPCPeeringConnectionID = elem.VpcPeeringConnectionId
		} else {
			cr.Status.AtProvider.VPCPeeringConnectionID = nil
		}
	}

	return cr
}

// GenerateCreateVpcPeeringConnectionInput returns a create input.
func GenerateCreateVpcPeeringConnectionInput(cr *svcapitypes.VPCPeeringConnection) *ec2.CreateVpcPeeringConnectionInput {
	res := &ec2.CreateVpcPeeringConnectionInput{}

	if cr.Spec.ForProvider.PeerOwnerID != nil {
		res.PeerOwnerId = cr.Spec.ForProvider.PeerOwnerID
	}
	if cr.Spec.ForProvider.PeerRegion != nil {
		res.PeerRegion = cr.Spec.ForProvider.PeerRegion
	}
	if cr.Spec.ForProvider.PeerVPCID != nil {
		res.PeerVpcId = cr.Spec.ForProvider.PeerVPCID
	}
	if cr.Spec.ForProvider.VPCID != nil {
		res.VpcId = cr.Spec.ForProvider.VPCID
	}

	return res
}

// EC2Client ec2 client
type EC2Client interface {
	// DescribeVpcPeeringConnectionsRequest describe vpc peering connection
	DescribeVpcPeeringConnections(context.Context, *ec2.DescribeVpcPeeringConnectionsInput, ...func(*ec2.Options)) (*ec2.DescribeVpcPeeringConnectionsOutput, error)
	// CreateVpcPeeringConnectionRequest create vpc peering connection
	CreateVpcPeeringConnection(context.Context, *ec2.CreateVpcPeeringConnectionInput, ...func(*ec2.Options)) (*ec2.CreateVpcPeeringConnectionOutput, error)
	// CreateTagsRequest create tags for vpc peering
	CreateTags(context.Context, *ec2.CreateTagsInput, ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)
	// DescribeRouteTablesRequest describe route table
	DescribeRouteTables(context.Context, *ec2.DescribeRouteTablesInput, ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error)
	// CreateRouteRequest create route
	CreateRoute(context.Context, *ec2.CreateRouteInput, ...func(*ec2.Options)) (*ec2.CreateRouteOutput, error)
	// DeleteRouteRequest delete route
	DeleteRoute(context.Context, *ec2.DeleteRouteInput, ...func(*ec2.Options)) (*ec2.DeleteRouteOutput, error)
	// ModifyVpcPeeringConnectionOptionsRequest motify vpc peering
	ModifyVpcPeeringConnectionOptions(context.Context, *ec2.ModifyVpcPeeringConnectionOptionsInput, ...func(*ec2.Options)) (*ec2.ModifyVpcPeeringConnectionOptionsOutput, error)
	// DeleteVpcPeeringConnectionRequest delete vpc peering
	DeleteVpcPeeringConnection(context.Context, *ec2.DeleteVpcPeeringConnectionInput, ...func(*ec2.Options)) (*ec2.DeleteVpcPeeringConnectionOutput, error)
	// AcceptVpcPeeringConnectionRequest accept vpc peering connection
	AcceptVpcPeeringConnection(context.Context, *ec2.AcceptVpcPeeringConnectionInput, ...func(*ec2.Options)) (*ec2.AcceptVpcPeeringConnectionOutput, error)
}

// NewEc2Client create ec2 client
func NewEc2Client(cfg sdkaws.Config) EC2Client {
	return ec2.NewFromConfig(cfg)
}

// NewRoute53Client create route53 client
func NewRoute53Client(cfg sdkaws.Config) Route53Client {
	return route53.NewFromConfig(cfg)
}

// Route53Client route53 client
type Route53Client interface {
	CreateVPCAssociationAuthorization(context.Context, *route53.CreateVPCAssociationAuthorizationInput, ...func(*route53.Options)) (*route53.CreateVPCAssociationAuthorizationOutput, error)
	DeleteVPCAssociationAuthorization(context.Context, *route53.DeleteVPCAssociationAuthorizationInput, ...func(*route53.Options)) (*route53.DeleteVPCAssociationAuthorizationOutput, error)
}

type StsClient interface {
	GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

func NewStsClient(cfg sdkaws.Config) StsClient {
	return sts.NewFromConfig(cfg)
}

package fake

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/aws/aws-sdk-go-v2/service/route53"
)

// MockEC2Client mock ec2 client
type MockEC2Client struct {
	// DescribeVpcPeeringConnectionsRequestFun
	DescribeVpcPeeringConnectionsRequestFun func(ctx context.Context, input *ec2.DescribeVpcPeeringConnectionsInput, opts ...func(*ec2.Options)) (*ec2.DescribeVpcPeeringConnectionsOutput, error)
	// CreateVpcPeeringConnectionRequestFun
	CreateVpcPeeringConnectionRequestFun func(ctx context.Context, input *ec2.CreateVpcPeeringConnectionInput, opts ...func(*ec2.Options)) (*ec2.CreateVpcPeeringConnectionOutput, error)
	// CreateRouteRequestFun
	CreateRouteRequestFun func(ctx context.Context, input *ec2.CreateRouteInput, opts ...func(*ec2.Options)) (*ec2.CreateRouteOutput, error)
	// DescribeRouteTablesRequestFun
	DescribeRouteTablesRequestFun func(ctx context.Context, input *ec2.DescribeRouteTablesInput, opts ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error)
	// DeleteRouteRequestFun
	DeleteRouteRequestFun func(ctx context.Context, input *ec2.DeleteRouteInput, opts ...func(*ec2.Options)) (*ec2.DeleteRouteOutput, error)
	// ModifyVpcPeeringConnectionOptionsRequestFun
	ModifyVpcPeeringConnectionOptionsRequestFun func(ctx context.Context, input *ec2.ModifyVpcPeeringConnectionOptionsInput, opts ...func(*ec2.Options)) (*ec2.ModifyVpcPeeringConnectionOptionsOutput, error)
	// DeleteVpcPeeringConnectionRequestFun
	DeleteVpcPeeringConnectionRequestFun func(ctx context.Context, input *ec2.DeleteVpcPeeringConnectionInput, opts ...func(*ec2.Options)) (*ec2.DeleteVpcPeeringConnectionOutput, error)
	// CreateTagsRequestFun
	CreateTagsRequestFun func(ctx context.Context, input *ec2.CreateTagsInput, opts ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)
	// AcceptVpcPeeringConnectionRequestFun
	AcceptVpcPeeringConnectionRequestFun func(ctx context.Context, input *ec2.AcceptVpcPeeringConnectionInput, opts ...func(*ec2.Options)) (*ec2.AcceptVpcPeeringConnectionOutput, error)
}

// CreateRoute create route request
func (m *MockEC2Client) CreateRoute(ctx context.Context, input *ec2.CreateRouteInput, opts ...func(*ec2.Options)) (*ec2.CreateRouteOutput, error) {
	return m.CreateRouteRequestFun(ctx, input)
}

// DescribeVpcPeeringConnections describe vpc peering connection
func (m *MockEC2Client) DescribeVpcPeeringConnections(ctx context.Context, input *ec2.DescribeVpcPeeringConnectionsInput, opts ...func(*ec2.Options)) (*ec2.DescribeVpcPeeringConnectionsOutput, error) {
	return m.DescribeVpcPeeringConnectionsRequestFun(ctx, input)
}

// CreateVpcPeeringConnection create vpc peering connection.
func (m *MockEC2Client) CreateVpcPeeringConnection(ctx context.Context, input *ec2.CreateVpcPeeringConnectionInput, opts ...func(*ec2.Options)) (*ec2.CreateVpcPeeringConnectionOutput, error) {
	return m.CreateVpcPeeringConnectionRequestFun(ctx, input)
}

// DescribeRouteTables describe route table.
func (m *MockEC2Client) DescribeRouteTables(ctx context.Context, input *ec2.DescribeRouteTablesInput, opts ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
	return m.DescribeRouteTablesRequestFun(ctx, input)
}

// DeleteRoute delete route.
func (m *MockEC2Client) DeleteRoute(ctx context.Context, input *ec2.DeleteRouteInput, opts ...func(*ec2.Options)) (*ec2.DeleteRouteOutput, error) {
	return m.DeleteRouteRequestFun(ctx, input)
}

// ModifyVpcPeeringConnectionOptions modify vpc peering
func (m *MockEC2Client) ModifyVpcPeeringConnectionOptions(ctx context.Context, input *ec2.ModifyVpcPeeringConnectionOptionsInput, opts ...func(*ec2.Options)) (*ec2.ModifyVpcPeeringConnectionOptionsOutput, error) {
	return m.ModifyVpcPeeringConnectionOptionsRequestFun(ctx, input)
}

// DeleteVpcPeeringConnection delete vpc peering
func (m *MockEC2Client) DeleteVpcPeeringConnection(ctx context.Context, input *ec2.DeleteVpcPeeringConnectionInput, opts ...func(*ec2.Options)) (*ec2.DeleteVpcPeeringConnectionOutput, error) {
	return m.DeleteVpcPeeringConnectionRequestFun(ctx, input)
}

// CreateTags create tags
func (m *MockEC2Client) CreateTags(ctx context.Context, input *ec2.CreateTagsInput, opts ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
	return m.CreateTagsRequestFun(ctx, input)
}

// AcceptVpcPeeringConnection accept vpc peering connection
func (m *MockEC2Client) AcceptVpcPeeringConnection(ctx context.Context, input *ec2.AcceptVpcPeeringConnectionInput, opts ...func(*ec2.Options)) (*ec2.AcceptVpcPeeringConnectionOutput, error) {
	return m.AcceptVpcPeeringConnectionRequestFun(ctx, input)
}

// MockRoute53Client route53 client
type MockRoute53Client struct {
	// CreateVPCAssociationAuthorizationRequestFun mock create vpc AssociationAuthorization
	CreateVPCAssociationAuthorizationRequestFun func(ctx context.Context, input *route53.CreateVPCAssociationAuthorizationInput, opts ...func(*route53.Options)) (*route53.CreateVPCAssociationAuthorizationOutput, error)
	// DeleteVPCAssociationAuthorizationRequestFun mock delete vpc AssociationAuthorization
	DeleteVPCAssociationAuthorizationRequestFun func(ctx context.Context, input *route53.DeleteVPCAssociationAuthorizationInput, opts ...func(*route53.Options)) (*route53.DeleteVPCAssociationAuthorizationOutput, error)
}

// CreateVPCAssociationAuthorization create AssociationAuthorization
func (m *MockRoute53Client) CreateVPCAssociationAuthorization(ctx context.Context, input *route53.CreateVPCAssociationAuthorizationInput, opts ...func(*route53.Options)) (*route53.CreateVPCAssociationAuthorizationOutput, error) {
	return m.CreateVPCAssociationAuthorizationRequestFun(ctx, input)
}

// DeleteVPCAssociationAuthorization delete AssociationAuthorization
func (m *MockRoute53Client) DeleteVPCAssociationAuthorization(ctx context.Context, input *route53.DeleteVPCAssociationAuthorizationInput, opts ...func(*route53.Options)) (*route53.DeleteVPCAssociationAuthorizationOutput, error) {
	return m.DeleteVPCAssociationAuthorizationRequestFun(ctx, input)
}

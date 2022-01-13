package vpcpeering

import (
	"fmt"
	"strings"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/meta"

	"net/http"

	"context"

	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"

	"github.com/crossplane/provider-aws/pkg/clients/peering"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	svcapitypes "github.com/crossplane/provider-aws/apis/vpcpeering/v1alpha1"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/provider-aws/pkg/clients/peering/fake"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var log = logging.NewLogrLogger(zap.New(zap.UseDevMode(true)).WithName("vpcpeering"))

type args struct {
	kube       client.Client
	client     peering.EC2Client
	peerClient peering.EC2Client
	route53Cli peering.Route53Client
	cr         *svcapitypes.VPCPeeringConnection
	isInternal bool
}

func TestObserve(t *testing.T) {
	g := NewGomegaWithT(t)
	type want struct {
		result managed.ExternalObservation
		err    error
	}

	cases := map[string]struct {
		args
		want
	}{
		"Create": {
			args: args{
				kube: &test.MockClient{
					MockUpdate: test.NewMockClient().Update,
					MockStatusUpdate: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return nil
					},
				},
				cr: buildVPCPeerConnection("test"),
				client: &fake.MockEC2Client{
					DescribeVpcPeeringConnectionsRequestFun: func(input *ec2.DescribeVpcPeeringConnectionsInput) ec2.DescribeVpcPeeringConnectionsRequest {
						return ec2.DescribeVpcPeeringConnectionsRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeVpcPeeringConnectionsOutput{
								//Attributes: attributes,
								VpcPeeringConnections: []ec2.VpcPeeringConnection{},
							}},
						}
					},
					DescribeRouteTablesRequestFun: func(input *ec2.DescribeRouteTablesInput) ec2.DescribeRouteTablesRequest {
						return ec2.DescribeRouteTablesRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeRouteTablesOutput{
								RouteTables: make([]ec2.RouteTable, 0),
							}},
						}
					},
				},
			},
			want: want{
				result: managed.ExternalObservation{
					ResourceExists:          false,
					ResourceUpToDate:        false,
					ResourceLateInitialized: false,
				},
			},
		},
		"Created": {
			args: args{
				kube: &test.MockClient{
					MockUpdate: test.NewMockClient().Update,
					MockStatusUpdate: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return nil
					},
				},
				cr: func() *svcapitypes.VPCPeeringConnection {
					cr := buildVPCPeerConnection("test")
					cr.Status.SetConditions(Approved())

					return cr
				}(),
				client: &fake.MockEC2Client{
					DescribeVpcPeeringConnectionsRequestFun: func(input *ec2.DescribeVpcPeeringConnectionsInput) ec2.DescribeVpcPeeringConnectionsRequest {
						return ec2.DescribeVpcPeeringConnectionsRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeVpcPeeringConnectionsOutput{
								//Attributes: attributes,
								VpcPeeringConnections: []ec2.VpcPeeringConnection{
									{
										Status: &ec2.VpcPeeringConnectionStateReason{
											Code: ec2.VpcPeeringConnectionStateReasonCodeActive,
										},

										Tags: []ec2.Tag{
											{
												Key:   aws.String("Name"),
												Value: aws.String("test"),
											},
										},
										VpcPeeringConnectionId: aws.String("pcx-xxx"),
									},
								},
							}},
						}
					},
					DescribeRouteTablesRequestFun: func(input *ec2.DescribeRouteTablesInput) ec2.DescribeRouteTablesRequest {
						return ec2.DescribeRouteTablesRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeRouteTablesOutput{
								RouteTables: make([]ec2.RouteTable, 0),
							}},
						}
					},
				},
			},
			want: want{
				result: managed.ExternalObservation{
					ResourceExists:          true,
					ResourceUpToDate:        false,
					ResourceLateInitialized: false,
				},
			},
		},
		"Update": {
			args: args{
				kube: &test.MockClient{
					MockUpdate: test.NewMockClient().Update,
					MockStatusUpdate: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return nil
					},
				},
				cr: buildVPCPeerConnection("test"),
				client: &fake.MockEC2Client{
					DescribeVpcPeeringConnectionsRequestFun: func(input *ec2.DescribeVpcPeeringConnectionsInput) ec2.DescribeVpcPeeringConnectionsRequest {
						return ec2.DescribeVpcPeeringConnectionsRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeVpcPeeringConnectionsOutput{
								//Attributes: attributes,
								VpcPeeringConnections: []ec2.VpcPeeringConnection{
									{
										Status: &ec2.VpcPeeringConnectionStateReason{
											Code: ec2.VpcPeeringConnectionStateReasonCodeActive,
										},

										Tags: []ec2.Tag{
											{
												Key:   aws.String("Name"),
												Value: aws.String("test"),
											},
										},
										VpcPeeringConnectionId: aws.String("pcx-xxx"),
									},
								},
							}},
						}
					},
					DescribeRouteTablesRequestFun: func(input *ec2.DescribeRouteTablesInput) ec2.DescribeRouteTablesRequest {
						return ec2.DescribeRouteTablesRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeRouteTablesOutput{
								RouteTables: make([]ec2.RouteTable, 0),
							}},
						}
					},
				},
			},
			want: want{
				result: managed.ExternalObservation{
					ResourceExists:          true,
					ResourceUpToDate:        false,
					ResourceLateInitialized: false,
				},
			},
		},
		"PendingAccept": {
			args: args{
				kube: &test.MockClient{
					MockUpdate: test.NewMockClient().Update,
					MockStatusUpdate: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return nil
					},
				},
				cr: buildVPCPeerConnection("test"),
				client: &fake.MockEC2Client{
					DescribeVpcPeeringConnectionsRequestFun: func(input *ec2.DescribeVpcPeeringConnectionsInput) ec2.DescribeVpcPeeringConnectionsRequest {
						return ec2.DescribeVpcPeeringConnectionsRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeVpcPeeringConnectionsOutput{
								//Attributes: attributes,
								VpcPeeringConnections: []ec2.VpcPeeringConnection{
									{
										Status: &ec2.VpcPeeringConnectionStateReason{
											Code: ec2.VpcPeeringConnectionStateReasonCodePendingAcceptance,
										},
									},
								},
							}},
						}
					},
					DescribeRouteTablesRequestFun: func(input *ec2.DescribeRouteTablesInput) ec2.DescribeRouteTablesRequest {
						return ec2.DescribeRouteTablesRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeRouteTablesOutput{
								RouteTables: make([]ec2.RouteTable, 0),
							}},
						}
					},
				},
			},
			want: want{
				result: managed.ExternalObservation{
					ResourceExists:          true,
					ResourceUpToDate:        false,
					ResourceLateInitialized: false,
				},
				err: fmt.Errorf(errWaitVpcPeeringConnectionAccept),
			},
		},
		"Deleting": {
			args: args{
				kube: &test.MockClient{
					MockUpdate: test.NewMockClient().Update,
					MockStatusUpdate: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return nil
					},
				},
				cr: inDeletingVPCPeerConnection("test"),
				client: &fake.MockEC2Client{
					DescribeVpcPeeringConnectionsRequestFun: func(input *ec2.DescribeVpcPeeringConnectionsInput) ec2.DescribeVpcPeeringConnectionsRequest {
						return ec2.DescribeVpcPeeringConnectionsRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeVpcPeeringConnectionsOutput{
								//Attributes: attributes,
								VpcPeeringConnections: []ec2.VpcPeeringConnection{
									{
										Status: &ec2.VpcPeeringConnectionStateReason{
											Code: ec2.VpcPeeringConnectionStateReasonCodePendingAcceptance,
										},
									},
								},
							}},
						}
					},
				},
			},
			want: want{
				result: managed.ExternalObservation{
					ResourceExists:          true,
					ResourceUpToDate:        false,
					ResourceLateInitialized: false,
				},
			},
		},
		"InternalPeering": {
			args: args{
				kube: &test.MockClient{
					MockUpdate: test.NewMockClient().Update,
					MockStatusUpdate: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return nil
					},
				},
				cr: buildVPCPeerConnection("test"),
				client: &fake.MockEC2Client{
					DescribeVpcPeeringConnectionsRequestFun: func(input *ec2.DescribeVpcPeeringConnectionsInput) ec2.DescribeVpcPeeringConnectionsRequest {
						return ec2.DescribeVpcPeeringConnectionsRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeVpcPeeringConnectionsOutput{
								//Attributes: attributes,
								VpcPeeringConnections: []ec2.VpcPeeringConnection{
									{
										Status: &ec2.VpcPeeringConnectionStateReason{
											Code: ec2.VpcPeeringConnectionStateReasonCodePendingAcceptance,
										},
										VpcPeeringConnectionId: aws.String("peerConnectionID"),
									},
								},
							}},
						}
					},
					DescribeRouteTablesRequestFun: func(input *ec2.DescribeRouteTablesInput) ec2.DescribeRouteTablesRequest {
						return ec2.DescribeRouteTablesRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeRouteTablesOutput{
								RouteTables: make([]ec2.RouteTable, 0),
							}},
						}
					},
				},
				peerClient: &fake.MockEC2Client{
					AcceptVpcPeeringConnectionRequestFun: func(input *ec2.AcceptVpcPeeringConnectionInput) ec2.AcceptVpcPeeringConnectionRequest {
						g.Expect(*input.VpcPeeringConnectionId).Should(Equal("peerConnectionID"))
						return ec2.AcceptVpcPeeringConnectionRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.AcceptVpcPeeringConnectionOutput{}},
						}
					},
				},
				isInternal: true,
			},
			want: want{
				result: managed.ExternalObservation{
					ResourceExists:          true,
					ResourceUpToDate:        false,
					ResourceLateInitialized: false,
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{
				client:        tc.client,
				kube:          tc.kube,
				route53Client: tc.route53Cli,
				log:           log,
				isInternal:    tc.isInternal,
				peerClient:    tc.peerClient,
			}

			o, err := e.Observe(context.Background(), tc.args.cr)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.result, o); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	g := NewGomegaWithT(t)

	type want struct {
		result   managed.ExternalCreation
		err      error
		vpcID    string
		acountID string
	}

	cases := map[string]struct {
		args
		want
	}{
		"Create": {
			args: args{
				kube: &test.MockClient{
					MockUpdate: test.NewMockClient().Update,
					MockStatusUpdate: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return nil
					},
				},
				route53Cli: &fake.MockRoute53Client{},
				cr:         buildVPCPeerConnection("test"),
				client: &fake.MockEC2Client{
					CreateVpcPeeringConnectionRequestFun: func(input *ec2.CreateVpcPeeringConnectionInput) ec2.CreateVpcPeeringConnectionRequest {
						g.Expect(*input.PeerRegion).Should(Equal("peerRegion"))
						g.Expect(*input.PeerOwnerId).Should(Equal("peerOwner"))
						g.Expect(*input.PeerVpcId).Should(Equal("peerVpc"))
						g.Expect(*input.VpcId).Should(Equal("ownerVpc"))

						return ec2.CreateVpcPeeringConnectionRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.CreateVpcPeeringConnectionOutput{
								//Attributes: attributes,
								VpcPeeringConnection: &ec2.VpcPeeringConnection{
									VpcPeeringConnectionId: aws.String("pcx-xxx"),
								},
							}},
						}
					},

					CreateTagsRequestFun: func(input *ec2.CreateTagsInput) ec2.CreateTagsRequest {
						g.Expect(len(input.Tags)).Should(Equal(1))
						g.Expect(*input.Tags[0].Key).Should(Equal("Name"))
						g.Expect(*input.Tags[0].Value).Should(Equal("test"))
						return ec2.CreateTagsRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.CreateTagsOutput{}},
						}
					},

					DescribeRouteTablesRequestFun: func(input *ec2.DescribeRouteTablesInput) ec2.DescribeRouteTablesRequest {
						return ec2.DescribeRouteTablesRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeRouteTablesOutput{
								RouteTables: make([]ec2.RouteTable, 0),
							}},
						}
					},
				},
			},
			want: want{
				result: managed.ExternalCreation{},
				vpcID:  "pcx-xxx",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{
				client:        tc.client,
				kube:          tc.kube,
				route53Client: tc.route53Cli,
				log:           log,
			}
			_, err := e.Create(context.Background(), tc.args.cr)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(meta.GetExternalName(tc.args.cr), tc.want.vpcID); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	g := NewGomegaWithT(t)
	var peeringConnectionID = "my-peering-id"
	pc := buildVPCPeerConnection("test")
	pc.Spec.ForProvider.PeerCIDR = aws.String("10.0.0.0/8")
	pc.Status.AtProvider.VPCPeeringConnectionID = aws.String(peeringConnectionID)
	pc.Status.AtProvider.RequesterVPCInfo = &svcapitypes.VPCPeeringConnectionVPCInfo{
		CIDRBlock: aws.String("196.168.0.0/16"),
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		args
		want
	}{
		"Delete": {
			args: args{
				kube: &test.MockClient{
					MockDelete: test.NewMockClient().Delete,
				},
				route53Cli: &fake.MockRoute53Client{
					DeleteVPCAssociationAuthorizationRequestFun: func(input *route53.DeleteVPCAssociationAuthorizationInput) route53.DeleteVPCAssociationAuthorizationRequest {
						g.Expect(*input.HostedZoneId).Should(Equal("owner"))
						g.Expect(*input.VPC.VPCId).Should(Equal("peerVpc"))
						g.Expect(string(input.VPC.VPCRegion)).Should(Equal("peerRegion"))

						return route53.DeleteVPCAssociationAuthorizationRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &route53.DeleteVPCAssociationAuthorizationOutput{}},
						}
					},
				},
				cr: pc.DeepCopy(),
				client: &fake.MockEC2Client{
					DescribeRouteTablesRequestFun: func(input *ec2.DescribeRouteTablesInput) ec2.DescribeRouteTablesRequest {
						return ec2.DescribeRouteTablesRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeRouteTablesOutput{
								RouteTables: make([]ec2.RouteTable, 0),
							}},
						}
					},
					DescribeVpcPeeringConnectionsRequestFun: func(input *ec2.DescribeVpcPeeringConnectionsInput) ec2.DescribeVpcPeeringConnectionsRequest {
						return ec2.DescribeVpcPeeringConnectionsRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeVpcPeeringConnectionsOutput{
								//Attributes: attributes,
								VpcPeeringConnections: []ec2.VpcPeeringConnection{
									{
										Status: &ec2.VpcPeeringConnectionStateReason{
											Code: ec2.VpcPeeringConnectionStateReasonCodePendingAcceptance,
										},

										Tags: []ec2.Tag{
											{
												Key:   aws.String("Name"),
												Value: aws.String("test"),
											},
										},
										VpcPeeringConnectionId: aws.String("pcx-xxx"),
									},
								},
							}},
						}
					},
					DeleteVpcPeeringConnectionRequestFun: func(input *ec2.DeleteVpcPeeringConnectionInput) ec2.DeleteVpcPeeringConnectionRequest {
						return ec2.DeleteVpcPeeringConnectionRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DeleteVpcPeeringConnectionOutput{}},
						}
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"InternalPeering": {
			args: args{
				isInternal: true,
				kube: &test.MockClient{
					MockDelete: test.NewMockClient().Delete,
				},
				route53Cli: &fake.MockRoute53Client{
					DeleteVPCAssociationAuthorizationRequestFun: func(input *route53.DeleteVPCAssociationAuthorizationInput) route53.DeleteVPCAssociationAuthorizationRequest {
						g.Expect(*input.HostedZoneId).Should(Equal("owner"))
						g.Expect(*input.VPC.VPCId).Should(Equal("peerVpc"))
						g.Expect(string(input.VPC.VPCRegion)).Should(Equal("peerRegion"))

						return route53.DeleteVPCAssociationAuthorizationRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &route53.DeleteVPCAssociationAuthorizationOutput{}},
						}
					},
				},
				cr: pc.DeepCopy(),
				client: &fake.MockEC2Client{
					DescribeRouteTablesRequestFun: func(input *ec2.DescribeRouteTablesInput) ec2.DescribeRouteTablesRequest {
						g.Expect(len(input.Filters)).Should(Equal(1))
						g.Expect(input.Filters[0].Name).Should((Equal(aws.String("vpc-id"))))
						g.Expect(input.Filters[0].Values).Should((Equal([]string{"ownerVpc"})))

						return ec2.DescribeRouteTablesRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeRouteTablesOutput{
								RouteTables: []ec2.RouteTable{
									{
										RouteTableId: aws.String("rt1"),
									},
								},
							}},
						}
					},
					DeleteRouteRequestFun: func(input *ec2.DeleteRouteInput) ec2.DeleteRouteRequest {
						g.Expect(input.RouteTableId).Should((Equal(aws.String("rt1"))))
						g.Expect(input.DestinationCidrBlock).Should((Equal(aws.String("10.0.0.0/8"))))
						return ec2.DeleteRouteRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DeleteRouteOutput{}},
						}
					},
					DescribeVpcPeeringConnectionsRequestFun: func(input *ec2.DescribeVpcPeeringConnectionsInput) ec2.DescribeVpcPeeringConnectionsRequest {
						return ec2.DescribeVpcPeeringConnectionsRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeVpcPeeringConnectionsOutput{
								//Attributes: attributes,
								VpcPeeringConnections: []ec2.VpcPeeringConnection{
									{
										Status: &ec2.VpcPeeringConnectionStateReason{
											Code: ec2.VpcPeeringConnectionStateReasonCodePendingAcceptance,
										},

										Tags: []ec2.Tag{
											{
												Key:   aws.String("Name"),
												Value: aws.String("test"),
											},
										},
										VpcPeeringConnectionId: aws.String("pcx-xxx"),
									},
								},
							}},
						}
					},
					DeleteVpcPeeringConnectionRequestFun: func(input *ec2.DeleteVpcPeeringConnectionInput) ec2.DeleteVpcPeeringConnectionRequest {
						return ec2.DeleteVpcPeeringConnectionRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DeleteVpcPeeringConnectionOutput{}},
						}
					},
				},
				peerClient: &fake.MockEC2Client{
					DescribeRouteTablesRequestFun: func(input *ec2.DescribeRouteTablesInput) ec2.DescribeRouteTablesRequest {
						g.Expect(len(input.Filters)).Should(Equal(1))
						g.Expect(input.Filters[0].Name).Should((Equal(aws.String("vpc-id"))))
						g.Expect(input.Filters[0].Values).Should((Equal([]string{"peerVpc"})))

						return ec2.DescribeRouteTablesRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeRouteTablesOutput{
								RouteTables: []ec2.RouteTable{
									{
										RouteTableId: aws.String("rt2"),
									},
								},
							}},
						}
					},
					DeleteRouteRequestFun: func(input *ec2.DeleteRouteInput) ec2.DeleteRouteRequest {
						g.Expect(input.RouteTableId).Should((Equal(aws.String("rt2"))))
						g.Expect(input.DestinationCidrBlock).Should((Equal(aws.String("196.168.0.0/16"))))
						return ec2.DeleteRouteRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DeleteRouteOutput{}},
						}
					},
				},
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{
				client:        tc.client,
				kube:          tc.kube,
				route53Client: tc.route53Cli,
				log:           log,
				isInternal:    tc.isInternal,
				peerClient:    tc.peerClient,
			}

			err := e.Delete(context.Background(), tc.args.cr)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestUpdateRouteTable(t *testing.T) {
	g := NewGomegaWithT(t)

	var peeringConnectionID = "my-peering-id"
	pc := buildVPCPeerConnection("test")
	// test vpc peering connection no route ready annotation
	pc.Annotations[attributeModified] = "true"
	pc.Annotations[hostedZoneEnsured] = "true"
	pc.Spec.ForProvider.PeerCIDR = aws.String("10.0.0.0/8")
	pc.Status.AtProvider.VPCPeeringConnectionID = aws.String(peeringConnectionID)
	pc.Status.AtProvider.RequesterVPCInfo = &svcapitypes.VPCPeeringConnectionVPCInfo{
		CIDRBlock: aws.String("196.168.0.0/16"),
	}
	type want struct {
		result managed.ExternalUpdate
		err    error
	}

	cases := map[string]struct {
		args
		want
	}{
		"Create route successful": {
			args: args{
				kube: &test.MockClient{
					MockUpdate: test.NewMockClient().Update,
					MockStatusUpdate: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return nil
					},
				},
				route53Cli: &fake.MockRoute53Client{},
				cr:         pc.DeepCopy(),
				client: &fake.MockEC2Client{
					DescribeRouteTablesRequestFun: func(input *ec2.DescribeRouteTablesInput) ec2.DescribeRouteTablesRequest {
						g.Expect(len(input.Filters)).Should(Equal(1))
						g.Expect(input.Filters[0].Name).Should((Equal(aws.String("vpc-id"))))
						g.Expect(input.Filters[0].Values).Should((Equal([]string{"ownerVpc"})))
						return ec2.DescribeRouteTablesRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeRouteTablesOutput{
								RouteTables: []ec2.RouteTable{
									{
										RouteTableId: aws.String("rt1"),
									},
								},
							}},
						}
					},
					CreateRouteRequestFun: func(input *ec2.CreateRouteInput) ec2.CreateRouteRequest {
						g.Expect(input.RouteTableId).Should((Equal(aws.String("rt1"))))
						g.Expect(input.DestinationCidrBlock).Should((Equal(aws.String("10.0.0.0/8"))))
						g.Expect(input.VpcPeeringConnectionId).Should((Equal(aws.String(peeringConnectionID))))
						return ec2.CreateRouteRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.CreateRouteOutput{
								Return: aws.Bool(true),
							}},
						}
					},
				},
			},
			want: want{
				result: managed.ExternalUpdate{},
			},
		},
		"Create route already exist and routes is match": {
			args: args{
				kube: &test.MockClient{
					MockUpdate: test.NewMockClient().Update,
					MockStatusUpdate: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return nil
					},
				},
				route53Cli: &fake.MockRoute53Client{},
				cr:         pc.DeepCopy(),
				client: &fake.MockEC2Client{
					DescribeRouteTablesRequestFun: func(input *ec2.DescribeRouteTablesInput) ec2.DescribeRouteTablesRequest {
						g.Expect(len(input.Filters)).Should(Equal(1))
						g.Expect(input.Filters[0].Name).Should((Equal(aws.String("vpc-id"))))
						g.Expect(input.Filters[0].Values).Should((Equal([]string{"ownerVpc"})))
						return ec2.DescribeRouteTablesRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeRouteTablesOutput{
								RouteTables: []ec2.RouteTable{
									{
										RouteTableId: aws.String("rt1"),
										Routes: []ec2.Route{
											{
												DestinationCidrBlock:   aws.String("10.0.0.0/8"),
												VpcPeeringConnectionId: aws.String(peeringConnectionID),
											},
											// cidr not equal will never conflict
											{
												DestinationCidrBlock:   aws.String("other-cidr"),
												VpcPeeringConnectionId: aws.String(peeringConnectionID),
											},
										},
									},
								},
							}},
						}
					},

					CreateRouteRequestFun: func(input *ec2.CreateRouteInput) ec2.CreateRouteRequest {
						g.Expect(input.RouteTableId).Should((Equal(aws.String("rt1"))))
						g.Expect(input.DestinationCidrBlock).Should((Equal(aws.String("10.0.0.0/8"))))
						g.Expect(input.VpcPeeringConnectionId).Should((Equal(aws.String(peeringConnectionID))))
						return ec2.CreateRouteRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.CreateRouteOutput{
								Return: aws.Bool(false),
							}, Error: fmt.Errorf("RouteAlreadyExists")},
						}
					},
				},
			},
			want: want{
				result: managed.ExternalUpdate{},
			},
		},
		"Create route already exist but route cidr already occupied": {
			args: args{
				kube: &test.MockClient{
					MockUpdate: test.NewMockClient().Update,
					MockStatusUpdate: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return nil
					},
				},
				route53Cli: &fake.MockRoute53Client{},
				cr:         pc.DeepCopy(),
				client: &fake.MockEC2Client{
					DescribeRouteTablesRequestFun: func(input *ec2.DescribeRouteTablesInput) ec2.DescribeRouteTablesRequest {
						g.Expect(len(input.Filters)).Should(Equal(1))
						g.Expect(input.Filters[0].Name).Should((Equal(aws.String("vpc-id"))))
						g.Expect(input.Filters[0].Values).Should((Equal([]string{"ownerVpc"})))
						return ec2.DescribeRouteTablesRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeRouteTablesOutput{
								RouteTables: []ec2.RouteTable{
									{
										RouteTableId: aws.String("rt1"),
										Routes: []ec2.Route{
											{
												DestinationCidrBlock:   aws.String("10.0.0.0/8"),
												VpcPeeringConnectionId: aws.String("other-peering"),
											},
										},
									},
								},
							}},
						}
					},

					CreateRouteRequestFun: func(input *ec2.CreateRouteInput) ec2.CreateRouteRequest {
						g.Expect(input.RouteTableId).Should((Equal(aws.String("rt1"))))
						g.Expect(input.DestinationCidrBlock).Should((Equal(aws.String("10.0.0.0/8"))))
						g.Expect(input.VpcPeeringConnectionId).Should((Equal(aws.String(peeringConnectionID))))
						return ec2.CreateRouteRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.CreateRouteOutput{
								Return: aws.Bool(false),
							}, Error: fmt.Errorf("RouteAlreadyExists")},
						}
					},
				},
			},
			want: want{
				result: managed.ExternalUpdate{},
				err:    fmt.Errorf("failed add route for vpc peering connection: my-peering-id, routeID: rt1: RouteAlreadyExists"),
			},
		},
		"Create route when internal vpc peering": {
			args: args{
				isInternal: true,
				kube: &test.MockClient{
					MockUpdate: test.NewMockClient().Update,
					MockStatusUpdate: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return nil
					},
				},
				route53Cli: &fake.MockRoute53Client{},
				cr:         pc.DeepCopy(),
				client: &fake.MockEC2Client{
					DescribeRouteTablesRequestFun: func(input *ec2.DescribeRouteTablesInput) ec2.DescribeRouteTablesRequest {
						g.Expect(len(input.Filters)).Should(Equal(1))
						g.Expect(input.Filters[0].Name).Should((Equal(aws.String("vpc-id"))))
						g.Expect(input.Filters[0].Values).Should((Equal([]string{"ownerVpc"})))

						return ec2.DescribeRouteTablesRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeRouteTablesOutput{
								RouteTables: []ec2.RouteTable{
									{
										RouteTableId: aws.String("rt1"),
									},
								},
							}},
						}
					},
					CreateRouteRequestFun: func(input *ec2.CreateRouteInput) ec2.CreateRouteRequest {
						g.Expect(input.RouteTableId).Should((Equal(aws.String("rt1"))))
						g.Expect(input.DestinationCidrBlock).Should((Equal(aws.String("10.0.0.0/8"))))
						g.Expect(input.VpcPeeringConnectionId).Should((Equal(aws.String(peeringConnectionID))))
						return ec2.CreateRouteRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.CreateRouteOutput{
								Return: aws.Bool(true),
							}},
						}
					},
				},
				peerClient: &fake.MockEC2Client{
					DescribeRouteTablesRequestFun: func(input *ec2.DescribeRouteTablesInput) ec2.DescribeRouteTablesRequest {
						g.Expect(len(input.Filters)).Should(Equal(1))
						g.Expect(input.Filters[0].Name).Should((Equal(aws.String("vpc-id"))))
						g.Expect(input.Filters[0].Values).Should((Equal([]string{"peerVpc"})))

						return ec2.DescribeRouteTablesRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.DescribeRouteTablesOutput{
								RouteTables: []ec2.RouteTable{
									{
										RouteTableId: aws.String("rt2"),
									},
								},
							}},
						}
					},
					CreateRouteRequestFun: func(input *ec2.CreateRouteInput) ec2.CreateRouteRequest {
						g.Expect(input.RouteTableId).Should((Equal(aws.String("rt2"))))
						g.Expect(input.DestinationCidrBlock).Should((Equal(aws.String("196.168.0.0/16"))))
						g.Expect(input.VpcPeeringConnectionId).Should((Equal(aws.String(peeringConnectionID))))
						return ec2.CreateRouteRequest{
							Request: &aws.Request{HTTPRequest: &http.Request{}, Retryer: aws.NoOpRetryer{}, Data: &ec2.CreateRouteOutput{
								Return: aws.Bool(true),
							}},
						}
					},
				},
			},
			want: want{
				result: managed.ExternalUpdate{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{
				client:        tc.client,
				peerClient:    tc.peerClient,
				kube:          tc.kube,
				route53Client: tc.route53Cli,
				log:           log,
				isInternal:    tc.isInternal,
			}
			result, err := e.Update(context.Background(), tc.args.cr)
			if tc.want.err != nil {
				if diff := cmp.Diff(strings.Contains(err.Error(), tc.want.err.Error()), true); diff != "" {
					t.Fatalf("r: -want, +got:\n%s", diff)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tc.want.result, result); diff != "" {
				t.Fatalf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func buildVPCPeerConnection(name string) *svcapitypes.VPCPeeringConnection {
	cr := &svcapitypes.VPCPeeringConnection{
		ObjectMeta: v1.ObjectMeta{
			Name: name,
		},

		Spec: svcapitypes.VPCPeeringConnectionSpec{
			ForProvider: svcapitypes.VPCPeeringConnectionParameters{
				VPCID:       aws.String("ownerVpc"),
				Region:      "ownerRegion",
				HostZoneID:  aws.String("owner"),
				PeerOwnerID: aws.String("peerOwner"),
				PeerVPCID:   aws.String("peerVpc"),
				PeerRegion:  aws.String("peerRegion"),
				PeerCIDR:    aws.String("10.0.0.1/32"),
			},
		},
	}

	meta.SetExternalName(cr, name)

	return cr
}

func inDeletingVPCPeerConnection(name string) *svcapitypes.VPCPeeringConnection {
	cr := buildVPCPeerConnection(name)

	cr.DeletionTimestamp = &v1.Time{time.Now()}
	return cr
}

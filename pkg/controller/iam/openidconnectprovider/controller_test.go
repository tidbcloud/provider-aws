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

package openidconnectprovider

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsiam "github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane-contrib/provider-aws/apis/iam/v1beta1"
	svcapitypes "github.com/crossplane-contrib/provider-aws/apis/iam/v1beta1"
	"github.com/crossplane-contrib/provider-aws/pkg/clients/iam/fake"
	errorutils "github.com/crossplane-contrib/provider-aws/pkg/utils/errors"
)

var (
	unexpectedItem resource.Managed
	providerArn    = "arn:aws:iam::123456789012:oidc-provider/example.com"
	url            = "https://example.com"
	name           = "oidcProvider"

	errBoom = errors.New("boom")

	key1   = "foo1"
	value1 = "bar1"
	key2   = "foo2"
	value2 = "bar2"

	tagComparer = cmp.Comparer(func(expected, actual iamtypes.Tag) bool {
		return cmp.Equal(expected.Key, actual.Key) &&
			cmp.Equal(expected.Value, actual.Value)
	})

	createInputComparer = cmp.Comparer(func(expected, actual *awsiam.CreateOpenIDConnectProviderInput) bool {
		return cmp.Equal(expected.Url, actual.Url) &&
			cmp.Equal(expected.ClientIDList, actual.ClientIDList, test.EquateConditions()) &&
			cmp.Equal(expected.ThumbprintList, actual.ThumbprintList, test.EquateConditions()) &&
			cmp.Equal(expected.Tags, actual.Tags, tagComparer, sortIAMTags)
	})

	tagInputComparer = cmp.Comparer(func(expected, actual *awsiam.TagOpenIDConnectProviderInput) bool {
		return cmp.Equal(expected.OpenIDConnectProviderArn, actual.OpenIDConnectProviderArn) &&
			cmp.Equal(expected.Tags, actual.Tags, tagComparer, sortIAMTags)
	})

	untagInputComparer = cmp.Comparer(func(expected, actual *awsiam.UntagOpenIDConnectProviderInput) bool {
		return cmp.Equal(expected.OpenIDConnectProviderArn, actual.OpenIDConnectProviderArn) &&
			cmp.Equal(expected.TagKeys, actual.TagKeys, sortStrings)
	})

	sortTags = cmpopts.SortSlices(func(a, b v1beta1.Tag) bool {
		return a.Key > b.Key
	})
	sortIAMTags = cmpopts.SortSlices(func(a, b iamtypes.Tag) bool {
		return *a.Key > *b.Key
	})
	sortStrings = cmpopts.SortSlices(func(x, y string) bool {
		return x < y
	})
)

type args struct {
	iam  *fake.MockOpenIDConnectProviderClient
	kube client.Client
	cr   resource.Managed
}

type oidcProviderModifier func(provider *svcapitypes.OpenIDConnectProvider)

func withConditions(c ...xpv1.Condition) oidcProviderModifier {
	return func(r *svcapitypes.OpenIDConnectProvider) { r.Status.ConditionedStatus.Conditions = c }
}

func withURL(s string) oidcProviderModifier {
	return func(r *svcapitypes.OpenIDConnectProvider) { r.Spec.ForProvider.URL = s }
}

func withName(name string) oidcProviderModifier {
	return func(r *svcapitypes.OpenIDConnectProvider) { r.Name = name }
}

func withExternalName(name string) oidcProviderModifier {
	return func(r *svcapitypes.OpenIDConnectProvider) { meta.SetExternalName(r, name) }
}

func withAtProvider(s svcapitypes.OpenIDConnectProviderObservation) oidcProviderModifier {
	return func(r *svcapitypes.OpenIDConnectProvider) { r.Status.AtProvider = s }
}

func withTags(tagMaps ...map[string]string) oidcProviderModifier {
	var tagList []v1beta1.Tag
	for _, tagMap := range tagMaps {
		for k, v := range tagMap {
			tagList = append(tagList, v1beta1.Tag{Key: k, Value: v})
		}
	}
	return func(r *v1beta1.OpenIDConnectProvider) {
		r.Spec.ForProvider.Tags = tagList
	}
}

func withClientIDList(l []string) oidcProviderModifier {
	return func(r *svcapitypes.OpenIDConnectProvider) {
		r.Spec.ForProvider.ClientIDList = l
	}
}

func withThumbprintList(l []string) oidcProviderModifier {
	return func(r *svcapitypes.OpenIDConnectProvider) {
		r.Spec.ForProvider.ThumbprintList = l
	}
}

func oidcProvider(m ...oidcProviderModifier) *svcapitypes.OpenIDConnectProvider {
	cr := &svcapitypes.OpenIDConnectProvider{}
	for _, f := range m {
		f(cr)
	}
	return cr
}

func oidcProviderList(n int) []iamtypes.OpenIDConnectProviderListEntry {
	providers := make([]iamtypes.OpenIDConnectProviderListEntry, n)
	for i := range providers {
		providers[i].Arn = aws.String(fmt.Sprintf("arn:aws:iam::123456789012:oidc-provider/unrelated-%03d.example.com", i))
	}
	return providers
}

func TestObserve(t *testing.T) {
	now := metav1.Now()
	type want struct {
		cr     resource.Managed
		result managed.ExternalObservation
		err    error
	}

	cases := map[string]struct {
		args
		want
	}{
		"InvalidInput": {
			args: args{
				cr: unexpectedItem,
			},
			want: want{
				cr:  unexpectedItem,
				err: errors.New(errUnexpectedObject),
			},
		},
		"ClientError": {
			args: args{
				iam: &fake.MockOpenIDConnectProviderClient{
					MockGetOpenIDConnectProvider: func(ctx context.Context, input *awsiam.GetOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.GetOpenIDConnectProviderOutput, error) {
						return nil, errBoom
					},
					MockListOpenIDConnectProviders: func(ctx context.Context, input *awsiam.ListOpenIDConnectProvidersInput, opts []func(*awsiam.Options)) (*awsiam.ListOpenIDConnectProvidersOutput, error) {
						return &awsiam.ListOpenIDConnectProvidersOutput{}, nil
					},
				},
				cr: oidcProvider(withURL(url),
					withExternalName(providerArn)),
			},
			want: want{
				cr: oidcProvider(withURL(url),
					withExternalName(providerArn)),
				err: errorutils.Wrap(errBoom, errGet),
			},
		},
		"NoExternalNameExistingResource": {
			args: args{
				iam: &fake.MockOpenIDConnectProviderClient{
					MockListOpenIDConnectProviders: func(ctx context.Context, input *awsiam.ListOpenIDConnectProvidersInput, opts []func(*awsiam.Options)) (*awsiam.ListOpenIDConnectProvidersOutput, error) {
						return &awsiam.ListOpenIDConnectProvidersOutput{
							OpenIDConnectProviderList: []iamtypes.OpenIDConnectProviderListEntry{
								{Arn: aws.String(providerArn)},
							},
						}, nil
					},
					MockListOpenIDConnectProviderTags: func(ctx context.Context, input *awsiam.ListOpenIDConnectProviderTagsInput, opts []func(*awsiam.Options)) (*awsiam.ListOpenIDConnectProviderTagsOutput, error) {
						return &awsiam.ListOpenIDConnectProviderTagsOutput{
							Tags: []iamtypes.Tag{
								{Key: aws.String(resource.ExternalResourceTagKeyName), Value: aws.String(name)},
							},
						}, nil
					},
					MockGetOpenIDConnectProvider: func(ctx context.Context, input *awsiam.GetOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.GetOpenIDConnectProviderOutput, error) {
						return &awsiam.GetOpenIDConnectProviderOutput{
							CreateDate: &now.Time,
						}, nil
					},
				},
				kube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				cr: oidcProvider(withName(name), withURL(url)),
			},
			want: want{
				cr: oidcProvider(withURL(url),
					withName(name),
					withExternalName(providerArn),
					withConditions(xpv1.Available()),
					withAtProvider(svcapitypes.OpenIDConnectProviderObservation{
						CreateDate: &now,
					})),
				result: managed.ExternalObservation{
					ResourceExists:   true,
					ResourceUpToDate: true,
				},
			},
		},
		"NoExternalNameClientError": {
			args: args{
				iam: &fake.MockOpenIDConnectProviderClient{
					MockListOpenIDConnectProviders: func(ctx context.Context, input *awsiam.ListOpenIDConnectProvidersInput, opts []func(*awsiam.Options)) (*awsiam.ListOpenIDConnectProvidersOutput, error) {
						return nil, errBoom
					},
				},
				cr: oidcProvider(withURL(url)),
			},
			want: want{
				cr:  oidcProvider(withURL(url)),
				err: errorutils.Wrap(errBoom, errList),
				result: managed.ExternalObservation{
					ResourceExists: false,
				},
			},
		},
		"ResourceDoesNotExistName": {
			args: args{
				iam: &fake.MockOpenIDConnectProviderClient{
					MockListOpenIDConnectProviders: func(ctx context.Context, input *awsiam.ListOpenIDConnectProvidersInput, opts []func(*awsiam.Options)) (*awsiam.ListOpenIDConnectProvidersOutput, error) {
						return &awsiam.ListOpenIDConnectProvidersOutput{}, nil
					},
				},
				cr: oidcProvider(withURL(url)),
			},
			want: want{
				cr: oidcProvider(withURL(url)),
				result: managed.ExternalObservation{
					ResourceExists: false,
				},
			},
		},
		"ResourceDoesNotExistAWS": {
			args: args{
				iam: &fake.MockOpenIDConnectProviderClient{
					MockGetOpenIDConnectProvider: func(ctx context.Context, input *awsiam.GetOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.GetOpenIDConnectProviderOutput, error) {
						return nil, &iamtypes.NoSuchEntityException{}
					},
				},
				cr: oidcProvider(withURL(url),
					withExternalName(providerArn)),
			},
			want: want{
				cr: oidcProvider(withURL(url),
					withExternalName(providerArn)),
				result: managed.ExternalObservation{
					ResourceExists: false,
				},
			},
		},
		"ValidInput": {
			args: args{
				iam: &fake.MockOpenIDConnectProviderClient{
					MockGetOpenIDConnectProvider: func(ctx context.Context, input *awsiam.GetOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.GetOpenIDConnectProviderOutput, error) {
						return &awsiam.GetOpenIDConnectProviderOutput{
							CreateDate: &now.Time,
						}, nil
					},
				},
				cr: oidcProvider(withURL(url),
					withExternalName(providerArn)),
			},
			want: want{
				cr: oidcProvider(withURL(url),
					withExternalName(providerArn),
					withAtProvider(svcapitypes.OpenIDConnectProviderObservation{
						CreateDate: &now,
					}),
					withConditions(xpv1.Available())),
				result: managed.ExternalObservation{
					ResourceExists:   true,
					ResourceUpToDate: true,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{kube: tc.kube, client: tc.iam}
			o, err := e.Observe(context.Background(), tc.args.cr)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.cr, tc.args.cr, test.EquateConditions()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.result, o); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestOIDCARNMatchesURL(t *testing.T) {
	cases := map[string]struct {
		providerARN string
		providerURL string
		want        bool
	}{
		"CommercialPartition": {
			providerARN: providerArn,
			providerURL: url,
			want:        true,
		},
		"GKEPath": {
			providerARN: "arn:aws:iam::123456789012:oidc-provider/container.googleapis.com/v1/projects/project/locations/us-central1/clusters/cluster",
			providerURL: "https://container.googleapis.com/v1/projects/project/locations/us-central1/clusters/cluster",
			want:        true,
		},
		"EKSPathWithTrailingSlash": {
			providerARN: "arn:aws:iam::123456789012:oidc-provider/oidc.eks.us-west-2.amazonaws.com/id/issuer/",
			providerURL: "https://oidc.eks.us-west-2.amazonaws.com/id/issuer/",
			want:        true,
		},
		"ChinaPartition": {
			providerARN: "arn:aws-cn:iam::123456789012:oidc-provider/example.com/path",
			providerURL: "https://example.com/path",
			want:        true,
		},
		"GovCloudPartition": {
			providerARN: "arn:aws-us-gov:iam::123456789012:oidc-provider/example.com/path",
			providerURL: "https://example.com/path",
			want:        true,
		},
		"DifferentPath": {
			providerARN: "arn:aws:iam::123456789012:oidc-provider/example.com/path-a",
			providerURL: "https://example.com/path-b",
		},
		"PathPrefixOnly": {
			providerARN: "arn:aws:iam::123456789012:oidc-provider/example.com/clusters/cluster-extra",
			providerURL: "https://example.com/clusters/cluster",
		},
		"CaseMismatch": {
			providerARN: "arn:aws:iam::123456789012:oidc-provider/Example.com/Path",
			providerURL: "https://example.com/Path",
		},
		"TrailingSlashMismatch": {
			providerARN: "arn:aws:iam::123456789012:oidc-provider/example.com/path/",
			providerURL: "https://example.com/path",
		},
		"PercentEscapeLiteralMatch": {
			providerARN: "arn:aws:iam::123456789012:oidc-provider/example.com/a%2Fb",
			providerURL: "https://example.com/a%2Fb",
			want:        true,
		},
		"PercentEscapeCaseMismatch": {
			providerARN: "arn:aws:iam::123456789012:oidc-provider/example.com/a%2Fb",
			providerURL: "https://example.com/a%2fb",
		},
		"PortLiteralMatch": {
			providerARN: "arn:aws:iam::123456789012:oidc-provider/example.com:8443/path",
			providerURL: "https://example.com:8443/path",
			want:        true,
		},
		"WrongService": {
			providerARN: "arn:aws:sts::123456789012:oidc-provider/example.com",
			providerURL: url,
		},
		"RegionalIAMARN": {
			providerARN: "arn:aws:iam:us-east-1:123456789012:oidc-provider/example.com",
			providerURL: url,
		},
		"WrongResourceType": {
			providerARN: "arn:aws:iam::123456789012:role/example.com",
			providerURL: url,
		},
		"MalformedARN": {
			providerARN: "arn:123",
			providerURL: url,
		},
		"EmptyARN": {
			providerURL: url,
		},
		"HTTPURL": {
			providerARN: providerArn,
			providerURL: "http://example.com",
		},
		"EmptyURL": {
			providerARN: providerArn,
		},
		"EmptyIssuer": {
			providerARN: "arn:aws:iam::123456789012:oidc-provider/",
			providerURL: "https://",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := oidcARNMatchesURL(tc.providerARN, tc.providerURL); got != tc.want {
				t.Errorf("oidcARNMatchesURL(%q, %q) = %t, want %t", tc.providerARN, tc.providerURL, got, tc.want)
			}
		})
	}
}

func TestGetOpenIDConnectProviderByTagsFiltersByURL(t *testing.T) {
	matchingProviders := oidcProviderList(164)
	matchingProviders[len(matchingProviders)-1].Arn = aws.String(providerArn)
	correctOwnershipTag := &awsiam.ListOpenIDConnectProviderTagsOutput{Tags: []iamtypes.Tag{{
		Key:   aws.String(resource.ExternalResourceTagKeyName),
		Value: aws.String(name),
	}}}

	type want struct {
		arn       *string
		err       error
		listCalls int
		tagCalls  int
		tagARN    string
	}
	cases := map[string]struct {
		providerURL  string
		externalTags map[string]string
		listOutput   *awsiam.ListOpenIDConnectProvidersOutput
		listErr      error
		tagOutput    *awsiam.ListOpenIDConnectProviderTagsOutput
		tagsByARN    map[string]*awsiam.ListOpenIDConnectProviderTagsOutput
		tagErr       error
		want
	}{
		"NoExpectedNameTag": {
			providerURL: url,
			listOutput: &awsiam.ListOpenIDConnectProvidersOutput{
				OpenIDConnectProviderList: []iamtypes.OpenIDConnectProviderListEntry{{Arn: aws.String(providerArn)}},
			},
		},
		"NoMatchingURLAmong164Providers": {
			providerURL:  url,
			externalTags: map[string]string{resource.ExternalResourceTagKeyName: name},
			listOutput: &awsiam.ListOpenIDConnectProvidersOutput{
				OpenIDConnectProviderList: oidcProviderList(164),
			},
			tagOutput: &awsiam.ListOpenIDConnectProviderTagsOutput{},
			want: want{
				listCalls: 1,
			},
		},
		"MatchingURLIsLastAmong164Providers": {
			providerURL:  url,
			externalTags: map[string]string{resource.ExternalResourceTagKeyName: name},
			listOutput: &awsiam.ListOpenIDConnectProvidersOutput{
				OpenIDConnectProviderList: matchingProviders,
			},
			tagOutput: &awsiam.ListOpenIDConnectProviderTagsOutput{},
			tagsByARN: map[string]*awsiam.ListOpenIDConnectProviderTagsOutput{
				providerArn: correctOwnershipTag,
			},
			want: want{
				arn:       aws.String(providerArn),
				listCalls: 1,
				tagCalls:  1,
				tagARN:    providerArn,
			},
		},
		"MatchingURLWrongOwnershipTag": {
			providerURL:  url,
			externalTags: map[string]string{resource.ExternalResourceTagKeyName: name},
			listOutput: &awsiam.ListOpenIDConnectProvidersOutput{
				OpenIDConnectProviderList: []iamtypes.OpenIDConnectProviderListEntry{{Arn: aws.String(providerArn)}},
			},
			tagOutput: &awsiam.ListOpenIDConnectProviderTagsOutput{Tags: []iamtypes.Tag{{
				Key:   aws.String(resource.ExternalResourceTagKeyName),
				Value: aws.String("different-name"),
			}}},
			want: want{
				listCalls: 1,
				tagCalls:  1,
				tagARN:    providerArn,
			},
		},
		"MatchingURLMissingOwnershipTag": {
			providerURL:  url,
			externalTags: map[string]string{resource.ExternalResourceTagKeyName: name},
			listOutput: &awsiam.ListOpenIDConnectProvidersOutput{
				OpenIDConnectProviderList: []iamtypes.OpenIDConnectProviderListEntry{{Arn: aws.String(providerArn)}},
			},
			tagOutput: &awsiam.ListOpenIDConnectProviderTagsOutput{},
			want: want{
				listCalls: 1,
				tagCalls:  1,
				tagARN:    providerArn,
			},
		},
		"NilTagFieldsDoNotMatch": {
			providerURL:  url,
			externalTags: map[string]string{resource.ExternalResourceTagKeyName: name},
			listOutput: &awsiam.ListOpenIDConnectProvidersOutput{
				OpenIDConnectProviderList: []iamtypes.OpenIDConnectProviderListEntry{{Arn: aws.String(providerArn)}},
			},
			tagOutput: &awsiam.ListOpenIDConnectProviderTagsOutput{Tags: []iamtypes.Tag{
				{Value: aws.String(name)},
				{Key: aws.String(resource.ExternalResourceTagKeyName)},
			}},
			want: want{
				listCalls: 1,
				tagCalls:  1,
				tagARN:    providerArn,
			},
		},
		"DifferentURLWithSameNameTagIsNotAdopted": {
			providerURL:  url,
			externalTags: map[string]string{resource.ExternalResourceTagKeyName: name},
			listOutput: &awsiam.ListOpenIDConnectProvidersOutput{
				OpenIDConnectProviderList: []iamtypes.OpenIDConnectProviderListEntry{{
					Arn: aws.String("arn:aws:iam::123456789012:oidc-provider/different.example.com"),
				}},
			},
			tagOutput: correctOwnershipTag,
			want: want{
				listCalls: 1,
			},
		},
		"MalformedAndNilARNsAreSkipped": {
			providerURL:  url,
			externalTags: map[string]string{resource.ExternalResourceTagKeyName: name},
			listOutput: &awsiam.ListOpenIDConnectProvidersOutput{
				OpenIDConnectProviderList: []iamtypes.OpenIDConnectProviderListEntry{
					{},
					{Arn: aws.String("arn:123")},
					{Arn: aws.String("arn:aws:iam::123456789012:role/example.com")},
				},
			},
			tagOutput: correctOwnershipTag,
			want: want{
				listCalls: 1,
			},
		},
		"DuplicateMatchingARNIsCheckedOnce": {
			providerURL:  url,
			externalTags: map[string]string{resource.ExternalResourceTagKeyName: name},
			listOutput: &awsiam.ListOpenIDConnectProvidersOutput{
				OpenIDConnectProviderList: []iamtypes.OpenIDConnectProviderListEntry{
					{Arn: aws.String(providerArn)},
					{Arn: aws.String(providerArn)},
				},
			},
			tagOutput: correctOwnershipTag,
			want: want{
				arn:       aws.String(providerArn),
				listCalls: 1,
				tagCalls:  1,
				tagARN:    providerArn,
			},
		},
		"ListError": {
			providerURL:  url,
			externalTags: map[string]string{resource.ExternalResourceTagKeyName: name},
			listErr:      errBoom,
			want: want{
				err:       errorutils.Wrap(errBoom, errList),
				listCalls: 1,
			},
		},
		"NilListOutput": {
			providerURL:  url,
			externalTags: map[string]string{resource.ExternalResourceTagKeyName: name},
			want: want{
				err:       errors.New(errList),
				listCalls: 1,
			},
		},
		"MatchingURLTagError": {
			providerURL:  url,
			externalTags: map[string]string{resource.ExternalResourceTagKeyName: name},
			listOutput: &awsiam.ListOpenIDConnectProvidersOutput{
				OpenIDConnectProviderList: []iamtypes.OpenIDConnectProviderListEntry{{Arn: aws.String(providerArn)}},
			},
			tagErr: errBoom,
			want: want{
				err:       errorutils.Wrap(errBoom, errListTags),
				listCalls: 1,
				tagCalls:  1,
				tagARN:    providerArn,
			},
		},
		"MatchingURLNilTagOutput": {
			providerURL:  url,
			externalTags: map[string]string{resource.ExternalResourceTagKeyName: name},
			listOutput: &awsiam.ListOpenIDConnectProvidersOutput{
				OpenIDConnectProviderList: []iamtypes.OpenIDConnectProviderListEntry{{Arn: aws.String(providerArn)}},
			},
			want: want{
				err:       errors.New(errListTags),
				listCalls: 1,
				tagCalls:  1,
				tagARN:    providerArn,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			listCalls := 0
			tagCalls := 0
			client := &fake.MockOpenIDConnectProviderClient{
				MockListOpenIDConnectProviders: func(ctx context.Context, input *awsiam.ListOpenIDConnectProvidersInput, opts []func(*awsiam.Options)) (*awsiam.ListOpenIDConnectProvidersOutput, error) {
					listCalls++
					return tc.listOutput, tc.listErr
				},
				MockListOpenIDConnectProviderTags: func(ctx context.Context, input *awsiam.ListOpenIDConnectProviderTagsInput, opts []func(*awsiam.Options)) (*awsiam.ListOpenIDConnectProviderTagsOutput, error) {
					tagCalls++
					if tc.want.tagARN != "" && aws.ToString(input.OpenIDConnectProviderArn) != tc.want.tagARN {
						t.Errorf("ListOpenIDConnectProviderTags ARN = %q, want %q", aws.ToString(input.OpenIDConnectProviderArn), tc.want.tagARN)
					}
					if output, ok := tc.tagsByARN[aws.ToString(input.OpenIDConnectProviderArn)]; ok {
						return output, tc.tagErr
					}
					return tc.tagOutput, tc.tagErr
				},
			}

			e := &external{client: client}
			got, err := e.getOpenIDConnectProviderByTags(context.Background(), tc.providerURL, tc.externalTags)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("error: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.arn, got); diff != "" {
				t.Errorf("ARN: -want, +got:\n%s", diff)
			}
			if listCalls != tc.want.listCalls {
				t.Errorf("ListOpenIDConnectProviders calls = %d, want %d", listCalls, tc.want.listCalls)
			}
			if tagCalls != tc.want.tagCalls {
				t.Errorf("ListOpenIDConnectProviderTags calls = %d, want %d", tagCalls, tc.want.tagCalls)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	type want struct {
		cr     resource.Managed
		result managed.ExternalCreation
		input  *awsiam.CreateOpenIDConnectProviderInput
		err    error
	}

	cases := map[string]struct {
		args
		want
	}{
		"InvalidInput": {
			args: args{
				cr: unexpectedItem,
			},
			want: want{
				cr:  unexpectedItem,
				err: errors.New(errUnexpectedObject),
			},
		},
		"ClientError": {
			args: args{
				iam: &fake.MockOpenIDConnectProviderClient{
					MockCreateOpenIDConnectProvider: func(ctx context.Context, input *awsiam.CreateOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.CreateOpenIDConnectProviderOutput, error) {
						return &awsiam.CreateOpenIDConnectProviderOutput{}, errBoom
					},
				},
				cr: oidcProvider(withURL(url)),
			},
			want: want{
				cr:  oidcProvider(withURL(url)),
				err: errorutils.Wrap(errBoom, errCreate),
			},
		},
		"ValidInput": {
			args: args{
				iam: &fake.MockOpenIDConnectProviderClient{
					MockCreateOpenIDConnectProvider: func(ctx context.Context, input *awsiam.CreateOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.CreateOpenIDConnectProviderOutput, error) {
						return &awsiam.CreateOpenIDConnectProviderOutput{OpenIDConnectProviderArn: aws.String(providerArn)}, nil
					},
				},
				cr: oidcProvider(withURL(url),
					withThumbprintList([]string{"thumbs1", "thumbs2"}),
					withClientIDList([]string{"client1", "client2"}),
					withTags(map[string]string{key1: value1, key2: value2})),
			},
			want: want{
				cr: oidcProvider(withURL(url),
					withThumbprintList([]string{"thumbs1", "thumbs2"}),
					withClientIDList([]string{"client1", "client2"}),
					withTags(map[string]string{key1: value1, key2: value2}),
					func(provider *svcapitypes.OpenIDConnectProvider) {
						meta.SetExternalName(provider, providerArn)
					}),
				result: managed.ExternalCreation{},
				input: &awsiam.CreateOpenIDConnectProviderInput{
					ThumbprintList: []string{"thumbs1", "thumbs2"},
					Url:            &url,
					ClientIDList:   []string{"client1", "client2"},
					Tags: []iamtypes.Tag{
						{
							Key:   &key1,
							Value: &value1,
						},
						{
							Key:   &key2,
							Value: &value2,
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{client: tc.iam}
			o, err := e.Create(context.Background(), tc.args.cr)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.cr, tc.args.cr, test.EquateConditions(), sortTags); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.result, o); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if tc.want.input != nil {
				actual := tc.args.iam.MockOpenIDConnectProviderInput.CreateOIDCProviderInput
				if diff := cmp.Diff(tc.want.input, actual, createInputComparer, sortTags); diff != "" {
					t.Errorf("r: -want, +got:\n%s", diff)
				}
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	type want struct {
		cr     resource.Managed
		result managed.ExternalUpdate
		err    error
	}

	cases := map[string]struct {
		args
		want
	}{
		"InvalidInput": {
			args: args{
				cr: unexpectedItem,
			},
			want: want{
				cr:  unexpectedItem,
				err: errors.New(errUnexpectedObject),
			},
		},
		"ThumbprintUpdateError": {
			args: args{
				iam: &fake.MockOpenIDConnectProviderClient{
					MockGetOpenIDConnectProvider: func(ctx context.Context, input *awsiam.GetOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.GetOpenIDConnectProviderOutput, error) {
						return &awsiam.GetOpenIDConnectProviderOutput{
							ThumbprintList: []string{"b"},
						}, nil
					},
					MockUpdateOpenIDConnectProviderThumbprint: func(ctx context.Context, input *awsiam.UpdateOpenIDConnectProviderThumbprintInput, opts []func(*awsiam.Options)) (*awsiam.UpdateOpenIDConnectProviderThumbprintOutput, error) {
						return &awsiam.UpdateOpenIDConnectProviderThumbprintOutput{}, errBoom
					},
				},
				cr: oidcProvider(withURL(url),
					func(provider *svcapitypes.OpenIDConnectProvider) {
						provider.Spec.ForProvider.ThumbprintList = []string{"a"}
					},
				),
			},
			want: want{
				cr: oidcProvider(withURL(url),
					func(provider *svcapitypes.OpenIDConnectProvider) {
						provider.Spec.ForProvider.ThumbprintList = []string{"a"}
					}),
				err: errorutils.Wrap(errBoom, errUpdateThumbprint),
			},
		},
		"AddClientError": {
			args: args{
				iam: &fake.MockOpenIDConnectProviderClient{
					MockGetOpenIDConnectProvider: func(ctx context.Context, input *awsiam.GetOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.GetOpenIDConnectProviderOutput, error) {
						return &awsiam.GetOpenIDConnectProviderOutput{}, nil
					},
					MockAddClientIDToOpenIDConnectProvider: func(ctx context.Context, input *awsiam.AddClientIDToOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.AddClientIDToOpenIDConnectProviderOutput, error) {
						return &awsiam.AddClientIDToOpenIDConnectProviderOutput{}, errBoom
					},
				},
				cr: oidcProvider(withURL(url),
					func(provider *svcapitypes.OpenIDConnectProvider) {
						provider.Spec.ForProvider.ClientIDList = []string{"a", "b"}
					},
				),
			},
			want: want{
				cr: oidcProvider(withURL(url),
					func(provider *svcapitypes.OpenIDConnectProvider) {
						provider.Spec.ForProvider.ClientIDList = []string{"a", "b"}
					}),
				err: errorutils.Wrap(errBoom, errAddClientID),
			},
		},
		"RemoveClientError": {
			args: args{
				iam: &fake.MockOpenIDConnectProviderClient{
					MockGetOpenIDConnectProvider: func(ctx context.Context, input *awsiam.GetOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.GetOpenIDConnectProviderOutput, error) {
						return &awsiam.GetOpenIDConnectProviderOutput{
							ClientIDList: []string{"a", "b"},
						}, nil
					},
					MockRemoveClientIDFromOpenIDConnectProvider: func(ctx context.Context, input *awsiam.RemoveClientIDFromOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.RemoveClientIDFromOpenIDConnectProviderOutput, error) {
						return &awsiam.RemoveClientIDFromOpenIDConnectProviderOutput{}, errBoom
					},
				},
				cr: oidcProvider(withURL(url),
					func(provider *svcapitypes.OpenIDConnectProvider) {
						provider.Spec.ForProvider.ClientIDList = []string{"a"}
					},
				),
			},
			want: want{
				cr: oidcProvider(withURL(url),
					func(provider *svcapitypes.OpenIDConnectProvider) {
						provider.Spec.ForProvider.ClientIDList = []string{"a"}
					}),
				err: errorutils.Wrap(errBoom, errRemoveClientID),
			},
		},
		"SuccessfulUpdate": {
			args: args{
				iam: &fake.MockOpenIDConnectProviderClient{

					MockGetOpenIDConnectProvider: func(ctx context.Context, input *awsiam.GetOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.GetOpenIDConnectProviderOutput, error) {
						return &awsiam.GetOpenIDConnectProviderOutput{
							ThumbprintList: []string{"b"},
							ClientIDList:   []string{"b"},
						}, nil
					},
					MockUpdateOpenIDConnectProviderThumbprint: func(ctx context.Context, input *awsiam.UpdateOpenIDConnectProviderThumbprintInput, opts []func(*awsiam.Options)) (*awsiam.UpdateOpenIDConnectProviderThumbprintOutput, error) {
						return &awsiam.UpdateOpenIDConnectProviderThumbprintOutput{}, nil
					},
					MockAddClientIDToOpenIDConnectProvider: func(ctx context.Context, input *awsiam.AddClientIDToOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.AddClientIDToOpenIDConnectProviderOutput, error) {
						return &awsiam.AddClientIDToOpenIDConnectProviderOutput{}, nil
					},
					MockRemoveClientIDFromOpenIDConnectProvider: func(ctx context.Context, input *awsiam.RemoveClientIDFromOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.RemoveClientIDFromOpenIDConnectProviderOutput, error) {
						return &awsiam.RemoveClientIDFromOpenIDConnectProviderOutput{}, nil
					},
				},
				cr: oidcProvider(withURL(url),
					func(provider *svcapitypes.OpenIDConnectProvider) {
						provider.Spec.ForProvider.ThumbprintList = []string{"a"}
						provider.Spec.ForProvider.ClientIDList = []string{"a", "c"}
					},
				),
			},
			want: want{
				cr: oidcProvider(withURL(url),
					func(provider *svcapitypes.OpenIDConnectProvider) {
						provider.Spec.ForProvider.ThumbprintList = []string{"a"}
						provider.Spec.ForProvider.ClientIDList = []string{"a", "c"}
					}),
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{client: tc.iam}
			o, err := e.Update(context.Background(), tc.args.cr)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.cr, tc.args.cr, test.EquateConditions()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.result, o); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestUpdate_Tags(t *testing.T) {
	type want struct {
		cr         resource.Managed
		result     managed.ExternalUpdate
		err        error
		tagInput   *awsiam.TagOpenIDConnectProviderInput
		untagInput *awsiam.UntagOpenIDConnectProviderInput
	}

	cases := map[string]struct {
		args
		want
	}{
		"AddTagsError": {
			args: args{
				iam: &fake.MockOpenIDConnectProviderClient{
					MockGetOpenIDConnectProvider: func(ctx context.Context, input *awsiam.GetOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.GetOpenIDConnectProviderOutput, error) {
						return &awsiam.GetOpenIDConnectProviderOutput{}, nil
					},
					MockTagOpenIDConnectProvider: func(ctx context.Context, input *awsiam.TagOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.TagOpenIDConnectProviderOutput, error) {
						return nil, errBoom
					},
				},
				cr: oidcProvider(withTags(map[string]string{key1: value1})),
			},
			want: want{
				cr:  oidcProvider(withTags(map[string]string{key1: value1})),
				err: errorutils.Wrap(errBoom, errAddTags),
			},
		},
		"AddTagsSuccess": {
			args: args{
				iam: &fake.MockOpenIDConnectProviderClient{
					MockGetOpenIDConnectProvider: func(ctx context.Context, input *awsiam.GetOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.GetOpenIDConnectProviderOutput, error) {
						return &awsiam.GetOpenIDConnectProviderOutput{}, nil
					},
					MockTagOpenIDConnectProvider: func(ctx context.Context, input *awsiam.TagOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.TagOpenIDConnectProviderOutput, error) {
						return &awsiam.TagOpenIDConnectProviderOutput{}, nil
					},
				},
				cr: oidcProvider(
					withTags(map[string]string{key1: value1, key2: value2}),
					withExternalName(providerArn)),
			},
			want: want{
				cr: oidcProvider(
					withTags(map[string]string{key1: value1, key2: value2}),
					withExternalName(providerArn)),
				tagInput: &awsiam.TagOpenIDConnectProviderInput{
					OpenIDConnectProviderArn: &providerArn,
					Tags: []iamtypes.Tag{
						{Key: &key1, Value: &value1},
						{Key: &key2, Value: &value2},
					}},
			},
		},
		"UpdateTagsSuccess": {
			args: args{
				iam: &fake.MockOpenIDConnectProviderClient{
					MockGetOpenIDConnectProvider: func(ctx context.Context, input *awsiam.GetOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.GetOpenIDConnectProviderOutput, error) {
						return &awsiam.GetOpenIDConnectProviderOutput{
							Tags: []iamtypes.Tag{
								{Key: &key1, Value: &value1},
								{Key: &key2, Value: &value2},
							}}, nil
					},
					MockTagOpenIDConnectProvider: func(ctx context.Context, input *awsiam.TagOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.TagOpenIDConnectProviderOutput, error) {
						return &awsiam.TagOpenIDConnectProviderOutput{}, nil
					},
				},
				cr: oidcProvider(
					withTags(map[string]string{key1: value2, key2: value2}),
					withExternalName(providerArn)),
			},
			want: want{
				cr: oidcProvider(
					withTags(map[string]string{key1: value2, key2: value2}),
					withExternalName(providerArn)),
				tagInput: &awsiam.TagOpenIDConnectProviderInput{
					OpenIDConnectProviderArn: &providerArn,
					Tags: []iamtypes.Tag{
						{Key: &key1, Value: &value2},
					}},
			},
		},
		"RemoveTagsError": {
			args: args{
				iam: &fake.MockOpenIDConnectProviderClient{
					MockGetOpenIDConnectProvider: func(ctx context.Context, input *awsiam.GetOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.GetOpenIDConnectProviderOutput, error) {
						return &awsiam.GetOpenIDConnectProviderOutput{
							Tags: []iamtypes.Tag{
								{Key: &key1, Value: &value1},
								{Key: &key2, Value: &value2},
							}}, nil
					},
					MockUntagOpenIDConnectProvider: func(ctx context.Context, input *awsiam.UntagOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.UntagOpenIDConnectProviderOutput, error) {
						return nil, errBoom
					},
				},
				cr: oidcProvider(withTags(map[string]string{key1: value1})),
			},
			want: want{
				cr:  oidcProvider(withTags(map[string]string{key1: value1})),
				err: errorutils.Wrap(errBoom, errRemoveTags),
			},
		},
		"RemoveTagsSuccess": {
			args: args{
				iam: &fake.MockOpenIDConnectProviderClient{
					MockGetOpenIDConnectProvider: func(ctx context.Context, input *awsiam.GetOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.GetOpenIDConnectProviderOutput, error) {
						return &awsiam.GetOpenIDConnectProviderOutput{
							Tags: []iamtypes.Tag{
								{Key: &key1, Value: &value1},
								{Key: &key2, Value: &value2},
							}}, nil
					},
					MockUntagOpenIDConnectProvider: func(ctx context.Context, input *awsiam.UntagOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.UntagOpenIDConnectProviderOutput, error) {
						return nil, nil
					},
				},
				cr: oidcProvider(withExternalName(providerArn)),
			},
			want: want{
				cr: oidcProvider(withExternalName(providerArn)),
				untagInput: &awsiam.UntagOpenIDConnectProviderInput{
					OpenIDConnectProviderArn: &providerArn,
					TagKeys:                  []string{key1, key2},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{client: tc.iam}
			o, err := e.Update(context.Background(), tc.args.cr)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.cr, tc.args.cr, test.EquateConditions(), sortTags); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.result, o); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if tc.want.tagInput != nil {
				if diff := cmp.Diff(tc.want.tagInput, tc.iam.MockOpenIDConnectProviderInput.TagOpenIDConnectProviderInput, tagInputComparer, sortIAMTags); diff != "" {
					t.Errorf("r: -want, +got:\n%s", diff)
				}
			}
			if tc.want.untagInput != nil {
				if diff := cmp.Diff(tc.want.untagInput, tc.iam.MockOpenIDConnectProviderInput.UntagOpenIDConnectProviderInput, untagInputComparer, sortStrings); diff != "" {
					t.Errorf("r: -want, +got:\n%s", diff)
				}
			}
		})
	}
}

func TestDelete(t *testing.T) {
	type want struct {
		cr  resource.Managed
		err error
	}

	cases := map[string]struct {
		args
		want
	}{
		"InvalidInput": {
			args: args{
				cr: unexpectedItem,
			},
			want: want{
				cr:  unexpectedItem,
				err: errors.New(errUnexpectedObject),
			},
		},
		"ClientError": {
			args: args{
				iam: &fake.MockOpenIDConnectProviderClient{
					MockDeleteOpenIDConnectProvider: func(ctx context.Context, input *awsiam.DeleteOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.DeleteOpenIDConnectProviderOutput, error) {
						return &awsiam.DeleteOpenIDConnectProviderOutput{}, errBoom
					},
				},
				cr: oidcProvider(withURL(url)),
			},
			want: want{
				cr:  oidcProvider(withURL(url)),
				err: errorutils.Wrap(errBoom, errDelete),
			},
		},
		"ValidInput": {
			args: args{
				iam: &fake.MockOpenIDConnectProviderClient{
					MockDeleteOpenIDConnectProvider: func(ctx context.Context, input *awsiam.DeleteOpenIDConnectProviderInput, opts []func(*awsiam.Options)) (*awsiam.DeleteOpenIDConnectProviderOutput, error) {
						return &awsiam.DeleteOpenIDConnectProviderOutput{}, nil
					},
				},
				cr: oidcProvider(withURL(url)),
			},
			want: want{
				cr: oidcProvider(withURL(url)),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{client: tc.iam}
			err := e.Delete(context.Background(), tc.args.cr)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.cr, tc.args.cr, test.EquateConditions()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

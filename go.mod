module github.com/crossplane/provider-aws

go 1.16

require (
	github.com/aws/aws-sdk-go v1.37.10
	github.com/aws/aws-sdk-go-v2 v1.11.0
	github.com/aws/aws-sdk-go-v2/config v1.10.0
	github.com/aws/aws-sdk-go-v2/credentials v1.6.0
	github.com/aws/aws-sdk-go-v2/service/acm v1.8.0
	github.com/aws/aws-sdk-go-v2/service/acmpca v1.10.0
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.21.0
	github.com/aws/aws-sdk-go-v2/service/ecr v1.9.0
	github.com/aws/aws-sdk-go-v2/service/eks v1.12.0
	github.com/aws/aws-sdk-go-v2/service/elasticache v1.13.0
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing v1.8.0
	github.com/aws/aws-sdk-go-v2/service/iam v1.12.0
	github.com/aws/aws-sdk-go-v2/service/rds v1.11.0
	github.com/aws/aws-sdk-go-v2/service/redshift v1.13.0
	github.com/aws/aws-sdk-go-v2/service/route53 v1.13.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.18.0
	github.com/aws/aws-sdk-go-v2/service/sns v1.10.0
	github.com/aws/aws-sdk-go-v2/service/sqs v1.11.0
	github.com/aws/aws-sdk-go-v2/service/sts v1.9.0
	github.com/aws/smithy-go v1.9.0
	github.com/crossplane/crossplane-runtime v0.19.3
	github.com/crossplane/crossplane-tools v0.0.0-20220310165030-1f43fc12793e
	github.com/crossplane/provider-aws/apis/vpcpeering/v1alpha1 v0.0.0-00010101000000-000000000000
	github.com/evanphx/json-patch v4.12.0+incompatible
	github.com/go-ini/ini v1.46.0
	github.com/golang/mock v1.6.0
	github.com/google/go-cmp v0.5.9
	github.com/mitchellh/copystructure v1.0.0
	github.com/onsi/gomega v1.24.2
	github.com/pkg/errors v0.9.1
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	k8s.io/api v0.26.1
	k8s.io/apimachinery v0.26.1
	k8s.io/client-go v0.26.1
	sigs.k8s.io/controller-runtime v0.14.1
	sigs.k8s.io/controller-tools v0.11.1
)

replace github.com/crossplane/provider-aws/apis/vpcpeering/v1alpha1 => ./apis/vpcpeering/v1alpha1

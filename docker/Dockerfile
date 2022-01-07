FROM alpine:3.14 as base

# this fils is generate by go build
COPY crossplane-aws-provider  /crossplane-aws-provider
ENTRYPOINT ["/crossplane-aws-provider"]

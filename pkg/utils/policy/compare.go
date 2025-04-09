package policy

import (
	"net/url"

	"github.com/google/go-cmp/cmp"
)

// ArePoliciesEqal determines if the two Policy objects can be considered
// equal.
func ArePoliciesEqal(a, b *Policy) (equal bool, diff string) {
	diff = cmp.Diff(a, b)
	return diff == "", diff
}

func IsPolicyDocumentUpToDate(a, b string) (bool, string, error) {
	ua, err := url.QueryUnescape(a)
	if err != nil {
		return false, "", err
	}
	ub, err := url.QueryUnescape(b)
	if err != nil {
		return false, "", err
	}
	pa, err := ParsePolicyString(ua)
	if err != nil {
		return false, "", err
	}
	pb, err := ParsePolicyString(ub)
	if err != nil {
		return false, "", err
	}

	areEqual, diff := ArePoliciesEqal(&pa, &pb)
	return areEqual, diff, nil
}

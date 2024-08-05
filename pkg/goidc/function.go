package goidc

import (
	"crypto/rand"
	"math/big"
	"slices"
	"strings"
	"time"
)

// TimestampNow returns the current timestamp. The result is always on UTC time.
func TimestampNow() int {
	return int(time.Now().Unix())
}

func SplitStringWithSpaces(s string) []string {
	slice := []string{}
	if strings.ReplaceAll(strings.Trim(s, " "), " ", "") != "" {
		slice = strings.Split(s, " ")
	}

	return slice
}

func RandomString(n int) (string, error) {
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(ClientSecretCharset))))
		if err != nil {
			return "", err
		}
		ret[i] = ClientSecretCharset[num.Int64()]
	}

	return string(ret), nil
}

func ContainsAllScopes(scopesSuperSet string, scopesSubSet string) bool {
	return ContainsAll(SplitStringWithSpaces(scopesSuperSet), SplitStringWithSpaces(scopesSubSet)...)
}

func ContainsAll[T comparable](superSet []T, subSet ...T) bool {
	for _, e := range subSet {
		if !slices.Contains(superSet, e) {
			return false
		}
	}

	return true
}

func ScopesContainsOpenID(scopes string) bool {
	return slices.Contains(SplitStringWithSpaces(scopes), ScopeOpenID.ID)
}

func ScopesContainsOfflineAccess(scopes string) bool {
	return slices.Contains(SplitStringWithSpaces(scopes), ScopeOffilineAccess.ID)
}

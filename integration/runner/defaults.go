/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package runner

import (
//	"context"
	"encoding/base32"
//	"net/http"
	"time"

	"github.com/hyperledger/fabric/common/util"
)

const DefaultStartTimeout = 30 * time.Second

// A NameFunc is used to generate container names.
type NameFunc func() string

// DefaultNamer is the default naming function.
var DefaultNamer NameFunc = UniqueName

// UniqueName is a NamerFunc that generates base-32 enocded UUIDs for container
// names.
func UniqueName() string {
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(util.GenerateBytesUUID())
}

//func endpointReady(ctx context.Context, url string) bool {
//	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
//	defer cancel()
//
//	req, err := http.NewRequest(http.MethodGet, url, nil)
//	if err != nil {
//		return false
//	}
//
//	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
//	return err == nil && resp.StatusCode == http.StatusOK
//}

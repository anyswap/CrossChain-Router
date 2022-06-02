package ripple

import (
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/tokens/ripple/rubblelabs/ripple/data"
)

// ParsePaths parse paths
func ParsePaths(s string) (*data.PathSet, error) {
	ps := data.PathSet{}
	for _, pathStr := range strings.Split(s, ",") {
		path, err := data.NewPath(pathStr)
		if err != nil {
			return nil, err
		}
		ps = append(ps, path)
	}
	return &ps, nil
}

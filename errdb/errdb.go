package errdb

import (
	"embed"
	"io"

	"github.com/GlobeMC/mcla/ghdb"
)

//go:embed database/*.json database/errors/*.json database/solutions/*.json
var embedFS embed.FS

//go:embed database/version.json
var VersionJson []byte

var DefaultErrDB = &ghdb.ErrDB{
	Fetch: func(path string) (io.ReadCloser, error) {
		return embedFS.Open(path)
	},
}

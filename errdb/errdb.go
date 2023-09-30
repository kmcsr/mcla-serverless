
package msldb

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"

	"github.com/kmcsr/mcla"
)

const syntaxVersion = 0 // 0 means dev

type UnsupportSyntaxErr struct {
	Version int
}

func (e *UnsupportSyntaxErr)Error()(string){
	return fmt.Sprintf("MCLA-DB syntax version %d is not supported, please update the application", e.Version)
}

type versionDataT struct {
	Major         int `json:"major"`
	Minor         int `json:"minor"`
	Patch         int `json:"patch"`
	ErrorIncId    int `json:"errorIncId"`
	SolutionIncId int `json:"solutionIncId"`
}

type fsErrDB struct {
	FS fs.FS
	cachedVersion versionDataT
}

var _ mcla.ErrorDB = (*fsErrDB)(nil)

func (db *fsErrDB)getErrorDesc(id int)(desc *mcla.ErrorDesc, err error){
	r, err := db.FS.Open(fmt.Sprintf("database/errors/%d.json", id))
	if err != nil {
		return
	}
	defer r.Close()
	if err = json.NewDecoder(r).Decode(&desc); err != nil {
		return
	}
	return
}

func (db *fsErrDB)checkUpdate()(err error){
	if db.cachedVersion == (versionDataT{}) {
		var fd io.ReadCloser
		if fd, err = db.FS.Open("database/version.json"); err != nil {
			return
		}
		defer fd.Close()
		var v versionDataT
		if err = json.NewDecoder(fd).Decode(&v); err != nil {
			return
		}
		if v.Major != syntaxVersion {
			return &UnsupportSyntaxErr{ v.Major }
		}
		db.cachedVersion = v
	}
	return
}

func (db *fsErrDB)GetVersion()(v versionDataT){
	if err := db.checkUpdate(); err != nil {
		return
	}
	return db.cachedVersion
}

func (db *fsErrDB)ForEachErrors(callback func(*mcla.ErrorDesc)(error))(err error){
	if err = db.checkUpdate(); err != nil {
		return
	}
	for i := 1; i <= db.cachedVersion.ErrorIncId; i++ {
		var desc *mcla.ErrorDesc
		if desc, err = db.getErrorDesc(i); err != nil {
			return
		}
		if err = callback(desc); err != nil {
			return
		}
	}
	return
}

func (db *fsErrDB)GetSolution(id int)(sol *mcla.SolutionDesc, err error){
	r, err := db.FS.Open(fmt.Sprintf("database/solutions/%d.json", id))
	if err != nil {
		return
	}
	defer r.Close()
	if err = json.NewDecoder(r).Decode(&sol); err != nil {
		return
	}
	return
}

//go:embed database/*.json database/errors/*.json database/solutions/*.json
var embedFS embed.FS

var DefaultErrDB = &fsErrDB{
	FS: embedFS,
}

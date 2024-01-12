// Serverless function web API
// Copyright (C) 2023  zyxkad@gmail.com
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/GlobeMC/mcla"
	"github.com/kmcsr/mcla-serverless/errdb"
)

type Map = map[string]any

func writeError(w http.ResponseWriter, status int, err string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Map{
		"status": "error",
		"error":  err,
	})
}

func Handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodPost:
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
	defer r.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	r.ParseForm()

	var (
		matchRate float32 = 0.5
	)
	{
		query := r.Form
		if query.Has("match") {
			match, err := strconv.ParseFloat(query.Get("match"), 32)
			if err != nil {
				writeError(w, http.StatusBadRequest,
					fmt.Sprintf("Error when parsing queue \"match\": %v", err))
				return
			}
			matchRate = (float32)(match)
		}
	}

	if mediatype, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Error when parsing Content-Type: %v", err))
	} else if mediatype == "multipart/form-data" {
		writeError(w, http.StatusUnsupportedMediaType, "multifile doesn't supported yet")
		return
	} else if mediatype == "text/plain" || mediatype == "application/octet-stream" {
	} else if mediatype == "application/x-www-form-urlencoded" {
	} else {
		writeError(w, http.StatusUnsupportedMediaType, fmt.Sprintf("Unsupport content type %q", mediatype))
		return
	}

	resCh, errCtx := defaultAnalyzer.DoLogStream(r.Context(), r.Body)
	ress := make([]*mcla.ErrorResult, 0, 8)
LOOP_RES:
	for {
		select {
		case res := <-resCh:
			if res == nil { // done
				break LOOP_RES
			}
			res.Matched = filtered(res.Matched, func(m mcla.SolutionPossibility) bool {
				return m.Match >= matchRate
			})
			ress = append(ress, res)
		case <-errCtx.Done():
			err := context.Cause(errCtx)
			writeError(w, http.StatusInternalServerError,
				fmt.Sprintf("Error when analyzing: %v", err))
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(Map{
		"status":      "ok",
		"_db_version": (json.RawMessage)(errdb.VersionJson),
		"res":         ress,
	}); err != nil {
		fmt.Println("Error when encoding to ResponseWriter:", err)
	}
}

func filtered[T comparable](arr []T, cb func(T) bool) (res []T) {
	res = make([]T, 0, len(arr)/2)
	for _, v := range arr {
		if cb(v) {
			res = append(res, v)
		}
	}
	return
}

func analyzeLogErrors(r io.Reader) (<-chan *mcla.ErrorResult, <-chan error) {
	resCh := make(chan *mcla.ErrorResult, 3)
	errCh := make(chan error, 0)
	go func() {
		defer close(resCh)
		jerrCh, errC := mcla.ScanJavaErrorsIntoChan(r)
	LOOP:
		for {
			select {
			case err := <-errC:
				errCh <- err
				return
			case jerr := <-jerrCh:
				if jerr == nil {
					break LOOP
				}
				var err error
				res := &mcla.ErrorResult{
					Error: jerr,
				}
				if res.Matched, err = defaultAnalyzer.DoError(jerr); err != nil {
					errCh <- err
					return
				}
				resCh <- res
			}
		}
	}()
	return resCh, errCh
}

var defaultAnalyzer = &mcla.Analyzer{
	DB: errdb.DefaultErrDB,
}

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
	"strings"
	"net/http"
)

func setCORSHeader(w http.ResponseWriter, r *http.Request){
	origin := r.Header.Get("Origin")
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Methods", strings.Join([]string{
			http.MethodOptions, http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
		}, ","))
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
}


func Handler(w http.ResponseWriter, r *http.Request) {
	setCORSHeader(w, r)
	u := *r.URL
	u.Scheme = "https"
	u.Host = "pastemcapi.crashmc.com"
	u.Path, _ = strings.CutPrefix(u.Path, "/paste")
	http.Redirect(w, r, u.String(), http.StatusFound)
}

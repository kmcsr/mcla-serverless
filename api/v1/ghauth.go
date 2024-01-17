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
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const GITHUB_AUTH_TOKEN_URL = "https://github.com/login/oauth/access_token"

var CLIENT_ID = os.Getenv("CLIENT_ID")
var CLIENT_SECRET = os.Getenv("CLIENT_SECRET")

func Handler(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	data := url.Values{
		"client_id":     {CLIENT_ID},
		"client_secret": {CLIENT_SECRET},
		"code":          {code},
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, GITHUB_AUTH_TOKEN_URL, strings.NewReader(data.Encode()))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(([]byte)(err.Error()))
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(([]byte)(err.Error()))
		return
	}
	defer res.Body.Close()
	w.WriteHeader(res.StatusCode)
	io.Copy(w, res.Body)
}

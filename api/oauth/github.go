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
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const GITHUB_AUTH_PAGE = "https://github.com/login/oauth/authorize"
const GITHUB_AUTH_TOKEN_URL = "https://github.com/login/oauth/access_token"

var CLIENT_ID = os.Getenv("CLIENT_ID")
var CLIENT_SECRET = os.Getenv("CLIENT_SECRET")

var hmacKey = func() (key []byte) {
	keyStr := os.Getenv("HMAC_KEY")
	if keyStr == "" {
		panic("You must set the envionment variable 'HMAC_KEY' as a non-empty base64 value")
	}
	key, err := base64.RawStdEncoding.DecodeString(keyStr)
	if err != nil {
		panic("Cannot decode hmac key: " + err.Error())
	}
	return
}()

const jwtIssuer = "api.crashmc.com"
const preAuthTokenId = "github_pre_oauth_token"
const afterAuthTokenId = "github_after_oauth_token"

func setCORSHeader(w http.ResponseWriter, r *http.Request){
	origin := r.Header.Get("Origin")
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
		w.Header().Set("Access-Control-Allow-Methods", http.MethodOptions + ", " + http.MethodPost)
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
}

func Handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodOptions:
		setCORSHeader(w, r)
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		query := r.URL.Query()
		state := query.Get("state")
		if code := query.Get("code"); code != "" {
			// If it redirected from github auth
			tokenCookie, _ := r.Cookie(preAuthTokenId)
			if tokenCookie == nil {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write(([]byte)("Pre auth token missing"))
				return
			}
			t, err := jwt.Parse(
				tokenCookie.Value,
				func(t *jwt.Token) (any, error) {
					if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, fmt.Errorf("Unexpected signing method: %v", t.Header["alg"])
					}
					return hmacKey, nil
				},
				jwt.WithSubject(preAuthTokenId),
				jwt.WithIssuedAt(),
				jwt.WithIssuer(jwtIssuer),
			)
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write(([]byte)("Pre auth token wrong"))
				return
			}
			claims, ok := t.Claims.(jwt.MapClaims)
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write(([]byte)("Pre auth token wrong"))
				return
			}
			if jti, ok := claims["jti"].(string); !ok || jti != state {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write(([]byte)("Pre auth token state not match"))
				return
			}
			redirectUri, _ := claims["redirect_uri"].(string)
			target, err := url.ParseRequestURI(redirectUri)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write(([]byte)("Cannot parse redirect uri" + err.Error()))
				return
			}

			http.SetCookie(w, &http.Cookie{
				Name:   preAuthTokenId,
				Value:  "",
				Path:   "/api/oauth/github",
				MaxAge: -1,
			})
			now := time.Now()
			tkExpires := now.Add(time.Minute * 5)
			jti, err := genRandB64(66)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write(([]byte)(err.Error()))
				return
			}
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"jti":  jti,
				"sub":  afterAuthTokenId,
				"iss":  jwtIssuer,
				"iat":  now.Unix(),
				"exp":  tkExpires.Unix(),
				"code": code,
			})
			tokenStr, err := token.SignedString(hmacKey)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write(([]byte)(err.Error()))
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:     afterAuthTokenId,
				Value:    tokenStr,
				Path:     "/api/oauth/github/",
				Expires:  tkExpires,
				Secure:   true,
				HttpOnly: true,
				SameSite: http.SameSiteNoneMode,
			})
			stateParam, _ := claims["state"].(string)
			tQuery := target.Query()
			tQuery.Set("code", jti)
			tQuery.Set("state", stateParam)
			target.RawQuery = tQuery.Encode()
			http.Redirect(w, r, target.String(), http.StatusFound)
		} else {
			scope := query.Get("scope")
			redirectUri := query.Get("redirect_uri")
			if redirectUri == "" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write(([]byte)(`"redirect_uri" param missing`))
				return
			}
			target, err := url.ParseRequestURI(GITHUB_AUTH_PAGE)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write(([]byte)(err.Error()))
				return
			}
			now := time.Now()
			tkExpires := now.Add(time.Minute * 10)
			jti, err := genRandB64(66)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write(([]byte)(err.Error()))
				return
			}
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"jti":          jti,
				"sub":          preAuthTokenId,
				"iss":          jwtIssuer,
				"iat":          now.Unix(),
				"exp":          tkExpires.Unix(),
				"state":        state,
				"redirect_uri": redirectUri,
			})
			tokenStr, err := token.SignedString(hmacKey)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write(([]byte)(err.Error()))
				return
			}
			tQuery := url.Values{
				"client_id": {CLIENT_ID},
				"state":     {jti},
			}
			if scope != "" {
				tQuery.Set("scope", scope)
			}
			target.RawQuery = tQuery.Encode()
			http.SetCookie(w, &http.Cookie{
				Name:     preAuthTokenId,
				Value:    tokenStr,
				Path:     "/api/oauth/github/",
				Expires:  tkExpires,
				Secure:   true,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
			http.Redirect(w, r, target.String(), http.StatusFound)
		}
	case http.MethodPost:
		setCORSHeader(w, r)
		tokenCookie, _ := r.Cookie(afterAuthTokenId)
		if tokenCookie == nil {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write(([]byte)("Auth token missing"))
			return
		}
		t, err := jwt.Parse(
			tokenCookie.Value,
			func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("Unexpected signing method: %v", t.Header["alg"])
				}
				return hmacKey, nil
			},
			jwt.WithSubject(afterAuthTokenId),
			jwt.WithIssuedAt(),
			jwt.WithIssuer(jwtIssuer),
		)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write(([]byte)("Auth token wrong"))
			return
		}
		claims, ok := t.Claims.(jwt.MapClaims)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write(([]byte)("Auth token wrong"))
			return
		}
		code := r.FormValue("code")
		if jti, _ := claims["jti"].(string); jti == "" || jti != code {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write(([]byte)("Auth token code not match"))
			return
		}
		ghCode, _ := claims["code"].(string)
		data := url.Values{
			"client_id":     {CLIENT_ID},
			"client_secret": {CLIENT_SECRET},
			"code":          {ghCode},
		}
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, GITHUB_AUTH_TOKEN_URL, strings.NewReader(data.Encode()))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write(([]byte)(err.Error()))
			return
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", r.Header.Get("Accept"))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write(([]byte)(err.Error()))
			return
		}
		defer res.Body.Close()
		w.Header().Set("Content-Type", res.Header.Get("Content-Type"))
		w.Header().Set("Content-Length", res.Header.Get("Content-Length"))
		w.WriteHeader(res.StatusCode)
		io.Copy(w, res.Body)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write(([]byte)("method not allowed"))
	}
}

func genRandB64(n int) (s string, err error) {
	buf := make([]byte, n)
	if _, err = rand.Read(buf); err != nil {
		return
	}
	s = base64.RawURLEncoding.EncodeToString(buf)
	return
}

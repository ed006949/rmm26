package aaa

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"rmm26/src/db"

	"github.com/nmcclain/ldap"
	"github.com/rs/zerolog/log"
)

type LDAPServer struct {
	server *ldap.Server
	db     *db.Client
}

func NewLDAPServer(client *db.Client) *LDAPServer {
	var (
		s  *ldap.Server
		ls *LDAPServer
	)

	s = ldap.NewServer()
	ls = &LDAPServer{
		server: s,
		db:     client,
	}

	s.BindFunc("", ls)
	s.SearchFunc("", ls)

	return ls
}

func (ls *LDAPServer) ListenAndServe(addr string) error {
	log.Info().Str("addr", addr).Msg("Starting LDAP server")
	return ls.server.ListenAndServe(addr)
}

func (ls *LDAPServer) Bind(bindDN, bindSimplePw string, conn net.Conn) (ldap.LDAPResultCode, error) {
	var (
		ctx   context.Context
		key   string
		val   string
		err   error
		entry map[string]any
		attrs map[string]any
		pass  []any
		ok    bool
	)

	log.Info().Str("bindDN", bindDN).Msg("LDAP Bind request")

	switch {
	case bindDN == "" && bindSimplePw == "":
		return ldap.LDAPResultSuccess, nil
	}

	ctx = context.Background()
	key = "dn:" + bindDN
	val, err = ls.db.Get(ctx, key)
	switch {
	case err != nil:
		log.Error().Err(err).Str("dn", bindDN).Msg("Bind failed: DN not found")
		return ldap.LDAPResultInvalidCredentials, nil
	}

	err = json.Unmarshal([]byte(val), &entry)
	switch {
	case err != nil:
		log.Error().Err(err).Msg("Failed to unmarshal entry")
		return ldap.LDAPResultOperationsError, nil
	}

	attrs, ok = entry["attributes"].(map[string]any)
	switch {
	case !ok:
		return ldap.LDAPResultInvalidCredentials, nil
	}

	pass, ok = attrs["userPassword"].([]any)
	switch {
	case !ok || len(pass) == 0:
		return ldap.LDAPResultInvalidCredentials, nil
	}

	switch {
	case pass[0].(string) == bindSimplePw:
		return ldap.LDAPResultSuccess, nil
	default:
		return ldap.LDAPResultInvalidCredentials, nil
	}
}

func (ls *LDAPServer) Search(boundDN string, searchReq ldap.SearchRequest, conn net.Conn) (ldap.ServerSearchResult, error) {
	var (
		ctx     context.Context
		res     ldap.ServerSearchResult
		err     error
		entries []string
	)

	log.Info().
		Str("baseDN", searchReq.BaseDN).
		Str("filter", searchReq.Filter).
		Msg("LDAP Search request")

	ctx = context.Background()

	// Simple implementation: if baseDN is provided and no filter, return that entry.
	// For full search we need to use RediSearch.

	switch {
	case searchReq.Filter == "(objectClass=*)":
		entries, err = ls.searchAll(ctx, searchReq.BaseDN)
	default:
		// TODO: Implement LDAP filter to RediSearch query conversion
		entries, err = ls.searchByFilter(ctx, searchReq.BaseDN, searchReq.Filter)
	}

	switch {
	case err != nil:
		return ldap.ServerSearchResult{ResultCode: ldap.LDAPResultOperationsError}, err
	}

	for _, eStr := range entries {
		var (
			entryData map[string]any
			ldapEntry *ldap.Entry
			dn        string
			attrs     map[string]any
		)

		err = json.Unmarshal([]byte(eStr), &entryData)
		switch {
		case err != nil:
			continue
		}

		dn, _ = entryData["dn"].(string)
		ldapEntry = &ldap.Entry{
			DN:         dn,
			Attributes: []*ldap.EntryAttribute{},
		}

		attrs, _ = entryData["attributes"].(map[string]any)
		for k, v := range attrs {
			var (
				vals []string
			)
			switch vv := v.(type) {
			case []any:
				for _, vvv := range vv {
					vals = append(vals, fmt.Sprint(vvv))
				}
			default:
				vals = append(vals, fmt.Sprint(vv))
			}
			ldapEntry.Attributes = append(ldapEntry.Attributes, &ldap.EntryAttribute{
				Name:   k,
				Values: vals,
			})
		}
		res.Entries = append(res.Entries, ldapEntry)
	}

	res.ResultCode = ldap.LDAPResultSuccess
	return res, nil
}

func (ls *LDAPServer) searchAll(ctx context.Context, baseDN string) ([]string, error) {
	var (
		query string
	)

	query = "*"
	switch {
	case baseDN != "":
		query = fmt.Sprintf("@dn:{%s}", baseDN)
	}

	return ls.db.Search(ctx, "idx:dn", query)
}

func (ls *LDAPServer) searchByFilter(ctx context.Context, baseDN, filter string) ([]string, error) {
	var (
		query string
	)

	query = ls.translateFilter(filter)
	switch {
	case baseDN != "":
		query = fmt.Sprintf("@dn:{%s} %s", baseDN, query)
	}

	return ls.db.Search(ctx, "idx:dn", query)
}

func (ls *LDAPServer) translateFilter(filter string) string {
	var (
		res string
	)

	// Basic translation logic for simple filters
	// (attr=val) -> @attributes.attr:{val}
	// (objectClass=val) -> @objectClass:{val}

	switch {
	case strings.HasPrefix(filter, "(objectClass="):
		res = strings.TrimPrefix(filter, "(objectClass=")
		res = strings.TrimSuffix(res, ")")
		return fmt.Sprintf("@objectClass:{%s}", res)
	case strings.HasPrefix(filter, "(") && strings.Contains(filter, "="):
		var (
			parts []string
			attr  string
			val   string
		)
		parts = strings.Split(strings.Trim(filter, "()"), "=")
		switch {
		case len(parts) == 2:
			attr = parts[0]
			val = parts[1]
			return fmt.Sprintf("@attributes.%s:{%s}", attr, val)
		}
	}

	return "*"
}

package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (c *Client) Accounts(ctx context.Context) (*protocol.AccountList, error) {
	var out protocol.AccountList
	if err := c.do(ctx, http.MethodGet, protocol.PathAccounts, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
func (c *Client) SaveAccount(ctx context.Context, id string, a protocol.Account) (*protocol.Account, error) {
	return c.SaveAccountCredentials(ctx, id, protocol.AccountCredentials{Account: a})
}
func (c *Client) SaveAccountCredentials(ctx context.Context, id string, a protocol.AccountCredentials) (*protocol.Account, error) {
	method, path := http.MethodPost, protocol.PathAccounts
	if id != "" {
		method, path = http.MethodPut, path+"/"+url.PathEscape(id)
	}
	var out protocol.Account
	if err := c.do(ctx, method, path, a, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
func (c *Client) SetAccountPassword(ctx context.Context, id, username, password string) error {
	return c.do(ctx, http.MethodPut, protocol.PathAccounts+"/"+url.PathEscape(id)+"/password", map[string]string{"username": username, "password": password}, nil)
}
func (c *Client) AccountSecurity(ctx context.Context, id string) (*protocol.AccountSecurity, error) {
	var out protocol.AccountSecurity
	if err := c.do(ctx, http.MethodGet, protocol.PathAccounts+"/"+url.PathEscape(id)+"/security", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
func (c *Client) AccountSecurityAction(ctx context.Context, id string, v protocol.AccountSecurityAction) error {
	return c.do(ctx, http.MethodPost, protocol.PathAccounts+"/"+url.PathEscape(id)+"/security", v, nil)
}
func (c *Client) IdentityBindings(ctx context.Context, id string) (*protocol.BindingList, error) {
	var out protocol.BindingList
	if err := c.do(ctx, http.MethodGet, protocol.PathAccounts+"/"+url.PathEscape(id)+"/identities", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
func (c *Client) BindIdentity(ctx context.Context, id string, i protocol.Identity, remove bool) error {
	method := http.MethodPost
	if remove {
		method = http.MethodDelete
	}
	return c.do(ctx, method, protocol.PathAccounts+"/"+url.PathEscape(id)+"/identities", i, nil)
}

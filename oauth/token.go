package oauth

import (
	"github.com/zalando/go-keyring"
)

const SERVICE = "echo"

type TokenStore struct {
	Service string
}

func (store *TokenStore) Save(provider, accountID string, token []byte) error {
	err := keyring.Set(store.Service, accountID, string(token))
	return err
}

func (store *TokenStore) Load(provider, accountID string) ([]byte, error) {
	token, err := keyring.Get(store.Service, accountID)
	return []byte(token), err
}
func (store *TokenStore) Delete(provider, accountID string) error {
	err := keyring.Delete(store.Service, accountID)
	return err
}

// user is a client library like the calender library, it should be able to request a token identified by provider and account_id

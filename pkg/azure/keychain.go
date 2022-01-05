package azure

import (
	"github.com/chrismellard/docker-credential-acr-env/pkg/credhelper"
	"github.com/google/go-containerregistry/pkg/authn"
)

// Keychain is a keychain that emulates the ACR env credential helper.
var Keychain = authn.NewKeychainFromHelper(credhelper.NewACRCredentialsHelper())

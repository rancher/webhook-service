package service

import (
	"crypto/rsa"
	"fmt"

	"github.com/rancher/rancher-auth-service/util"
	"github.com/urfave/cli"
)

func GetKeys(c *cli.Context) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	var PrivateKey *rsa.PrivateKey
	var PublicKey *rsa.PublicKey
	privateKeyFile := c.GlobalString("rsa-private-key-file")
	privateKeyFileContents := c.GlobalString("rsa-private-key-contents")

	if privateKeyFile != "" && privateKeyFileContents != "" {
		return nil, nil, fmt.Errorf("Can't specify both, file and contents, halting")
	}
	if privateKeyFile != "" {
		PrivateKey = util.ParsePrivateKey(privateKeyFile)
	} else if privateKeyFileContents != "" {
		PrivateKey = util.ParsePrivateKeyContents(privateKeyFileContents)
	} else {
		return nil, nil, fmt.Errorf("Please provide either rsa-private-key-file or rsa-private-key-contents, halting")
	}

	publicKeyFile := c.GlobalString("rsa-public-key-file")
	publicKeyFileContents := c.GlobalString("rsa-public-key-contents")

	if publicKeyFile != "" && publicKeyFileContents != "" {
		return nil, nil, fmt.Errorf("Can't specify both, file and contents, halting")
	}
	if publicKeyFile != "" {
		PublicKey = util.ParsePublicKey(publicKeyFile)
	} else if publicKeyFileContents != "" {
		PublicKey = util.ParsePublicKeyContents(publicKeyFileContents)
	} else {
		return nil, nil, fmt.Errorf("Please provide either rsa-public-key-file or rsa-public-key-contents, halting")
	}

	return PrivateKey, PublicKey, nil
}

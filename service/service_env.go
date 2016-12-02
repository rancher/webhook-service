package service

import (
	"crypto/rsa"
	"fmt"
	"github.com/rancher/rancher-auth-service/util"
	"github.com/urfave/cli"
)

//PrivateKey to be used for jwt creation
var PrivateKey *rsa.PrivateKey

//PublicKey to be used for jwt decode
var PublicKey *rsa.PublicKey

func SetEnv(c *cli.Context) error {
	privateKeyFile := c.GlobalString("rsa-private-key-file")
	privateKeyFileContents := c.GlobalString("rsa-private-key-contents")

	if privateKeyFile != "" && privateKeyFileContents != "" {
		return fmt.Errorf("Can't specify both, file and contents, halting")
	}
	if privateKeyFile != "" {
		PrivateKey = util.ParsePrivateKey(privateKeyFile)
	} else if privateKeyFileContents != "" {
		PrivateKey = util.ParsePrivateKeyContents(privateKeyFileContents)
	} else {
		return fmt.Errorf("Please provide either rsa-private-key-file or rsa-private-key-contents, halting")
	}

	publicKeyFile := c.GlobalString("rsa-public-key-file")
	publicKeyFileContents := c.GlobalString("rsa-public-key-contents")

	if publicKeyFile != "" && publicKeyFileContents != "" {
		return fmt.Errorf("Can't specify both, file and contents, halting")
	}
	if publicKeyFile != "" {
		PublicKey = util.ParsePublicKey(publicKeyFile)
	} else if publicKeyFileContents != "" {
		PublicKey = util.ParsePublicKeyContents(publicKeyFileContents)
	} else {
		return fmt.Errorf("Please provide either rsa-public-key-file or rsa-public-key-contents, halting")
	}

	return nil
}

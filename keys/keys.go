package keys

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"log"
	"os"

	"golang.org/x/crypto/ssh"
)

//MakeSSHKeyPair creats a pair of ssh keys
func MakeSSHKeyPair(pubKeyPath, privateKeyPath string) error {
	exist, err := checkIfKeysExist(pubKeyPath, privateKeyPath)
	if err != nil {
		return err
	}
	if exist {
		return nil
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	// generate and write private key as PEM
	privateKeyFile, err := os.Create(privateKeyPath)
	defer privateKeyFile.Close()
	if err != nil {
		return err
	}
	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		return err
	}

	// generate and write public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(pubKeyPath, ssh.MarshalAuthorizedKey(pub), 0655)
}

func checkIfKeysExist(pubKeyPath, privateKeyPath string) (bool, error) {
	var privateKeyExists bool
	var publicKeyExists bool
	if _, err := os.Stat(privateKeyPath); err == nil {
		privateKeyExists = true
	} else if os.IsNotExist(err) {
		privateKeyExists = false
	} else {
		// Schrodinger: file may or may not exist. See err for details.
		// Therefore, do *NOT* use !os.IsNotExist(err) to test for file existence
		return false, err
	}
	if _, err := os.Stat(pubKeyPath); err == nil {
		publicKeyExists = true
	} else if os.IsNotExist(err) {
		publicKeyExists = false
	} else {
		return false, err
	}
	if !privateKeyExists && publicKeyExists {
		return false, errors.New("One key file is missing")
	} else if !publicKeyExists && privateKeyExists {
		return false, errors.New("One key file is missing")
	} else if privateKeyExists && publicKeyExists {
		log.Println("Keys found, loading keys")
		return true, nil
	} else {
		pubKeyFile, err := os.Create(pubKeyPath)
		privateKeyFile, err := os.Create(privateKeyPath)
		defer pubKeyFile.Close()
		defer privateKeyFile.Close()
		if err != nil {
			return false, err
		}
		return false, nil
	}
}

//PublicKeyFile Load the public/private key pair
func PublicKeyFile(file string) (ssh.AuthMethod, error) {
	buffer, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(key), nil
}

func PrivateKeySigner(file string) (*ssh.Signer, error) {
	buffer, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil, err
	}
	return &key, nil
}

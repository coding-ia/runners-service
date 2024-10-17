package ssh_gen

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"golang.org/x/crypto/ssh"
	"os"
)

func GenerateMachineKeys(filename string) (string, error) {
	privKey, err := generateRSAKeyPair(2048)
	if err != nil {
		return "", err
	}
	pemBlock := generatePrivateKeyPEM(privKey)
	writePEMBlock(filename, pemBlock)

	authKey, err := generateAuthorizedKey(privKey.PublicKey)
	if err != nil {
		return "", err
	}

	return string(authKey), nil
}

func generateRSAKeyPair(bits int) (*rsa.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

func generatePrivateKeyPEM(key *rsa.PrivateKey) *pem.Block {
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(key)
	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	return pemBlock
}

func writePEMBlock(filename string, pemBlock *pem.Block) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	return pem.Encode(file, pemBlock)
}

func generateAuthorizedKey(pubkey rsa.PublicKey) ([]byte, error) {
	sshPublicKey, err := ssh.NewPublicKey(&pubkey)
	if err != nil {
		return nil, err
	}

	pubKeyBytes := ssh.MarshalAuthorizedKey(sshPublicKey)

	return pubKeyBytes, nil
}

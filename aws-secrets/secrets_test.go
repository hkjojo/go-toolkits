package aws_secrets

import (
	"testing"
)

func TestAESEncrypt(t *testing.T) {
	encrypt, err := AESEncrypt("Abcd123456!", "ijhFBDUIG8eIC4NdAc0w7xOgdmXBzhiR")
	if err != nil {
		t.Fatal(err)
		return
	}

	t.Log(encrypt)
}

func TestAESDecrypt(t *testing.T) {
	decrypt, err := AESDecrypt("UurquXhROZpitmDvq1MYIMrA/F1VE+xEIviw6J7ybH0=", "ijhFBDUIG8eIC4NdAc0w7xOgdmXBzhiR")
	if err != nil {
		t.Fatal(err)
		return
	}

	t.Log(string(decrypt))
}

func TestGetSecrets(t *testing.T) {
	secrets, err := GetAwsSecrets("beta/mt5manager/1000")
	if err != nil {
		t.Fatal(err)
		return
	}

	t.Log(secrets)
}

package helper


import (
    "crypto/rand"
    "crypto/rsa"
    "crypto/sha256"
    "crypto/x509"
)



func EncryptData(data string) ([]byte, []byte, error){
    privKey, err := rsa.GenerateKey(rand.Reader, 2048)
    if err != nil {
        return []byte{}, []byte{}, err
    }

    privBytes := x509.MarshalPKCS1PrivateKey(privKey)

    encryptedData, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &privKey.PublicKey, []byte(data), nil)
    if err != nil {
        return []byte{}, []byte{}, err
    }
    return encryptedData, privBytes, nil
}


func DecryptData(data []byte, key []byte) (string, error){
    privKey, err := x509.ParsePKCS1PrivateKey(key)
    if err != nil {
        return "", err
    }

    decryptedData, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privKey, data, nil)
    if err != nil {
        return "", err
    }

    return string(decryptedData), nil
}

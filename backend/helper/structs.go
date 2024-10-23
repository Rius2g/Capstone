package helper 

import (
    "math/big"
)



type PublishData struct { 
    Data []byte `json:"data"`
    Owner string `json:"owner"`
    ReleaseTime uint64 `json:"releaseTime"`
    Hash []byte `json:"hash"`
}



type PushEncrytedDataEvent struct {
    EncryptedData []byte `json:"encryptedData"`
    Owner string `json:"owner"`
    DataName string `json:"dataName"`
    Hash []byte `json:"hash"`
}


type PushPrivateKeyEvent struct {
    DecryptionKey []byte `json:"decryptionKey"`
    Owner string `json:"owner"`
    DataName string `json:"dataName"`
    Hash []byte `json:"hash"`
}



type PublicData struct {
    EncryptedData []byte `json:"encryptedData"`
    Owner string `json:"owner"`
    ReleaseTime *big.Int `json:"releaseTime"`
    Hash [32]byte `json:"hash"`
    DataName string `json:"dataName"`
}

type ReleaseEncryptedDataEvent struct {
    EncryptedData []byte
    Owner         string
    DataName      string
    ReleaseTime   *big.Int
    Hash          []byte
}

type KeyReleasedEvent struct {
    PrivateKey []byte
    Owner      string
    DataName   string
}

type KeyReleaseRequestedEvent struct {
    Index    *big.Int
    Owner    string
    DataName string
}

package main

import (
	"fmt"
	"log"
    "crypto/x509"
    "encoding/base64"
    "encoding/pem"
	"net/http"
    "errors"
    "crypto/rand"
    "crypto/rsa"
    "crypto/sha256"
    "github.com/gin-contrib/cors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
    "github.com/ethereum/go-ethereum/accounts/abi"
    "strings"
    "github.com/gorilla/websocket"
    
)


const (
    infuraURL       = "https://mainnet.infura.io/v3/YOUR_INFURA_PROJECT_ID"
    contractAddress  = "0xYourContractAddress"
    contractABI      = `[{"constant":true,"inputs":[],"name":"getValue","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"}]`
)

type Contract struct {
    contract abi.ABI 
    address common.Address
    client *ethclient.Client
}

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true
    },
}

func HandleWebSocket(c *gin.Context) {
    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)   
    if err != nil {
        log.Println(err)
        return
    }
    defer conn.Close()

    for {
        messageType, p, err := conn.ReadMessage()
        if err != nil {
            log.Println(err)
            return
        }
        if err := conn.WriteMessage(messageType, p); err != nil {
            log.Println(err)
            return
        }
    }
}


var client *ethclient.Client
var contractInstance *Contract
func main() {
	// Connect to an Avalanche node
    var err error
	client, err = ethclient.Dial("https://api.avax.network/ext/bc/C/rpc")
	if err != nil {
		log.Fatal(err)
	}


    parsedABI, err := abi.JSON(strings.NewReader(contractABI))
    if err != nil {
        log.Fatalf("Failed to parse contract ABI: %v", err)
    }

    contractAddress := common.HexToAddress(contractAddress)

    contractInstance = &Contract{contract: parsedABI, address: contractAddress, client: client}


	// Create a new Gin router
    startServer()
}

func startServer(){
    // Create a new Gin router
    r := gin.Default()
    r.Use(cors.Default())

    r.GET("/chainID", getChainID)
    r.GET("/blockNumber", getBlockNumber)
    r.GET("/ws", HandleWebSocket)
    r.GET("get_locked_data", getLockedData)
    r.GET("/getFromChain", getFromChain)

    r.POST("/postToChain", postToChain)

    // Run the server
    r.Run(":8080")
}

func getLockedData(c *gin.Context){
    //interact with the contract and get the data
}


func getChainID(c *gin.Context){
    // Get the chain ID
    chainID, err := client.ChainID(c)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"chainID": chainID.String()})
}


func getFromChain(c *gin.Context){
}


func generateKeyPair() (*rsa.PrivateKey, *rsa.PublicKey, error){
    // Generate a public, private key pair
    privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
    if err != nil {
        return nil, nil, err
    }

    publicKey := &privateKey.PublicKey

    return privateKey, publicKey, nil
}

func encryptData(data string) (string, string, error){

    // create a public, privatekey pair and encrypt the encryptData 
    privateKey, publicKey, err := generateKeyPair()
    if err != nil {
        return "", "", err
    }

    // Encrypt the encryptData

    encryptedData, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, []byte(data), nil)
    if err != nil {
        return "", "", err
    }

    return string(encryptedData), string(privateKey.D.Bytes()), nil

}


func decryptData(encryptedData, privateKeyPEM string) (string, error) {
    // Decode the base64 encrypted data
    encryptedDataBytes, err := base64.StdEncoding.DecodeString(encryptedData)
    if err != nil {
        return "", err
    }

    // Decode the PEM private key
    block, _ := pem.Decode([]byte(privateKeyPEM))
    if block == nil || block.Type != "RSA PRIVATE KEY" {
        return "", errors.New("failed to decode PEM block containing private key")
    }

    // Parse the private key
    privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
    if err != nil {
        return "", err
    }

    // Decrypt the data
    decryptedData, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, encryptedDataBytes, nil)
    if err != nil {
        return "", err
    }

    return string(decryptedData), nil
}

func postToChain(c *gin.Context){ 
    //get the data from the request and post to chain 

    Data := c.PostForm("data")
    releaseTime := c.PostForm("releaseTime")
    owner := c.PostForm("owner")

    //encrypt the data and post to postToChain

    fmt.Println(Data, releaseTime, owner)

    encryptedData, privateKey, err := encryptData(Data)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    fmt.Println(encryptedData, privateKey)


    // Post the data to the postToChain

    c.JSON(http.StatusOK, gin.H{"message": "Post to chain"})
}


func getBlockNumber(c *gin.Context){
	// Get the latest block number
		blockNumber, err := client.BlockNumber(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"blockNumber": blockNumber})

}
